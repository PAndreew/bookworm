#!/usr/bin/env bash
# Run locally: bash ops/setup-resend.sh root@YOUR_VPS
# Or on VPS as root: bash ops/setup-resend.sh
set -euo pipefail
set +x
umask 077
if [[ $# -gt 1 ]]; then echo "Usage: $0 [root@VPS]" >&2; exit 1; fi
IFS= read -r -s -p 'Resend API key (hidden): ' key </dev/tty
printf '\n' >/dev/tty
if [[ ! "$key" =~ ^re_[A-Za-z0-9_-]+$ ]]; then echo 'Invalid Resend key format.' >&2; exit 1; fi
# Key travels only through stdin; never through command arguments or shell history.
install_command='set -eu; umask 077; test "$(id -u)" = 0 || { echo "Run as root" >&2; exit 1; }; mkdir -p /opt/bookworm-secrets; chmod 700 /opt/bookworm-secrets; tmp=$(mktemp /opt/bookworm-secrets/.resend.XXXXXX); trap '\''rm -f "$tmp"'\'' EXIT; cat > "$tmp"; chown root:10001 "$tmp"; chmod 640 "$tmp"; mv "$tmp" /opt/bookworm-secrets/resend_key'
if [[ $# -eq 1 ]]; then
  printf '%s' "$key" | ssh "$1" "$install_command"
else
  printf '%s' "$key" | bash -c "$install_command"
fi
unset key
printf '%s\n' 'Key installed securely. After deploying the updated code, run on the VPS:' 'cd /opt/bookworm && docker compose -f compose.production.yml up -d --build api web' 'Disable Resend click/open tracking for authentication emails.'
