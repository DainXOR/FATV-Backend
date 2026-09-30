# FATV authentication setup

## Required deployment configuration

Add these values to the backend environment (see `src/.env.example`):

- `SMTP_HOST`, `SMTP_PORT`, `SMTP_FROM`
- `SMTP_USERNAME` and `SMTP_PASSWORD` when the SMTP server requires authentication
- `FRONTEND_BASE_URL` (for setup/recovery links and student invitations)
- `FRONTEND_ORIGIN` (allowed credentialed browser origin)
- `SESSION_COOKIE_SECURE=true` when serving over HTTPS

SMTP is not configured in the current backend `.env`. Account setup, recovery, and invitation emails will not work until valid SMTP settings are supplied. The application does not log SMTP credentials or email codes.

## Initial administrator

Set `FATV_BOOTSTRAP_ADMIN_EMAIL` to the intended administrator's email before the first startup. When the auth account collection is empty, startup creates one pending Admin and emails a single-use setup code. Once any platform account exists, bootstrap is skipped. Remove the variable after successful bootstrap.

Do not create a default password or expose a public bootstrap endpoint.

## API summary

- `POST /api/v1/auth/login`, `POST /api/v1/auth/logout`, `GET /api/v1/auth/me`
- `POST /api/v1/auth/setup`
- `POST /api/v1/auth/recovery/request`, `POST /api/v1/auth/recovery/complete`
- Admin-only: `/api/v1/auth/accounts` and `/api/v1/auth/settings`
- Staff/Admin: `/api/v1/form-invitations`
- Invitation-scoped public form API: `/api/v1/public/forms/:token`

Sessions use server-side records and HttpOnly cookies. Unsafe authenticated requests require the double-submit CSRF cookie/header pair. Student invitations are random, per-student, expiring, revocable, and single-submission.

## Validation

From `src`, run `go test ./...`. From the frontend root, run `npm test -- --watchAll=false` and `npm run build`.
