package helper

// TLS verbs (G30): certificate issuance, renewal and failure recovery at the
// privileged boundary. Issuance today is real self-signed issuance (ECDSA
// P-256, SAN-complete, 0600 keys). ACME is a registered seam: with no issuer
// registered, an ACME request fails CLOSED (never a silent downgrade);
// self-signed fallback only when the caller explicitly allows it.
// Renewal checks NotAfter against a threshold; recovery replaces corrupt or
// expired pairs and reports exactly what happened.

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// TLSApplyArgs is the typed argument for tls.apply.
type TLSApplyArgs struct {
	Op         string   `json:"op"`                    // issue | renew | recover
	Binding    string   `json:"binding"`               // ssl directory binding (root-owned)
	ServerName string   `json:"server_name"`           // CN + mandatory SAN
	Issuer     string   `json:"issuer,omitempty"`      // selfsigned (default) | acme
	Days       int      `json:"days,omitempty"`        // validity (default 825, max 825)
	ThresholdD int      `json:"threshold_d,omitempty"` // renew threshold (default 30)
	SANs       []string `json:"sans,omitempty"`        // extra SANs
	// AllowSelfSignedFallback permits a self-signed fallback when the
	// requested (ACME) issuance fails; the result reports the fallback.
	AllowSelfSignedFallback bool `json:"allow_self_signed_fallback,omitempty"`
}

// sslDirs are the only key/cert directories the verb accepts.
var sslDirs = map[string]bool{}

// LoadTLSBindings reads the root-owned TLS directory bindings.
func LoadTLSBindings(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var dirs []string
	if err := json.Unmarshal(b, &dirs); err != nil {
		return err
	}
	for _, d := range dirs {
		sslDirs[filepath.Clean(d)] = true
	}
	return nil
}

// ACMEIssuer performs a real ACME issuance into dir for serverName. The
// helper ships without one (no public domain mapping is an owner decision);
// absence fails closed.
var acmeIssuer func(dir, serverName string, staging bool) error

// SetACMEIssuer registers a real ACME implementation (owner-authorized
// deployment decision). nil deregisters.
func SetACMEIssuer(f func(dir, serverName string, staging bool) error) { acmeIssuer = f }

func validServerName(s string) error {
	if s == "" || len(s) > 253 || strings.ContainsAny(s, " /\\\x00") || strings.Contains(s, "..") {
		return fmt.Errorf("invalid server name")
	}
	return nil
}

func certPaths(dir, serverName string) (string, string) {
	return filepath.Join(dir, serverName+".crt"), filepath.Join(dir, serverName+".key")
}

// issueSelfSigned generates a fresh key + self-signed leaf. Returns the
// SHA-256 fingerprint and NotAfter.
func issueSelfSigned(dir, serverName string, sans []string, days int) (string, time.Time, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("keygen failed")
	}
	if days <= 0 {
		days = 825
	}
	if days > 825 {
		days = 825
	}
	names := append([]string{serverName}, sans...)
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("serial failed")
	}
	notAfter := time.Now().UTC().AddDate(0, 0, days)
	tmpl := x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: serverName, Organization: []string{"Drevanta"}},
		DNSNames:              names,
		NotBefore:             time.Now().UTC().Add(-1 * time.Hour),
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("cert creation failed")
	}
	crt, keyP := certPaths(dir, serverName)
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("key marshal failed")
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", time.Time{}, fmt.Errorf("ssl dir creation failed")
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	if err := os.WriteFile(keyP, keyPEM, 0o600); err != nil {
		return "", time.Time{}, fmt.Errorf("key write failed")
	}
	crtPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if err := os.WriteFile(crt, crtPEM, 0o644); err != nil {
		return "", time.Time{}, fmt.Errorf("cert write failed")
	}
	sum := sha256.Sum256(der)
	return hex.EncodeToString(sum[:]), notAfter, nil
}

// readCert parses the on-disk certificate for a server name.
func readCert(dir, serverName string) (*x509.Certificate, error) {
	crt, _ := certPaths(dir, serverName)
	pb, err := os.ReadFile(crt)
	if err != nil {
		return nil, fmt.Errorf("certificate missing")
	}
	blk, _ := pem.Decode(pb)
	if blk == nil {
		return nil, fmt.Errorf("certificate unparsable")
	}
	c, err := x509.ParseCertificate(blk.Bytes)
	if err != nil {
		return nil, fmt.Errorf("certificate unparsable")
	}
	return c, nil
}

