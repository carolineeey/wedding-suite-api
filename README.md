# wedding-suite-api

All-in-one wedding invitation, RSVP, and planning platform — backend, built with Go.

Built for one wedding today, but every guest-facing table is scoped by
`wedding_id`, so it can grow into a multi-couple platform later without a
schema rewrite.

## Stack

- Go 1.22, `net/http` + [gorilla/mux](https://github.com/gorilla/mux)
- PostgreSQL via [lib/pq](https://github.com/lib/pq)
- Deploys to [Render](https://render.com) (web service + managed Postgres)

## Local development

1. Install Go 1.22+ and Postgres, or just have a Postgres connection string
   handy (e.g. a free Render Postgres instance).
2. `cp .env.example .env` and fill in `DATABASE_URL` and `ADMIN_TOKEN`.
3. Apply the schema:
   ```
   psql "$DATABASE_URL" -f migrations/0001_init.sql
   ```
4. Run it:
   ```
   go run ./cmd/api
   ```
5. Create the (one) wedding record — nothing else works until this exists:
   ```
   curl -X PUT localhost:8080/api/v1/admin/wedding \
     -H "Authorization: Bearer $ADMIN_TOKEN" -H "Content-Type: application/json" \
     -d '{"slug":"your-slug","partner_one_name":"...","partner_two_name":"...","wedding_date":"2027-06-12"}'
   ```

## Deploying to Render

`render.yaml` defines a free web service plus a free managed Postgres
database. From the Render dashboard: New -> Blueprint -> point it at this
repo. After the first deploy:

1. Run `migrations/0001_init.sql` against the new database (Render's
   dashboard gives you a `psql` connection command on the database page).
2. Set `ADMIN_TOKEN` in the service's environment settings (`render.yaml`
   deliberately leaves it out of source control -- generate one with
   `openssl rand -hex 24`).
3. Call `PUT /api/v1/admin/wedding` once to create the wedding record.
4. Once the frontend has a real domain, tighten `ALLOW_ORIGINS` from `*`
   to that domain.

## API

All responses are JSON. Admin routes require `Authorization: Bearer $ADMIN_TOKEN`.

### Public

| Method | Path | Purpose |
|---|---|---|
| GET | `/health` | Liveness + DB check |
| GET | `/api/v1/wedding` | Couple info, date, and full event schedule |
| GET | `/api/v1/guests/{code}` | Look up a guest by their invite code |
| POST | `/api/v1/guests/{code}/rsvp` | Submit an RSVP: `{attending, attending_count, message}` |
| GET | `/api/v1/wishes` | List approved guestbook messages (`?limit=`) |
| POST | `/api/v1/wishes` | Leave a guestbook message: `{guest_name, message}` |

### Admin

| Method | Path | Purpose |
|---|---|---|
| PUT | `/api/v1/admin/wedding` | Create/update the wedding record |
| POST | `/api/v1/admin/events` | Add a schedule item |
| DELETE | `/api/v1/admin/events/{id}` | Remove a schedule item |
| GET | `/api/v1/admin/guests` | List every guest + RSVP status |
| POST | `/api/v1/admin/guests` | Add a guest, returns a generated `invite_code` |
| PUT | `/api/v1/admin/guests/{id}` | Edit a guest |
| DELETE | `/api/v1/admin/guests/{id}` | Remove a guest |
| GET | `/api/v1/admin/rsvp-summary` | Aggregate RSVP counts for a dashboard |
| GET | `/api/v1/admin/wishes` | List all guestbook messages, including unapproved |
| PUT | `/api/v1/admin/wishes/{id}/approval` | Hide/show a message: `{is_approved}` |
| DELETE | `/api/v1/admin/wishes/{id}` | Permanently delete a message |

## What's not in the MVP yet

Deliberately out of scope for v1 -- flag if you want any of these next:
gift registry, photo gallery uploads, multiple invite languages, email/WA
invite delivery, real per-user auth (today it's a single shared admin
token, which is fine for a couple running their own wedding).
