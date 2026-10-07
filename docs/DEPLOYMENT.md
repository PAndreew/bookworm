# Potpuri reader deployment

The Bookworm codebase is branded **Potpuri** and runs on the Potpuri VPS
(46.62.236.54), in `/opt/bookworm`. Container/project names remain `bookworm`
to distinguish it from the original bookmarking application and preserve data.

The frontend (Nginx), API, and PostgreSQL run in `compose.production.yml`.
No reader or database ports are published; the frontend joins `potpuri_proxy`.
Secrets are in `/opt/bookworm/.env` (never commit).

```sh
docker compose -f compose.production.yml up -d --build
docker compose -f compose.production.yml exec api ingest -all
docker compose -f compose.production.yml exec web wget -qO- http://127.0.0.1/healthz
```

## Migration status

Rebranding is deployed, but the public hostname switch is **pending Tailscale
approval**. The original bookmarking app still serves `potpuri.cc` until private
access is verified. `currantfeed.cc` remains on its existing Cloudflare tunnel.

Tailscale is installed on the VPS with requested hostname `potpuri-vps` but needs
interactive enrollment. Run `tailscale status` or `tailscale up` for its login URL.
Do not enable Funnel: the original app must remain tailnet-only.

The original app has a loopback-only port at `127.0.0.1:8081`, configured by:

```sh
cd /opt/potpuri
docker compose -f compose.production.yml -f compose.private.yml up -d --no-build app
curl -fsS http://127.0.0.1:8081/health
```

Always include the private override when updating that app. The override keeps
its existing proxy connection during the transition. Its database was backed up
under `/opt/backups/potpuri-before-private-*.sql.gz` before recreating the app.

## Finish cutover after approval

1. Verify Tailscale enrollment and obtain the VPS MagicDNS hostname.
2. Enable tailnet-only HTTPS: `tailscale serve --bg http://127.0.0.1:8081`.
   HTTPS certificates may require approval in the Tailscale admin console.
3. Verify the private URL from another tailnet device, including login. Set the
   original app's `POTPURI_PUBLIC_URL` to that HTTPS URL, and update the browser
   extension's server URL. Keep secure cookies enabled.
4. Back up `/opt/potpuri/proxy/Caddyfile`. Change only the `potpuri.cc,
   www.potpuri.cc` upstream to `bookworm-web:80`. Remove the unused
   `currantfeed.cc` block added for the previous deployment attempt.
5. Set `BOOKWORM_HOST=potpuri.cc` in `/opt/bookworm/.env` and recreate the API.
   Validate and reload `potpuri-caddy` before testing public reading and login.
6. Confirm the bookmarking app is no longer reachable through a public hostname.
   Review tailnet ACLs if other tailnet users must not access it.

No domain change requires renaming containers, databases, or volumes. Do not
remove volumes. Back up both databases and their `.env` files regularly; the
reader's TOTP encryption key is required to decrypt enrolled 2FA secrets.