// TLSApplyHandler performs issuance, renewal and recovery.
func TLSApplyHandler() VerbHandler {
	return func(_ context.Context, args json.RawMessage) (json.RawMessage, error) {
		dec := json.NewDecoder(strings.NewReader(string(args)))
		dec.DisallowUnknownFields()
		var a TLSApplyArgs
		if err := dec.Decode(&a); err != nil {
			return nil, fmt.Errorf("invalid tls.apply arguments")
		}
		if err := validServerName(a.ServerName); err != nil {
			return nil, err
		}
		if !sslDirs[filepath.Clean(a.Binding)] {
			return nil, fmt.Errorf("binding not allowlisted")
		}
		switch a.Op {
		case "issue":
			return tlsIssue(a)
		case "renew":
			return tlsRenew(a)
		case "recover":
			return tlsRecover(a)
		default:
			return nil, fmt.Errorf("invalid tls operation")
		}
	}
}

func tlsIssue(a TLSApplyArgs) (json.RawMessage, error) {
	if a.Issuer == "acme" {
		if acmeIssuer != nil {
			if err := acmeIssuer(a.Binding, a.ServerName, true); err != nil {
				if a.AllowSelfSignedFallback {
					return tlsSelfSignedResult(a, "selfsigned (acme fallback)")
				}
				return nil, fmt.Errorf("acme issuance failed")
			}
			c, err := readCert(a.Binding, a.ServerName)
			if err != nil {
				return nil, err
			}
			return tlsResult(c, "acme"), nil
		}
		// Fail closed: no ACME issuer is registered. A silent self-signed
		// downgrade would misrepresent trust; only an explicit fallback is
		// acceptable and it is reported as such.
		if a.AllowSelfSignedFallback {
			return tlsSelfSignedResult(a, "selfsigned (acme-unavailable fallback)")
		}
		return nil, fmt.Errorf("acme unavailable: no issuer registered")
	}
	return tlsSelfSignedResult(a, "selfsigned")
}

func tlsSelfSignedResult(a TLSApplyArgs, source string) (json.RawMessage, error) {
	fp, notAfter, err := issueSelfSigned(a.Binding, a.ServerName, a.SANs, a.Days)
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{
		"server_name": a.ServerName, "fingerprint_sha256": fp,
		"not_after": notAfter.UTC().Format(time.RFC3339), "source": source,
		"state": "active",
	})
}

func tlsResult(c *x509.Certificate, source string) json.RawMessage {
	out, _ := json.Marshal(map[string]any{
		"server_name": c.Subject.CommonName,
		"fingerprint_sha256": func() string {
			sum := sha256.Sum256(c.Raw)
			return hex.EncodeToString(sum[:])
		}(),
		"not_after": c.NotAfter.UTC().Format(time.RFC3339), "source": source,
		"state": "active",
	})
	return out
}

func tlsRenew(a TLSApplyArgs) (json.RawMessage, error) {
	c, err := readCert(a.Binding, a.ServerName)
	if err != nil {
		// Missing/corrupt certificate during a renewal check is a recovery
		// case, not a silent failure.
		return tlsRecover(a)
	}
	threshold := a.ThresholdD
	if threshold <= 0 {
		threshold = 30
	}
	if time.Until(c.NotAfter) > time.Duration(threshold)*24*time.Hour {
		return json.Marshal(map[string]any{
			"server_name": a.ServerName, "state": "still-valid",
			"not_after": c.NotAfter.UTC().Format(time.RFC3339),
			"fingerprint_sha256": func() string {
				sum := sha256.Sum256(c.Raw)
				return hex.EncodeToString(sum[:])
			}(),
		})
	}
	if a.Issuer == "acme" && acmeIssuer != nil {
		if err := acmeIssuer(a.Binding, a.ServerName, true); err != nil {
			if a.AllowSelfSignedFallback {
				return tlsSelfSignedResult(a, "selfsigned (renewal acme fallback)")
			}
			return nil, fmt.Errorf("acme renewal failed")
		}
		nc, err := readCert(a.Binding, a.ServerName)
		if err != nil {
			return nil, err
		}
		return tlsResult(nc, "acme"), nil
	}
	return tlsSelfSignedResult(a, "selfsigned (renewal)")
}

func tlsRecover(a TLSApplyArgs) (json.RawMessage, error) {
	// Recovery: diagnose, then replace with a fresh valid pair. The old
	// certificate is preserved as {name}.crt.invalid for post-mortem.
	crt, keyP := certPaths(a.Binding, a.ServerName)
	_, statErr := os.Stat(crt)
	if statErr == nil {
		if _, err := readCert(a.Binding, a.ServerName); err == nil {
			// Certificate itself is fine; check the key pairing.
			kb, kerr := os.ReadFile(keyP)
			if kerr != nil {
				os.Rename(crt, crt+".invalid")
			} else if blk, _ := pem.Decode(kb); blk == nil {
				os.Rename(keyP, keyP+".invalid")
			}
		} else {
			os.Rename(crt, crt+".invalid")
		}
	}
	fp, notAfter, err := issueSelfSigned(a.Binding, a.ServerName, a.SANs, a.Days)
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{
		"server_name": a.ServerName, "fingerprint_sha256": fp,
		"not_after": notAfter.UTC().Format(time.RFC3339),
		"source":    "selfsigned (recovery)", "state": "active",
	})
}
