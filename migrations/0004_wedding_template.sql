-- A stable template ID is shared by admin settings, public pages, and invitations.
-- Existing weddings start with the first moodboard template.
ALTER TABLE weddings
    ADD COLUMN IF NOT EXISTS template_id TEXT NOT NULL DEFAULT 'ulos-heritage';
