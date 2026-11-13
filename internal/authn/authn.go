// Package authn implements identity proof: Argon2id credentials, RFC 6238
// TOTP MFA, single-use challenges and epoch-bound sessions.
//
// Contract basis: spec/specification/contracts/profile.md §Authorization and
// storage-schema entities principal/credential/session/challenge (DEC-004,
// DEC-008, DEC-011). Epoch mismatch or revocation denies every request.
package authn

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base32"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"

	"drevanta/internal/store"
)

var (
	ErrInvalidCredential = errors.New("invalid credential")
	ErrInvalidMFA        = errors.New("invalid_mfa")
	ErrPrincipalDisabled = errors.New("principal_disabled")
	ErrSessionInvalid    = errors.New("session_invalid")
)

// MFARequired signals successful password proof with MFA enrollment; the
// principal ID lets the client complete verification without revealing
// account existence (password was already proven).
type MFARequired struct{ PrincipalID string }

func (e *MFARequired) Error() string { return "mfa_required" }

// Argon2id parameters (OWASP 2024 baseline: 19 MiB, t=2, p=1).
const (
	argonMemory  uint32 = 19 * 1024
	argonTime    uint32 = 2
	argonThreads uint8  = 1
	argonKeyLen  uint32 = 32
	argonSaltLen        = 16
)

// HashPassword returns a PHC-format Argon2id hash.
func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		argonMemory, argonTime, argonThreads,
		base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(salt),
		base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(key)), nil
}

// VerifyPassword checks a password against a PHC-format Argon2id hash,
// re-deriving with the stored parameters.
func VerifyPassword(password, phc string) (bool, error) {
	parts := strings.Split(phc, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, fmt.Errorf("unexpected hash format")
	}
	var m uint32
	var t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return false, err
	}
	salt, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(parts[4])
	if err != nil {
		return false, err
	}
	want, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(parts[5])
	if err != nil {
		return false, err
	}
	got := argon2.IDKey([]byte(password), salt, t, m, p, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

// TOTPValidate validates a 6-digit RFC 6238 code within ±1 time step (30s).
func TOTPValidate(secretB32, code string, at time.Time) bool {
	secret, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(
		strings.ToUpper(strings.ReplaceAll(secretB32, " ", "")))
	if err != nil {
		return false
	}
	if len(code) != 6 {
		return false
	}
	var n int
	if _, err := fmt.Sscanf(code, "%06d", &n); err != nil {
		return false
	}
	counter := uint64(at.Unix()) / 30
	for _, off := range []uint64{0, 1, ^uint64(0)} { // current, next, previous
		if hotpValue(secret, counter+off) == uint32(n) {
			return true
		}
	}
	return false
}

// TOTPCodeAt computes the 6-digit RFC 6238 code for a base32 secret at the
// given time (current step). Used by tests and enrollment verification UX.
func TOTPCodeAt(secretB32 string, at time.Time) (string, error) {
	secret, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(
		strings.ToUpper(strings.ReplaceAll(secretB32, " ", "")))
	if err != nil {
		return "", err
	}
	if len(secret) == 0 {
		return "", errors.New("empty totp secret")
	}
	return fmt.Sprintf("%06d", hotpValue(secret, uint64(at.Unix())/30)), nil
}

func hotpValue(secret []byte, counter uint64) uint32 {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], counter)
	mac := hmac.New(sha256.New, secret)
	mac.Write(buf[:])
	sum := mac.Sum(nil)
	idx := sum[len(sum)-1] & 0x0f
	v := (uint32(sum[idx])&0x7f)<<24 |
		uint32(sum[idx+1])<<16 |
		uint32(sum[idx+2])<<8 |
		uint32(sum[idx+3])
	return v % 1000000
}

// GenerateTOTPSecret returns a fresh base32 secret (160 bits).
func GenerateTOTPSecret() (string, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b), nil
}

// HashToken hashes session/recovery material for storage (raw never stored).
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Session is an issued session with its raw bearer token returned once.
type IssuedSession struct {
	SessionID    string
	Token        string // raw bearer token, shown to caller exactly once
	PrincipalID  string
	ExpiresAt    time.Time
	MFASatisfied bool
}

// Service implements authentication on top of the store.
type Service struct {
	db          *store.DB
	sessionTTL  time.Duration
	recoveryTTL time.Duration
}

