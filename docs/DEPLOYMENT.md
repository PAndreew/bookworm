# Production deployment

Deployed on the Potpuri VPS (46.62.236.54), in `/opt/bookworm`.
The API, frontend (Nginx), and PostgreSQL run in `compose.production.yml`.
No app or database ports are published; the frontend joins `potpuri_proxy`.
Existing services are untouched.

Create `.env` (never commit it):

```sh
printf 'POSTGRES_PASSWORD=%s\nBW_TOTP_KEY=%s\nBOOKWORM_HOST=currantfeed.cc\n' \
  "$(openssl rand -hex 24)" "$(openssl rand -hex 16)" > .env
chmod 600 .env
docker compose -f compose.production.yml up -d --build
docker compose -f compose.production.yml exec api ingest -all
```

The existing Caddy config at `/opt/potpuri/proxy/Caddyfile` has:

```caddy
currantfeed.cc {
    encode zstd gzip
    reverse_proxy bookworm-web:80
}
```

## Domain cutover still required

`currantfeed.cc` currently routes through a Cloudflare tunnel on another VPS
(46.62.240.211), and Cloudflare Access intercepts requests. It does **not** yet
route to this deployment.

Replace the apex tunnel CNAME with an A record pointing at `46.62.236.54`.
Remove stale AAAA records, if any. Temporarily use DNS-only mode so Caddy can
obtain its HTTPS certificate; then proxy again if desired. Remove or update the
Cloudflare Access application for this hostname if the site should be public.
Verify `https://currantfeed.cc/healthz` and the home page after cutover.

To change domains later, update `BOOKWORM_HOST` in `.env`, recreate the API,
and update/reload the Caddy hostname plus DNS.

## Verification

```sh
docker compose -f compose.production.yml ps
docker compose -f compose.production.yml exec web wget -qO- http://127.0.0.1/healthz
```

Back up PostgreSQL regularly; `pgdata` is persistent. Keep `.env` with your
backups: losing the TOTP encryption key makes enrolled 2FA secrets unusable.
