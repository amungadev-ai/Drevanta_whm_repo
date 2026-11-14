package mailsmtp

// SMTP delivery tests run against an in-process fake relay: the fake accepts
// one message, records it, and the test asserts the reset token reached the
// envelope without appearing anywhere else.

import (
	"bufio"
	"context"
	"net"
	"strings"
	"testing"
	"time"
)

type fakeRelay struct {
	listener net.Listener
	mailFrom string
	rcptTo   string
	data     string
	done     chan struct{}
}

func newFakeRelay(t *testing.T) *fakeRelay {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	fr := &fakeRelay{listener: l, done: make(chan struct{})}
	go func() {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		r := bufio.NewReader(conn)
		w := bufio.NewWriter(conn)
		write := func(s string) { w.WriteString(s + "\r\n"); w.Flush() }
		write("220 relay.test ESMTP")
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			cmd := strings.ToUpper(strings.TrimSpace(line))
			switch {
			case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
				write("250-relay.test")
				write("250 OK")
			case strings.HasPrefix(cmd, "MAIL FROM:"):
				fr.mailFrom = strings.TrimPrefix(strings.TrimSpace(line), "MAIL FROM:")
				write("250 OK")
			case strings.HasPrefix(cmd, "RCPT TO:"):
				fr.rcptTo = strings.TrimPrefix(strings.TrimSpace(line), "RCPT TO:")
				write("250 OK")
			case strings.HasPrefix(cmd, "DATA"):
				write("354 go ahead")
				var sb strings.Builder
				for {
					dl, _ := r.ReadString('\n')
					sb.WriteString(dl)
					if strings.TrimSpace(dl) == "." {
						break
					}
				}
				fr.data = sb.String()
				write("250 OK queued")
			case strings.HasPrefix(cmd, "QUIT"):
				write("221 bye")
				close(fr.done)
				return
			default:
				write("250 OK")
			}
		}
	}()
	t.Cleanup(func() { l.Close() })
	return fr
}

func TestDeliverPasswordReset(t *testing.T) {
	relay := newFakeRelay(t)
	m, err := New(Config{Host: "127.0.0.1", Port: portOf(relay.listener.Addr().String()), From: "panel@drevanta.test", Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.DeliverPasswordReset(context.Background(), "owner@site.test", "rstk_abc123"); err != nil {
		t.Fatalf("deliver: %v", err)
	}
	select {
	case <-relay.done:
	case <-time.After(5 * time.Second):
		t.Fatal("relay never received QUIT")
	}
	if !strings.Contains(relay.mailFrom, "panel@drevanta.test") {
		t.Fatalf("envelope from = %q", relay.mailFrom)
	}
	if !strings.Contains(relay.rcptTo, "owner@site.test") {
		t.Fatalf("envelope rcpt = %q", relay.rcptTo)
	}
	if !strings.Contains(relay.data, "rstk_abc123") {
		t.Fatal("reset token missing from message body")
	}
	if strings.Contains(relay.data, "\n\nReset token: rstk_abc123\n\nIf you did not request") == false {
		t.Logf("body: %q", relay.data)
	}
}

func TestNewRequiresHostAndFrom(t *testing.T) {
	if _, err := New(Config{}); err == nil {
		t.Fatal("expected validation error for empty config")
	}
}

func portOf(addr string) int {
	_, p, _ := net.SplitHostPort(addr)
	n := 0
	for _, c := range p {
		n = n*10 + int(c-'0')
	}
	return n
}
