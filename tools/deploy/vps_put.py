#!/usr/bin/env python3
"""G40: upload a file to the disposable VPS over SFTP (paramiko).

Credentials only via environment (see vps_run.py). Usage:
  ... vps_put.py <local path> <remote path>
"""
import os
import sys

import paramiko


def main() -> int:
    if len(sys.argv) < 3:
        print("usage: vps_put.py <local> <remote>", file=sys.stderr)
        return 2
    host = os.environ["DREVANTA_VPS_HOST"]
    user = os.environ.get("DREVANTA_VPS_USER", "root")
    password = os.environ["DREVANTA_VPS_PASS"]
    port = int(os.environ.get("DREVANTA_VPS_PORT", "22"))
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    try:
        client.connect(host, port=port, username=user, password=password,
                       timeout=15, banner_timeout=15, auth_timeout=15)
        sftp = client.open_sftp()
        sftp.put(sys.argv[1], sys.argv[2])
        st = sftp.stat(sys.argv[2])
        print(f"uploaded {sys.argv[1]} -> {sys.argv[2]} ({st.st_size} bytes)")
        return 0
    except Exception as e:  # noqa: BLE001
        print(f"UPLOAD ERROR: {type(e).__name__}: {e}", file=sys.stderr)
        return 255
    finally:
        client.close()


if __name__ == "__main__":
    sys.exit(main())
