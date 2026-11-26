# certs — local HTTPS

Run once (Git Bash) before `docker compose up`:

```bash
bash certs/generate-certs.sh
```

This creates a private CA (`ca.crt`, `ca.key`) and a server certificate (`server.crt`, `server.key`)
signed by it, valid for `localhost`, `127.0.0.1`, `api-gateway` and `frontend`.
All of these files are git-ignored.

| Who | Uses |
| :--- | :--- |
| nginx (frontend, `https://localhost:3000`) | `server.crt` + `server.key`; verifies the gateway with `ca.crt` |
| api-gateway (`https://localhost:8443`) | `server.crt` + `server.key` |
| browser / Postman / curl | must trust `ca.crt` |

## Trusting the CA

- **Windows (Chrome / Edge):** double-click `certs\ca.crt` → *Install Certificate* → *Current User* →
  *Place all certificates in the following store* → **Trusted Root Certification Authorities**.
  Or from PowerShell: `certutil -user -addstore Root certs\ca.crt`. Restart the browser.
- **Firefox:** Settings → Privacy & Security → Certificates → View Certificates → Authorities → Import `ca.crt`.
- **Postman:** Settings → Certificates → CA Certificates → select `certs/ca.crt`.
- **curl:** `curl --cacert certs/ca.crt https://localhost:8443/...`. On Windows (Git Bash curl uses Schannel) add
  `--ssl-no-revoke`: a private CA has no revocation list, so Schannel's revocation check would fail even though
  the certificate chain itself is valid.

To remove it later: `certmgr.msc` → Trusted Root Certification Authorities → delete "Stashr Local Dev CA".
