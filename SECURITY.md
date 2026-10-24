# Security

## Reporting a vulnerability

Please report security issues privately through GitHub's
[private vulnerability reporting](https://github.com/Ezo-mas/Watcht/security/advisories/new)
rather than in a public issue.

## Security model

watcht is a self-hosted app for one person or a household. It assumes:

- It runs behind HTTPS, usually through a reverse proxy, when exposed beyond
  the local network.
- All accounts are created by the instance admin; there is no public
  sign-up.
- The data directory is readable only by the watcht process and the host
  admin.

## Changes that need a security review

Any change to the following needs a security review (`/security-review`)
before it is considered done:

- Password hashing, login, logout, first-run setup, or account management
  (`internal/auth/`)
- Session cookies, session storage, or expiry
- API tokens: creation, hashing, lookup, scopes, or revocation
- CSRF protection (the `Origin` check) or CORS
- Webhook authentication (`/api/v1/webhooks/`)
- OIDC sign-in and account linking
- Request logging and redaction
- The trusted-proxy and client-IP logic
- Rate limiting on login
- File uploads (importers) and the image cache: path handling and size
  limits
- Outbound requests built from user input (provider URLs, server addresses
  for Jellyfin/Plex)
- The Dockerfile's user, permissions, or base image
