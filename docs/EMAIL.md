# Transactional email

Potpuri uses Resend from `Potpuri <noreply@potpuri.cc>`. Registration alerts go to
`thepandras@gmail.com` and contain only handle and UTC registration time. Imported
and seeded accounts do not trigger alerts. These alerts indicate signup, not verification.

## Install and deploy

Domain `potpuri.cc` must be verified in Resend. Use a sending-only API key scoped
to that domain. Disable click/open tracking for authentication emails.

From a trusted local terminal in this repository:

```bash
bash ops/setup-resend.sh root@YOUR_VPS
```

Or run `bash ops/setup-resend.sh` as root on the VPS. The script prompts without
echo and sends the key through SSH stdin, not arguments/history. It atomically
writes `/opt/bookworm-secrets/resend_key` with owner root, group 10001, and mode
0640 in a root-only directory. Group 10001 matches the non-root API container;
Docker Compose bind-mounted secrets retain host permissions.
It does not deploy, restart, print the key, or send a test email.

Deploy the updated repository to `/opt/bookworm`, then run on the VPS:

```bash
cd /opt/bookworm
docker compose -f compose.production.yml up -d --build api web
```

The API automatically applies migration 0011. Production Compose requires the
secret file; install it before deployment. It survives reboot (unlike `/run`).
Back up the key only to encrypted storage. Rotation: rerun the script and recreate
the API container, then revoke the old key in Resend.

Development email is disabled unless `BW_RESEND_KEY_FILE` points to a key file.
Optional settings: `BW_EMAIL_FROM`, `BW_SIGNUP_NOTIFY` (production Compose sets
these explicitly), and `BW_PUBLIC_URL`. Production email links require HTTPS.

## Behaviour and security

- Signup queues confirmation email and an owner alert; signup redirects to the
  confirmation instructions. Reading remains available without verification.
- Profile shows a resend link for unverified users; login links to password reset.
- Verification expires in 24 hours, reset in 30 minutes. Tokens are random,
  hashed in `email_tokens`, purpose-bound, single-use, and atomically redeemed.
- Links carry tokens in fragments, not server-visible URL queries. Opening a link
  does nothing until the user submits; scanner GET requests cannot redeem tokens.
- Reset revokes all sessions and sends a password-changed notice. TOTP and its
  recovery codes remain unchanged; reset never automatically signs a user in.
- Request endpoints return the same message for unknown and eligible accounts.
  IP limits use the existing auth limiter; per-account issuance has a persistent
  one-minute cooldown. As with the current limiter, multiple API replicas have
  separate IP budgets.
- PostgreSQL outbox retries with exponential backoff and Resend idempotency keys.
  Delivered messages and expired queued messages are deleted. Authentication
  messages stop retrying when their links expire; alerts retry for up to 24 hours.
- **Outbox bodies contain plaintext bearer links until delivery/expiry.** Restrict
  database and backup access, use encrypted backups, and never dump bodies into
  logs. Token-table hashes alone cannot be used to redeem links.
- Queue failures are logged without addresses or tokens. Users can request another
  confirmation email; owner-alert queue failures are not automatically repaired.

## Verification

Go suite and frontend build:

```bash
cd api && go test ./...
cd ../web && npm run build
```

Database integration test (use a disposable PostgreSQL database, never production):

```bash
cd api
BW_EMAIL_TEST_DATABASE_URL='postgres://user:password@localhost:5432/test?sslmode=disable' \
  go test ./internal/store -run TestEmail -v
```

Covers verification, wrong-purpose rejection, replay, cooldown, expired tokens,
password changes, session revocation, provider failures/retries, and outbox cleanup.
Provider calls use a local test server, not Resend.

After deployment, manually test signup/confirmation, notification receipt, resend,
and password reset, including a 2FA account. Confirm Resend delivery status and
check spam. Never share token links, secret files, or database dumps in chat.
