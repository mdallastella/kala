-- When a refresh token was exchanged for a new one. Only such tokens get the
-- reuse grace window; tokens revoked by logout or a theft sweep never do.
ALTER TABLE refresh_tokens ADD COLUMN rotated_at TEXT;
