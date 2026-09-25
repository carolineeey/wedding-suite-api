-- Content for the invitation page guests open from their personal link
-- (/u/{invite_code}): free text the admin writes, a map link per event, and
-- the accounts guests can send a digital gift to. Every column is nullable
-- or defaulted, so existing rows need no backfill.

ALTER TABLE weddings
    ADD COLUMN IF NOT EXISTS opening_text TEXT, -- greeting, verse, or quote at the top
    ADD COLUMN IF NOT EXISTS story        TEXT, -- the couple's story
    ADD COLUMN IF NOT EXISTS dress_code   TEXT;

ALTER TABLE events
    ADD COLUMN IF NOT EXISTS maps_url TEXT;

-- Shown only on the invitation page, which needs a valid invite code, never
-- on the public wedding route.
CREATE TABLE IF NOT EXISTS gift_accounts (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    wedding_id     UUID NOT NULL REFERENCES weddings(id) ON DELETE CASCADE,
    bank_name      TEXT NOT NULL, -- bank or e-wallet, e.g. "BCA", "GoPay"
    account_name   TEXT NOT NULL,
    account_number TEXT NOT NULL,
    sort_order     INT NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_gift_accounts_wedding_id ON gift_accounts(wedding_id);
