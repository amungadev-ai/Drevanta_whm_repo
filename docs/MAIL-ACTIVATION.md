# Mail Activation Runbook (G30)

The `mail.apply` helper verb provisions the DATA (domains, DKIM keypairs, passdb
entries, maildirs). This runbook documents the host-level activation that connects
the provisioned data to the running mail servers. It was executed and verified on
the drill VPS (see DEPLOY-DRILL-G30-G50.md D2).

## 1. What the verb provisions (per domain / mailbox)

| Artifact | Path (mail root binding) | Notes |
| --- | --- | --- |
| Virtual domain | `postfix/vmaildomains` | one domain per line, idempotent |
| DKIM keypair | `dkim/<domain>/mail.private` (0600) + `mail.txt` | RSA-2048; `mail.txt` holds the DNS TXT record to publish |
| Passdb entry | `dovecot/users` (0600) | `user@domain:{PLAIN}…` from a root-owned file/env ref — never logged |
| Maildir | `vmail/<domain>/<local>/Maildir/{cur,new,tmp}` | created 0700; removal preserves stored mail |

## 2. Postfix activation (per host, once + per new domain)

```
# domains file is the verb-managed one (publish a copy readable by postfix)
postconf -e "virtual_mailbox_domains = <activation copy of vmaildomains>"
postconf -e "virtual_mailbox_base  = <mail root>/vmail"
postconf -e "virtual_mailbox_maps  = hash:<activation map: user@domain → domain/local/Maildir/>"
postconf -e "virtual_uid_maps     = static:65534"
postconf -e "virtual_gid_maps     = static:65534"
postconf -e "virtual_transport    = virtual"
postmap <activation map>          # after every mailbox batch
systemctl reload postfix
```

Notes learned on 26.04:
- The postfix daemon (unprivileged) must be able to TRAVERSE every directory of the
  maps' path and READ the `.db` — keep activation copies under `/etc/postfix/…`.
- `virtual_mailbox_base` must be traversable to uid 65534 (the delivery uid).
- After map changes, a full `systemctl restart postfix` (or flush after restart)
  clears cached lookup failures.

## 3. Dovecot activation (2.4 flattened grammar)

```
# /etc/dovecot/conf.d/99-drevanta.conf
passdb_driver = passwd-file
passwd_file_path = <mail root>/dovecot/users        # make group-readable by dovenull
passdb_default_password_scheme = PLAIN
mail_driver = maildir
mail_path = <mail root>/vmail/%d/%n/Maildir
protocols = lmtp                                   # +imap once dovecot-imapd is present
```

- `dovecot-lmtpd` is a SEPARATE package on 26.04 (install when using lmtp transport).
- Permissions: passdb file `root:dovenull 0640`, parent dirs traversable.
- IMAP auth wiring (userdb) is the remaining item — delivery does not require it.

## 4. DKIM signing (opendkim)

- Publish `mail.txt` contents as DNS TXT `mail._domainkey.<domain>`.
- opendkim key table: `<domain> <mail root>/dkim/<domain>/mail.private`
  (signing activation is part of the domain-activation copy; key material never
  leaves the root-owned tree).

## 5. Verification (executed in the drill)

1. Send: `printf "To: postmaster@<domain>\nSubject: …\n\nbody" | sendmail -t`
2. Expect: file in `<mail root>/vmail/<domain>/postmaster/Maildir/new/` with the
   subject line present.
3. `mailq` empty; `journalctl -t postfix/virtual` shows `status=sent`.
