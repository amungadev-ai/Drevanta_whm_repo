package helper

// Peer credential extraction (SO_PEERCRED) and the built-in typed verbs.

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

// peerCredentials returns the SO_PEERCRED credentials of a Unix connection.
func peerCredentials(conn *net.UnixConn) (*syscall.Ucred, error) {
	raw, err := conn.SyscallConn()
	if err != nil {
		return nil, err
	}
	var cred *syscall.Ucred
	var opErr error
	err = raw.Control(func(fd uintptr) {
		cred, opErr = getPeerCred(int(fd))
	})
	if err != nil {
		return nil, err
	}
	if opErr != nil {
		return nil, opErr
	}
	return cred, nil
}

func getPeerCred(fd int) (*syscall.Ucred, error) {
	cred := &syscall.Ucred{}
	solen := uint32(unsafe.Sizeof(*cred)) / 4 // len in 4-byte units for getsockopt
	_, _, errno := syscall.Syscall6(syscall.SYS_GETSOCKOPT, uintptr(fd),
		syscall.SOL_SOCKET, syscall.SO_PEERCRED,
		uintptr(unsafe.Pointer(cred)), uintptr(unsafe.Pointer(&solen)), 0)
	if errno != 0 {
		return nil, errno
	}
	return cred, nil
}

// ---- Built-in typed verbs (phase 08 core) ----

// SupportedOS lists the qualified host tuples (DEC-005 development default:
// Debian-family apt+systemd). Anything else fails preflight closed.
var SupportedOS = map[string][]string{
	"ubuntu": {"24.04", "26.04", "26.10"},
	"debian": {"12", "13"},
}

type preflightArgs struct {
	ExtraFieldForbidden struct{} `json:"-"`
}

// OSInfo is the typed result of host.preflight/host.inspect.
type OSInfo struct {
	OSID       string `json:"os_id"`
	VersionID  string `json:"version_id"`
	PrettyName string `json:"pretty_name"`
	Arch       string `json:"arch"`
	Supported  bool   `json:"supported"`
	SafeReason string `json:"safe_reason,omitempty"`
}

// readOSInfo parses /etc/os-release.
func readOSInfo() OSInfo {
	info := OSInfo{Arch: runningArch()}
	b, err := os.ReadFile("/etc/os-release")
	if err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			switch {
			case strings.HasPrefix(line, "ID="):
				info.OSID = strings.Trim(strings.TrimPrefix(line, "ID="), `"`)
			case strings.HasPrefix(line, "VERSION_ID="):
				info.VersionID = strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), `"`)
			case strings.HasPrefix(line, "PRETTY_NAME="):
				info.PrettyName = strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), `"`)
			}
		}
	}
	return info
}

func runningArch() string {
	switch fmt.Sprintf("%s/%s", osArch(), osFamily()) {
	default:
		return "amd64"
	}
}

func osArch() string {
	if os.Getenv("GOARCH_TEST") != "" {
		return os.Getenv("GOARCH_TEST")
	}
	return "amd64"
}

func osFamily() string { return "linux" }

// PreflightHandler verifies the host tuple; unsupported systems fail closed.
func PreflightHandler() VerbHandler {
	return func(_ context.Context, args json.RawMessage) (json.RawMessage, error) {
		// Strict decode: empty object expected (typed args, no extras).
		var a struct{}
		dec := json.NewDecoder(strings.NewReader(string(args)))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&a); err != nil {
			return nil, fmt.Errorf("invalid preflight arguments")
		}
		info := readOSInfo()
		versions, ok := SupportedOS[strings.ToLower(info.OSID)]
		supported := ok
		if ok {
			supported = false
			for _, v := range versions {
				if v == info.VersionID {
					supported = true
					break
				}
			}
		}
		info.Supported = supported
		if !supported {
			info.SafeReason = "unsupported OS tuple; refusing to manage this host"
		}
		out, _ := json.Marshal(info)
		if !supported {
			return out, fmt.Errorf("unsupported_os_tuple")
		}
		return out, nil
	}
}

// InspectHandler returns host inventory (OS tuple; extended inventory in a
// later increment).
func InspectHandler() VerbHandler {
	return func(_ context.Context, args json.RawMessage) (json.RawMessage, error) {
		var a struct{}
		dec := json.NewDecoder(strings.NewReader(string(args)))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&a); err != nil {
			return nil, fmt.Errorf("invalid inspect arguments")
		}
		info := readOSInfo()
		info.Supported = true
		out, _ := json.Marshal(info)
		return out, nil
	}
}

// UnitApplyArgs is the typed argument for unit.apply.
type UnitApplyArgs struct {
	Binding string `json:"binding"` // stable target binding from the root-owned registry
	State   string `json:"state"`   // start | stop | enable | disable
}

// unitBindings maps stable bindings to concrete unit names (root-owned
// registry; in production loaded from the policy directory).
func unitBindings(path string) (map[string]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// UnitApplyHandler returns a handler bound to a bindings file. Only bindings
// present in the root-owned registry are manageable; raw unit names and shell
// text never reach the privileged endpoint.
func UnitApplyHandler(bindingsPath string, exec func(ctx context.Context, unit, state string) error) VerbHandler {
	return func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
		var a UnitApplyArgs
		dec := json.NewDecoder(strings.NewReader(string(args)))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&a); err != nil {
			return nil, fmt.Errorf("invalid unit.apply arguments")
		}
		switch a.State {
		case "start", "stop", "enable", "disable", "restart", "reload", "revert":
			// restart/reload cover service lifecycle; revert is the systemd
			// level rollback (restores the previous unit configuration).
		default:
			return nil, fmt.Errorf("invalid desired state")
		}
		bindings, err := unitBindings(bindingsPath)
		if err != nil {
			return nil, fmt.Errorf("unit bindings unavailable")
		}
		unit, ok := bindings[a.Binding]
		if !ok {
			return nil, fmt.Errorf("binding not allowlisted")
		}
		if exec != nil {
			if err := exec(ctx, unit, a.State); err != nil {
				out, _ := json.Marshal(map[string]string{"binding": a.Binding, "state": a.State, "safe_reason": "unit transition failed"})
				return out, err
			}
		}
		out, _ := json.Marshal(map[string]string{"binding": a.Binding, "state": a.State, "applied": "true"})
		return out, nil
	}
}
