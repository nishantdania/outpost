ALTER TABLE outposts ADD COLUMN image_id TEXT NOT NULL DEFAULT 'default';
ALTER TABLE outposts ADD COLUMN vcpus INTEGER NOT NULL DEFAULT 2;
ALTER TABLE outposts ADD COLUMN memory_mib INTEGER NOT NULL DEFAULT 4096;
ALTER TABLE outposts ADD COLUMN disk_gib INTEGER NOT NULL DEFAULT 8;
ALTER TABLE outposts ADD COLUMN desired_state TEXT NOT NULL DEFAULT 'stopped';
ALTER TABLE outposts ADD COLUMN status TEXT NOT NULL DEFAULT 'stopped';
ALTER TABLE outposts ADD COLUMN guest_ip TEXT NOT NULL DEFAULT '';
ALTER TABLE outposts ADD COLUMN failure TEXT NOT NULL DEFAULT '';
ALTER TABLE outposts ADD COLUMN created_at TEXT NOT NULL DEFAULT '';
ALTER TABLE outposts ADD COLUMN updated_at TEXT NOT NULL DEFAULT '';
UPDATE outposts
SET created_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now'),
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE created_at = '' OR updated_at = '';
