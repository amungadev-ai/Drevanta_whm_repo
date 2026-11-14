// verbctl: drill client for the Drevanta privileged helper socket.
// Speaks the exact HostRequest/HostResult newline-JSON protocol; runs as
// root on drill hosts (peer UID 0 is allowlisted by the helper registry).
// NEVER embeds credentials; the socket path is a flag.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"drevanta/internal/helper"
	"drevanta/internal/hostagent"
)

func main() {
	socket := flag.String("socket", "/run/drevanta/helper.sock", "helper unix socket")
	verb := flag.String("verb", "", "helper verb")
	argsJSON := flag.String("args", "{}", "typed verb arguments (JSON)")
	installID := flag.String("installation", "", "installation id")
	hostID := flag.String("host", "*", "host id binding")
	timeout := flag.Duration("timeout", 120*time.Second, "call timeout")
	flag.Parse()
	if *verb == "" {
		fmt.Fprintln(os.Stderr, "usage: verbctl -verb VERB [-args JSON]")
		os.Exit(2)
	}
	var args json.RawMessage
	if err := json.Unmarshal([]byte(*argsJSON), &args); err != nil {
		fmt.Fprintln(os.Stderr, "args JSON invalid:", err)
		os.Exit(2)
	}
	opID := fmt.Sprintf("drill-%d", time.Now().UnixNano())
	req := helper.HostRequest{
		Version:        helper.Version,
		InstallationID: *installID,
		HostID:         *hostID,
		Verb:           *verb,
		Arguments:      args,
		Fence:          1,
		OperationID:    opID,
		Digest:         helper.DigestOf(opID, *verb, args),
		ExpiresAt:      time.Now().UTC().Add(2 * time.Minute).Format(time.RFC3339Nano),
	}
	client := &hostagent.Client{SocketPath: *socket, Timeout: *timeout}
	res, err := client.Call(context.Background(), req)
	if err != nil {
		fmt.Fprintln(os.Stderr, "CALL-ERROR:", err)
		os.Exit(1)
	}
	out, _ := json.MarshalIndent(map[string]any{
		"status":      res.Status,
		"outcome":     res.Outcome,
		"safe_reason": res.SafeReason,
		"result":      res.Observation,
		"journal_id":  res.JournalID,
	}, "", "  ")
	fmt.Println(string(out))
	if res.Status != "ok" {
		os.Exit(1)
	}
}
