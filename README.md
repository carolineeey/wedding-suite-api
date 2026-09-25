# wedding-suite-api

All-in-one wedding invitation, RSVP, and planning platform — backend, built with Go.

Multi-wedding: each one is addressed by its slug under `/api/v1/w/{slug}/`,
and every guest-facing table is scoped by `wedding_id`. Admins log in with
their own email and password and can only manage the weddings they were
granted.

## Stack

- Go 1.26, `net/http` + [gorilla/mux](https://github.com/gorilla/mux)
- PostgreSQL via [lib/pq](https://github.com/lib/pq)
- Deploys to [Render](https://render.com) (web service + managed Postgres)

## Project layout

Requests flow handler → usecase → repository:

```
cmd/api/             entrypoint: config, wiring, routes
cmd/admin/           operator CLI: create admins, grant weddings, reset passwords
internal/handlers/   HTTP only: decode requests, map errors to status codes
internal/usecase/    business rules (RSVP, invite codes, moderation); no HTTP or SQL
internal/repository/ all SQL; returns models and ErrNotFound/ErrDuplicate
internal/middleware/ logging, admin auth, rate limiting
internal/models/     shared data types
```

Usecases depend on repository interfaces they declare themselves, so their
tests (`go test ./internal/usecase`) run against in-memory fakes with no
database.

## Local development

1. Install Go 1.26+ and Postgres, or just have a Postgres connection string
   handy (e.g. a free Render Postgres instance).
2. `cp .env.example .env` and fill in `DATABASE_URL`.
3. Apply the migrations, in order (they're safe to rerun):
   ```
   for f in migrations/*.sql; do psql "$DATABASE_URL" -f "$f"; done
   ```
4. Create your admin account (prompts for a password, min 10 characters):
   ```
   go run ./cmd/admin create -email you@example.com
   ```
5. Run it:
   ```
   go run ./cmd/api
   ```
6. Log in to get a session token, then create the wedding record. Its slug
   becomes the address every other route hangs off, nothing else works until
   it exists, and you become its admin:
   ```
   TOKEN=$(curl -s -X POST localhost:8080/api/v1/auth/login \
     -H "Content-Type: application/json" \
     -d '{"email":"you@example.com","password":"..."}' | jq -r .token)

   curl -X POST localhost:8080/api/v1/admin/weddings \
     -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
     -d '{"slug":"your-slug","partner_one_name":"...","partner_two_name":"...","wedding_date":"2027-06-12"}'
   ```

### Managing admins

There is no signup or password-reset endpoint; the operator runs
`cmd/admin` against the database (`DATABASE_URL` from `.env` or the
environment; for Render, use the database's external connection string):

```
go run ./cmd/admin create -email EMAIL [-wedding SLUG]   # new admin, optionally granted a wedding
go run ./cmd/admin grant  -email EMAIL -wedding SLUG     # let an admin (e.g. a planner) manage a wedding
go run ./cmd/admin passwd -email EMAIL                   # new password; logs out every session
```

## Deploying to Render

`render.yaml` defines a free web service plus a free managed Postgres
database. From the Render dashboard: New -> Blueprint -> point it at this
repo. After the first deploy:

1. Run every file in `migrations/`, in order, against the new database
   (Render's dashboard gives you a `psql` connection command on the
   database page). The files are safe to rerun, so when new migrations
   are added later you can run them all again or just the new ones.
2. Create an admin account with `go run ./cmd/admin create` pointed at the
   Render database (see [Managing admins](#managing-admins)).
3. Log in and call `POST /api/v1/admin/weddings` once to create the wedding
   record. A wedding created before admin accounts existed has no admin yet:
   grant it with `go run ./cmd/admin grant -email EMAIL -wedding SLUG`.
4. Once the frontend has a real domain, tighten `ALLOW_ORIGINS` from `*`
   to that domain.

## API

All responses are JSON. Admin routes require `Authorization: Bearer <token>`,
with the token from `POST /api/v1/auth/login`.

- Request bodies over 64 KB are rejected with `413`.
- Invite codes are case-insensitive.
- Invite-code lookups (60 per 10 min) and public writes (30 per 10 min,
  shared between RSVPs and wishes) are rate limited per client IP; over the
  limit returns `429` with a `Retry-After` header.
- Login is limited to 10 attempts per 10 min per client IP. A wrong
  password and an unknown email both return `401` with the same message.
- Session tokens last 30 days. Logging out, or a password change via
  `cmd/admin passwd`, ends them early; a missing, expired, or ended token
  returns `401`.
- Admins only reach the weddings they were granted: any other slug under
  `/api/v1/admin/w/{slug}/` returns `404`, the same as an unknown slug.
- Set `WISHES_REQUIRE_APPROVAL=true` to hide new guestbook messages until
  they're approved via the admin API. By default they appear immediately.
- Every route that belongs to a wedding is addressed by its slug, under
  `/api/v1/w/{slug}/` (admin: `/api/v1/admin/w/{slug}/`). A slug no wedding
  holds returns `404`. Slugs are matched case-insensitively.
- Invite-code routes stay global: a code is unique across weddings and
  already identifies the guest.
- A slug another wedding already holds returns `400`, on create and on
  rename. Nothing merges into an existing wedding.

### Public

| Method | Path | Purpose |
|---|---|---|
| GET | `/health` | Liveness + DB check |
| GET | `/api/v1/w/{slug}/wedding` | Couple info, date, and full event schedule |
| GET | `/api/v1/invitations/{code}` | Everything the guest's invitation page shows: `{guest, wedding, events, gifts}`. Gift accounts are only served here, behind an invite code |
| GET | `/api/v1/guests/{code}` | Look up a guest by their invite code |
| POST | `/api/v1/guests/{code}/rsvp` | Submit an RSVP: `{attending, attending_count, message}` — `attending` is required; `message` max 1000 characters |
| GET | `/api/v1/w/{slug}/wishes` | List approved guestbook messages (`?limit=`) |
| POST | `/api/v1/w/{slug}/wishes` | Leave a guestbook message: `{guest_name, message}` (max 100 / 1000 characters); returns `{id, is_approved}` |

### Auth

| Method | Path | Purpose |
|---|---|---|
| POST | `/api/v1/auth/login` | Log in: `{email, password}`; returns `{token, expires_at, admin}` |
| POST | `/api/v1/auth/logout` | End the session whose bearer token is sent |

### Admin

| Method | Path | Purpose |
|---|---|---|
| GET | `/api/v1/admin/me` | The logged-in admin and the weddings they manage |
| POST | `/api/v1/admin/weddings` | Create a wedding; the creating admin becomes its admin. `slug` is lowercased and trimmed, and must be URL-safe (letters, numbers, single hyphens between them, max 63 characters); a slug in use returns `400` |
| PUT | `/api/v1/admin/w/{slug}/wedding` | Edit the wedding, including the invitation text (`opening_text` max 2000, `story` max 5000, `dress_code` max 200 characters). It replaces every field, so omitted text is cleared. A different `slug` in the body renames it, which breaks links already shared with guests |
| POST | `/api/v1/admin/w/{slug}/events` | Add a schedule item; optional `maps_url` must be an `http(s)` link |
| DELETE | `/api/v1/admin/w/{slug}/events/{id}` | Remove a schedule item |
| GET | `/api/v1/admin/w/{slug}/gifts` | List digital-gift accounts |
| POST | `/api/v1/admin/w/{slug}/gifts` | Add one: `{bank_name, account_name, account_number, sort_order}` |
| DELETE | `/api/v1/admin/w/{slug}/gifts/{id}` | Remove one |
| GET | `/api/v1/admin/w/{slug}/guests` | List every guest + RSVP status |
| POST | `/api/v1/admin/w/{slug}/guests` | Add a guest, returns a generated `invite_code` |
| PUT | `/api/v1/admin/w/{slug}/guests/{id}` | Edit a guest |
| DELETE | `/api/v1/admin/w/{slug}/guests/{id}` | Remove a guest |
| GET | `/api/v1/admin/w/{slug}/rsvp-summary` | Aggregate RSVP counts for a dashboard |
| GET | `/api/v1/admin/w/{slug}/wishes` | List all guestbook messages, including unapproved |
| PUT | `/api/v1/admin/w/{slug}/wishes/{id}/approval` | Hide/show a message: `{is_approved}` |
| DELETE | `/api/v1/admin/w/{slug}/wishes/{id}` | Permanently delete a message |

## What's not in the MVP yet

Deliberately out of scope for v1 -- flag if you want any of these next:
gift registry, photo gallery uploads, multiple invite languages, email/WA
invite delivery, self-service signup and password reset (both go through
`cmd/admin` for now).
