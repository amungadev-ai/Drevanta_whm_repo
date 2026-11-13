// Package hostagent is the panel-side client for the privileged helper
// (DEC-003): dials the local Unix socket, sends typed requests, enforces
// bounded responses. The panel process is unprivileged; the socket peer
// check and root-owned policy are the helper's defenses.
package hostagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"

	"drevanta/internal/helper"
)

// Client connects to the helper socket.
type Client struct {
	SocketPath string
	Timeout    time.Duration
}

// ErrRejected indicates the helper refused before effects (safe).
var ErrRejected = errors.New("helper rejected request")

// Call sends one request and returns the result.
func (c *Client) Call(ctx context.Context, req helper.HostRequest) (helper.HostResult, error) {
	if c.Timeout == 0 {
		c.Timeout = 30 * time.Second
	}
	d := net.Dialer{Timeout: c.Timeout}
	conn, err := d.DialContext(ctx, "unix", c.SocketPath)
	if err != nil {
		return helper.HostResult{}, fmt.Errorf("helper socket: %w", err)
	}
	defer conn.Close()

	deadline := time.Now().Add(c.Timeout)
	_ = conn.SetDeadline(deadline)

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return helper.HostResult{}, err
	}
	dec := json.NewDecoder(conn)
	var res helper.HostResult
	if err := dec.Decode(&res); err != nil {
		// No parsable reply: outcome unknown.
		return helper.HostResult{}, fmt.Errorf("helper reply unparsable: %w", err)
	}
	if res.Status == "rejected" {
		return res, ErrRejected
	}
	return res, nil
}
