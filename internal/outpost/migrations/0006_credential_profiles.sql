-- Profiles contain host secret names, never secret values. Each VM pins a copy.
ALTER TABLE images ADD COLUMN credential_config TEXT NOT NULL DEFAULT '';
ALTER TABLE outposts ADD COLUMN credential_config TEXT NOT NULL DEFAULT '';