// NewService builds an authn Service.
func NewService(db *store.DB, sessionTTL, recoveryTTL time.Duration) *Service {
	return &Service{db: db, sessionTTL: sessionTTL, recoveryTTL: recoveryTTL}
}

// SetPrimaryPassword stores (or replaces) the primary password credential and
// bumps the principal's credential epoch, revoking prior sessions.
func (s *Service) SetPrimaryPassword(ctx context.Context, principalID, password string) error {
	phc, err := HashPassword(password)
	if err != nil {
		return err
	}
	// Revoke existing password credentials (single active primary).
	if _, err := s.db.ExecContext(ctx,
		`UPDATE credential SET state='revoked', revoked_at=? WHERE principal_id=? AND purpose='primary_password' AND state='active'`,
		store.Now(), principalID); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO credential(credential_id, principal_id, purpose, state, material, created_at)
		 VALUES(?,?,?,?,?,?)`,
		store.NewID("crd"), principalID, "primary_password", "active", phc, store.Now()); err != nil {
		return err
	}
	_, err = s.db.BumpCredentialEpoch(ctx, principalID)
	return err
}

// EnrollTOTP activates an MFA credential and returns the enrollment secret.
func (s *Service) EnrollTOTP(ctx context.Context, principalID string) (string, error) {
	secret, err := GenerateTOTPSecret()
	if err != nil {
		return "", err
	}
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO credential(credential_id, principal_id, purpose, state, material, created_at)
		 VALUES(?,?,?,?,?,?)`,
		store.NewID("crd"), principalID, "mfa_totp", "active", secret, store.Now()); err != nil {
		return "", err
	}
	return secret, nil
}

// HasActiveMFA reports whether an active TOTP credential exists.
func (s *Service) HasActiveMFA(ctx context.Context, principalID string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM credential WHERE principal_id=? AND purpose='mfa_totp' AND state='active'`,
		principalID).Scan(&n)
	return n > 0, err
}

// MFASecret returns the active TOTP secret (server-side verification only).
func (s *Service) MFASecret(ctx context.Context, principalID string) (string, error) {
	var m string
	err := s.db.QueryRowContext(ctx,
		`SELECT material FROM credential WHERE principal_id=? AND purpose='mfa_totp' AND state='active'
		 ORDER BY created_at DESC LIMIT 1`, principalID).Scan(&m)
	if errors.Is(err, sql.ErrNoRows) {
		return "", store.ErrNotFound
	}
	return m, err
}

// Login verifies the password and either issues a session (no MFA enrolled)
// or returns ErrMFARequired; in that case a mfa_verify challenge is created
// bound to the principal.
func (s *Service) Login(ctx context.Context, email, password string) (*IssuedSession, error) {
	p, err := s.db.PrincipalByEmail(ctx, email)
	if errors.Is(err, store.ErrNotFound) {
		// Equal-cost deny: perform a hash verification against a dummy hash to
		// avoid user enumeration by timing.
		_, _ = VerifyPassword(password, dummyHash)
		return nil, ErrInvalidCredential
	}
	if err != nil {
		return nil, err
	}
	if p.Status != "active" {
		return nil, ErrPrincipalDisabled
	}
	var material string
	err = s.db.QueryRowContext(ctx,
		`SELECT material FROM credential WHERE principal_id=? AND purpose='primary_password' AND state='active'
		 ORDER BY created_at DESC LIMIT 1`, p.PrincipalID).Scan(&material)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrInvalidCredential
	}
	if err != nil {
		return nil, err
	}
	ok, err := VerifyPassword(password, material)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrInvalidCredential
	}
	mfa, err := s.HasActiveMFA(ctx, p.PrincipalID)
	if err != nil {
		return nil, err
	}
	if mfa {
		if _, err := s.db.ExecContext(ctx,
			`INSERT INTO challenge(challenge_id, principal_id, purpose, verifier_ref, expires_at)
			 VALUES(?,?,?,?,?)`,
			store.NewID("chl"), p.PrincipalID, "mfa_verify", "-",
			time.Now().UTC().Add(5*time.Minute).Format(time.RFC3339Nano)); err != nil {
			return nil, err
		}
		return nil, &MFARequired{PrincipalID: p.PrincipalID}
	}
	return s.issueSession(ctx, p, false)
}

// VerifyMFA completes login for MFA-enrolled principals: the most recent
// unconsumed mfa_verify challenge is consumed atomically. A valid recovery
// code is accepted as the second factor and burned (single use).
func (s *Service) VerifyMFA(ctx context.Context, principalID, code string) (*IssuedSession, error) {
	p, err := s.db.PrincipalByID(ctx, principalID)
	if err != nil {
		return nil, err
	}
	if p.Status != "active" {
		return nil, ErrPrincipalDisabled
	}
	// Try a recovery code first (single use, hashed comparison).
	if used, err := s.ConsumeRecoveryCode(ctx, principalID, code); err == nil && used {
		// Burn any pending mfa_verify challenges and issue the session.
		_, _ = s.db.ExecContext(ctx,
			`UPDATE challenge SET consumed_at=? WHERE principal_id=? AND purpose='mfa_verify' AND consumed_at IS NULL`,
			store.Now(), principalID)
		return s.issueSession(ctx, p, true)
	}
	secret, err := s.MFASecret(ctx, principalID)
	if err != nil {
		return nil, ErrInvalidMFA
	}
	if !TOTPValidate(secret, code, time.Now()) {
		return nil, ErrInvalidMFA
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE challenge SET consumed_at=? WHERE principal_id=? AND purpose='mfa_verify'
		 AND consumed_at IS NULL AND expires_at > ?`,
		store.Now(), principalID, store.Now())
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, ErrInvalidMFA
	}
	return s.issueSession(ctx, p, true)
}

