// Package mailsmtp implements the recovery-delivery Mailer seam over real
// SMTP transport (G34 messaging wave). When SMTP configuration is present the
// control plane delivers password-reset material out-of-band; without it the
// panel keeps the auditable no-op adapter.
package mailsmtp

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"
	"time"
)

// Config carries the SMTP relay settings (env-sourced; credentials stay in
// the process environment, never in the database).
type Config struct {
	Host     string // relay host (e.g. smtp.example.net)
	Port     int    // relay port (default 587)
	From     string // RFC 5322 from address
	Username string // optional AUTH
	Password string // optional AUTH
	Timeout  time.Duration
}

// Mailer delivers password-reset tokens via SMTP.
type Mailer struct {
	cfg Config
}

// New validates the configuration (host and from are required).
func New(cfg Config) (*Mailer, error) {
	if strings.TrimSpace(cfg.Host) == "" || strings.TrimSpace(cfg.From) == "" {
		return nil, fmt.Errorf("smtp host and from address are required")
	}
	if cfg.Port == 0 {
		cfg.Port = 587
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}
	return &Mailer{cfg: cfg}, nil
}

// DeliverPasswordReset sends the reset token. The message body carries no
// credential material beyond the single-use token and never echoes the
// caller's schema.
func (m *Mailer) DeliverPasswordReset(_ context.Context, email, token string) error {
	addr := fmt.Sprintf("%s:%d", m.cfg.Host, m.cfg.Port)
	subject := "Drevanta password reset"
	body := "A password reset was requested for your Drevanta account.\n\n" +
		"Reset token: " + token + "\n\n" +
		"If you did not request this, ignore this message. The token expires automatically.\n"
	msg := strings.Join([]string{
		"From: " + m.cfg.From,
		"To: " + email,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=utf-8",
		"",
		body,
	}, "\r\n")

	var auth smtp.Auth
	if m.cfg.Username != "" {
		auth = smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, m.cfg.Host)
	}
	send := func() error {
		client, err := smtp.Dial(addr)
		if err != nil {
			return err
		}
		defer client.Close()
		if err := client.Hello("localhost"); err != nil {
			return err
		}
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(nil); err != nil {
				return err
			}
		}
		if auth != nil {
			if ok, _ := client.Extension("AUTH"); ok {
				if err := client.Auth(auth); err != nil {
					return err
				}
			}
		}
		if err := client.Mail(m.cfg.From); err != nil {
			return err
		}
		if err := client.Rcpt(email); err != nil {
			return err
		}
		w, err := client.Data()
		if err != nil {
			return err
		}
		if _, err := w.Write([]byte(msg)); err != nil {
			return err
		}
		if err := w.Close(); err != nil {
			return err
		}
		return client.Quit()
	}

	done := make(chan error, 1)
	go func() { done <- send() }()
	select {
	case err := <-done:
		return err
	case <-time.After(m.cfg.Timeout):
		return fmt.Errorf("smtp delivery timed out")
	}
}
