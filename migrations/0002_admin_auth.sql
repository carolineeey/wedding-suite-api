-- Per-admin accounts replace the shared ADMIN_TOKEN, which let any admin
-- manage every wedding. Admins log in with email + password and get an
-- opaque session token; wedding_admins decides which weddings each admin may
-- manage. Accounts are created with `go run ./cmd/admin`, not over the API.

CREATE TABLE IF NOT EXISTS admins (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT UNIQUE NOT NULL, -- stored lowercased
    password_hash TEXT NOT NULL,        -- bcrypt
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS wedding_admins (
    wedding_id UUID NOT NULL REFERENCES weddings(id) ON DELETE CASCADE,
    admin_id   UUID NOT NULL REFERENCES admins(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (wedding_id, admin_id)
);

-- Only a SHA-256 of each token is stored, so a leaked table cannot be
-- replayed as live sessions.
CREATE TABLE IF NOT EXISTS admin_sessions (
    token_hash TEXT PRIMARY KEY,
    admin_id   UUID NOT NULL REFERENCES admins(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_wedding_admins_admin_id ON wedding_admins(admin_id);
CREATE INDEX IF NOT EXISTS idx_admin_sessions_admin_id ON admin_sessions(admin_id);