func (s *Service) issueSession(ctx context.Context, p *store.Principal, mfa bool) (*IssuedSession, error) {
	raw, err := randomToken(32)
	if err != nil {
		return nil, err
	}
	sess := &IssuedSession{
		SessionID:    store.NewID("ses"),
		Token:        "dvt_" + raw,
		PrincipalID:  p.PrincipalID,
		ExpiresAt:    time.Now().UTC().Add(s.sessionTTL),
		MFASatisfied: mfa,
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO session(session_id, principal_id, token_hash, credential_epoch, issued_at, expires_at, mfa_satisfied)
		 VALUES(?,?,?,?,?,?,?)`,
		sess.SessionID, p.PrincipalID, HashToken(sess.Token), p.CredentialEpoch,
		store.Now(), sess.ExpiresAt.Format(time.RFC3339Nano), boolInt(mfa))
	if err != nil {
		return nil, err
	}
	return sess, nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// Authenticate resolves a bearer token to its principal, enforcing epoch
// match, non-revocation and non-expiry. Any failure denies uniformly.
func (s *Service) Authenticate(ctx context.Context, bearer string) (*store.Principal, *SessionInfo, error) {
	if bearer == "" {
		return nil, nil, ErrSessionInvalid
	}
	const q = `SELECT session_id, principal_id, credential_epoch, expires_at, revoked_at, mfa_satisfied
		 FROM session WHERE token_hash = ?`
	var si SessionInfo
	var revoked *string
	var exp string
	err := s.db.QueryRowContext(ctx, q, HashToken(bearer)).
		Scan(&si.SessionID, &si.PrincipalID, &si.CredentialEpoch, &exp, &revoked, &si.MFASatisfied)
	if err != nil {
		return nil, nil, ErrSessionInvalid
	}
	if revoked != nil {
		return nil, nil, ErrSessionInvalid
	}
	expT, err := time.Parse(time.RFC3339Nano, exp)
	if err != nil || time.Now().UTC().After(expT) {
		return nil, nil, ErrSessionInvalid
	}
	p, err := s.db.PrincipalByID(ctx, si.PrincipalID)
	if err != nil {
		return nil, nil, ErrSessionInvalid
	}
	if p.Status != "active" || p.CredentialEpoch != si.CredentialEpoch {
		return nil, nil, ErrSessionInvalid
	}
	si.ExpiresAt = expT
	return p, &si, nil
}

// SessionInfo carries authenticated session metadata.
type SessionInfo struct {
	SessionID       string
	PrincipalID     string
	CredentialEpoch int64
	ExpiresAt       time.Time
	MFASatisfied    bool
}

// RevokeAllSessions revokes every session of a principal (credential change).
func (s *Service) RevokeAllSessions(ctx context.Context, principalID string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE session SET revoked_at=? WHERE principal_id=? AND revoked_at IS NULL`,
		store.Now(), principalID)
	return err
}

