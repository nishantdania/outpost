ALTER TABLE arks ADD COLUMN image_id TEXT NOT NULL DEFAULT 'default';
ALTER TABLE arks ADD COLUMN vcpus INTEGER NOT NULL DEFAULT 2;
ALTER TABLE arks ADD COLUMN memory_mib INTEGER NOT NULL DEFAULT 4096;
ALTER TABLE arks ADD COLUMN disk_gib INTEGER NOT NULL DEFAULT 8;
ALTER TABLE arks ADD COLUMN desired_state TEXT NOT NULL DEFAULT 'stopped';
ALTER TABLE arks ADD COLUMN status TEXT NOT NULL DEFAULT 'stopped';
ALTER TABLE arks ADD COLUMN guest_ip TEXT NOT NULL DEFAULT '';
ALTER TABLE arks ADD COLUMN failure TEXT NOT NULL DEFAULT '';
ALTER TABLE arks ADD COLUMN created_at TEXT NOT NULL DEFAULT '';
ALTER TABLE arks ADD COLUMN updated_at TEXT NOT NULL DEFAULT '';
UPDATE arks
SET created_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now'),
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE created_at = '' OR updated_at = '';
