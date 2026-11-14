#!/usr/bin/env python3
"""G40: run a command on the disposable VPS over SSH (paramiko).

Credentials are NEVER stored in this file or any artifact: the caller
passes DREVANTA_VPS_HOST / DREVANTA_VPS_USER / DREVANTA_VPS_PASS in the
environment. Usage:
  DREVANTA_VPS_HOST=... DREVANTA_VPS_USER=root DREVANTA_VPS_PASS=... \
    python3 tools/deploy/vps_run.py '<remote command>' [timeout_seconds]

Exit code mirrors the remote command unless the session fails (then 255).
"""
import os
import sys

import paramiko


def main() -> int:
    if len(sys.argv) < 2:
        print("usage: vps_run.py '<remote command>' [timeout]", file=sys.stderr)
        return 2
    cmd = sys.argv[1]
    timeout = float(sys.argv[2]) if len(sys.argv) > 2 else 120.0
    host = os.environ["DREVANTA_VPS_HOST"]
    user = os.environ.get("DREVANTA_VPS_USER", "root")
    password = os.environ["DREVANTA_VPS_PASS"]
    port = int(os.environ.get("DREVANTA_VPS_PORT", "22"))

    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    try:
        client.connect(host, port=port, username=user, password=password,
                       timeout=15, banner_timeout=15, auth_timeout=15)
        stdin, stdout, stderr = client.exec_command(cmd, timeout=timeout)
        out = stdout.read().decode("utf-8", "replace")
        err = stderr.read().decode("utf-8", "replace")
        code = stdout.channel.recv_exit_status()
        if out:
            sys.stdout.write(out)
        if err:
            sys.stderr.write(err)
        return code if code >= 0 else 0
    except Exception as e:  # noqa: BLE001 - report any session failure
        print(f"SESSION ERROR: {type(e).__name__}: {e}", file=sys.stderr)
        return 255
    finally:
        client.close()


if __name__ == "__main__":
    sys.exit(main())