// IssueChallenge creates a single-use challenge of the given purpose with a
// random verifier; the raw verifier is returned once and its hash stored.
func (s *Service) IssueChallenge(ctx context.Context, principalID, purpose string) (string, error) {
	raw, err := randomToken(16)
	if err != nil {
		return "", err
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO challenge(challenge_id, principal_id, purpose, verifier_ref, expires_at)
		 VALUES(?,?,?,?,?)`,
		store.NewID("chl"), principalID, purpose, HashToken(raw),
		time.Now().UTC().Add(s.recoveryTTL).Format(time.RFC3339Nano))
	if err != nil {
		return "", err
	}
	return raw, nil
}

// ConsumeChallenge atomically consumes a challenge if the verifier matches,
// it is unconsumed and unexpired. Returns store.ErrNotFound when no matching
// challenge exists (uniform response, no enumeration).
func (s *Service) ConsumeChallenge(ctx context.Context, principalID, purpose, verifier string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE challenge SET consumed_at=? WHERE principal_id=? AND purpose=? AND verifier_ref=?
		 AND consumed_at IS NULL AND expires_at > ?`,
		store.Now(), principalID, purpose, HashToken(verifier), store.Now())
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return store.ErrNotFound
	}
	return nil
}

// dummyHash is a fixed Argon2id PHC string used for timing-equal denial.
var dummyHash = "$argon2id$v=19$m=19456,t=2,p=1$AAAAAAAAAAAAAAAAAAAAAQQAAAAAAQ$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

// ---- Password reset (phase 17: binds account, purpose, expiry, unique
// token digest; reset revokes sessions, preserves MFA, requires fresh sign-in) ----

// IssuePasswordReset creates a single-use password_reset challenge for the
// principal. The raw token is the verifier (returned once for the delivery
// adapter); only its hash is stored.
func (s *Service) IssuePasswordReset(ctx context.Context, principalID string) (string, error) {
	return s.IssueChallenge(ctx, principalID, "password_reset")
}

// ConfirmPasswordReset atomically consumes the challenge and rotates the
// password. Credential epoch advances, denying every prior session at their
// next request (epoch mismatch). MFA enrollment is preserved by design:
// credentials of purpose mfa_totp are untouched.
func (s *Service) ConfirmPasswordReset(ctx context.Context, principalID, token, newPassword string) error {
	if err := s.ConsumeChallenge(ctx, principalID, "password_reset", token); err != nil {
		return err // ErrNotFound → single-use/expiry/mismatch all deny uniformly
	}
	return s.SetPrimaryPassword(ctx, principalID, newPassword)
}

// ---- Recovery codes (phase 17: one consumption, rotate on regeneration,
// only verifier material stored) ----

// GenerateRecoveryCodes revokes any prior codes and returns 10 fresh raw
// codes (shown once). Hashes are stored as credential rows.
func (s *Service) GenerateRecoveryCodes(ctx context.Context, principalID string) ([]string, error) {
	if _, err := s.db.ExecContext(ctx,
		`UPDATE credential SET state='revoked', revoked_at=? WHERE principal_id=? AND purpose='recovery_code' AND state='active'`,
		store.Now(), principalID); err != nil {
		return nil, err
	}
	codes := make([]string, 0, 10)
	for i := 0; i < 10; i++ {
		raw, err := randomToken(8) // 16 hex chars
		if err != nil {
			return nil, err
		}
		if _, err := s.db.ExecContext(ctx,
			`INSERT INTO credential(credential_id, principal_id, purpose, state, material, created_at)
			 VALUES(?,?,?,?,?,?)`,
			store.NewID("crd"), principalID, "recovery_code", "active", HashToken(raw), store.Now()); err != nil {
			return nil, err
		}
		codes = append(codes, raw)
	}
	return codes, nil
}

// ConsumeRecoveryCode validates and burns one recovery code (atomic single
// use). Recovery codes satisfy the MFA step; they never bypass password proof.
func (s *Service) ConsumeRecoveryCode(ctx context.Context, principalID, code string) (bool, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE credential SET state='revoked', revoked_at=? WHERE principal_id=? AND purpose='recovery_code' AND state='active' AND material=?`,
		store.Now(), principalID, HashToken(code))
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}
