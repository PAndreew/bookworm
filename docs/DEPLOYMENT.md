# Potpuri deployment

The Bookworm codebase is branded **Potpuri** and serves **https://potpuri.cc**.
`www.potpuri.cc` redirects to the canonical hostname. It runs on the Potpuri VPS
(46.62.236.54), in `/opt/bookworm`. Container/project names remain `bookworm`
to distinguish it from the original bookmarking app and preserve existing data.

## Public reader

The frontend (Nginx), API, and PostgreSQL run in `compose.production.yml`.
No reader or database ports are published; the frontend joins `potpuri_proxy`.
Database/TOTP secrets are in `/opt/bookworm/.env` (never commit), with
`BOOKWORM_HOST=potpuri.cc`. Keep its TOTP encryption key with backups.
OpenRouter uses a separate encrypted VPS credential vault, not `.env`:
see [ASSISTANT.md](ASSISTANT.md). Production Compose requires the decrypted
secret at `/run/potpuri-reader-secrets/openrouter_api_key`; restore/configure
its encrypted credential and enable `potpuri-reader-secrets.service` before
starting the stack on a new VPS.

```sh
cd /opt/bookworm
docker compose -f compose.production.yml up -d --build
docker compose -f compose.production.yml exec api ingest -all
curl -fsS https://potpuri.cc/healthz
```

The existing Caddy instance (`potpuri-caddy`) routes the public reader through
`/opt/potpuri/proxy/Caddyfile`:

```caddy
www.potpuri.cc {
    redir https://potpuri.cc{uri} permanent
}
potpuri.cc {
    encode zstd gzip
    request_body {
        max_size 64MB
    }
    reverse_proxy bookworm-web:80
    header {
        Permissions-Policy "camera=(), microphone=(), geolocation=()"
        X-Frame-Options "DENY"
    }
}
```

Validate before reloading:

```sh
docker exec potpuri-caddy caddy validate --config /etc/caddy/Caddyfile
docker exec potpuri-caddy caddy reload --config /etc/caddy/Caddyfile
```

## Original bookmarking app: private only

**https://potpuri-vps.tailf0acdd.ts.net/** is accessible only from the tailnet.
Tailscale Serve proxies HTTPS to the original app at `127.0.0.1:8081`.
Funnel is not enabled. Its original login is still required; sessions from the
old domain do not transfer. Set browser extensions to this private server URL.
Review tailnet ACLs if other tailnet users must not access it.

The app is still in `/opt/potpuri` with its original database and volumes.
`POTPURI_PUBLIC_URL` points to the private HTTPS URL, and secure cookies remain
on. Always include its private override when updating:

```sh
cd /opt/potpuri
docker compose -f compose.production.yml -f compose.private.yml up -d --build app
curl -fsS http://127.0.0.1:8081/health
tailscale serve status
# Restore Serve if needed (persistent background configuration):
tailscale serve --bg http://127.0.0.1:8081
```

The override publishes only loopback port `127.0.0.1:8081`, never a public port.
It retains existing Docker proxy networks; no public Caddy route targets this
app. Do not enable Tailscale Funnel or restore a public upstream to it.

## Backups and verification

The bookmarking database was backed up before migration under
`/opt/backups/potpuri-before-private-*.sql.gz`. Previous `.env` and Caddy configs
are saved alongside them as `*.before-domain-switch` and
`Caddyfile.before-reader-switch`. These contain secrets; keep them private.
Back up both databases and their `.env` files regularly. Back up the encrypted
OpenRouter credential together with its host encryption key to independently
encrypted storage, as described in ASSISTANT.md. Never remove volumes.

Verified after cutover:
- Private HTTPS certificate and original app health/login page.
- Public reader homepage, branding assets, library and reader URLs.
- Public signup, secure-cookie session, saved page/resume, and logout.
- Public API health and canonical www redirect.

The original account's authenticated private login requires the owner's
credentials and was not tested. `currantfeed.cc` remains on its existing
Cloudflare tunnel; the unused route on this VPS has been removed.
