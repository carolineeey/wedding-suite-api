-- 0001_init.sql
-- Core schema. Every guest-facing table carries a wedding_id so a second
-- wedding can be onboarded later without touching the schema.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS weddings (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug             TEXT UNIQUE NOT NULL,
    partner_one_name TEXT NOT NULL,
    partner_two_name TEXT NOT NULL,
    wedding_date     DATE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS events (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    wedding_id UUID NOT NULL REFERENCES weddings(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    starts_at  TIMESTAMPTZ NOT NULL,
    ends_at    TIMESTAMPTZ,
    venue_name TEXT,
    address    TEXT,
    notes      TEXT,
    sort_order INT NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS guests (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    wedding_id        UUID NOT NULL REFERENCES weddings(id) ON DELETE CASCADE,
    invite_code       TEXT UNIQUE NOT NULL,
    name              TEXT NOT NULL,
    group_name        TEXT,
    max_guests        INT NOT NULL DEFAULT 1,
    rsvp_status       TEXT NOT NULL DEFAULT 'pending'
                          CHECK (rsvp_status IN ('pending', 'attending', 'declined')),
    attending_count   INT NOT NULL DEFAULT 0,
    rsvp_message      TEXT,
    rsvp_responded_at TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS wishes (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    wedding_id  UUID NOT NULL REFERENCES weddings(id) ON DELETE CASCADE,
    guest_name  TEXT NOT NULL,
    message     TEXT NOT NULL,
    is_approved BOOLEAN NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_events_wedding_id ON events(wedding_id);
CREATE INDEX IF NOT EXISTS idx_guests_wedding_id ON guests(wedding_id);
CREATE INDEX IF NOT EXISTS idx_guests_invite_code ON guests(invite_code);
CREATE INDEX IF NOT EXISTS idx_wishes_wedding_id ON wishes(wedding_id);
