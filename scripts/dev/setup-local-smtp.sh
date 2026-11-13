#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$root_dir"
umask 077
mkdir -p secrets

if [[ ! -f secrets/local-smtp.crt || ! -f secrets/local-smtp.key ]]; then
  openssl req -x509 -newkey rsa:2048 -nodes -sha256 -days 365 \
    -subj '/CN=mailpit' \
    -addext 'subjectAltName=DNS:mailpit,DNS:localhost' \
    -keyout secrets/local-smtp.key -out secrets/local-smtp.crt >/dev/null 2>&1
fi
chmod 600 secrets/local-smtp.key
chmod 644 secrets/local-smtp.crt

if [[ ! -f secrets/auth-smtp-password ]]; then
  openssl rand -hex 32 > secrets/auth-smtp-password
fi
if [[ ! -f secrets/auth-invitation-key ]]; then
  openssl rand -hex 32 > secrets/auth-invitation-key
fi
chmod 600 secrets/auth-smtp-password secrets/auth-invitation-key

if [[ ! -f .env ]]; then
  cp .env.example .env
fi

add_env_if_missing() {
  local name="$1" value="$2"
  if ! grep -q "^${name}=" .env; then
    printf '%s=%s\n' "$name" "$value" >> .env
  fi
}

add_env_if_missing AUTH_SMTP_ADDRESS mailpit:1025
add_env_if_missing AUTH_SMTP_FROM invite@reservehub.local
add_env_if_missing AUTH_SMTP_USERNAME local-invite
add_env_if_missing AUTH_SMTP_PASSWORD_HOST_FILE ./secrets/auth-smtp-password
add_env_if_missing AUTH_INVITATION_ENCRYPTION_KEY_HOST_FILE ./secrets/auth-invitation-key
add_env_if_missing AUTH_SMTP_CA_FILE /run/secrets/local-smtp.crt
add_env_if_missing AUTH_INVITATION_LINK_BASE_URL https://localhost:8443/invitations/accept

printf 'Local SMTP is configured. Open https://localhost:8443 and http://localhost:8025 after starting Compose.\n'
