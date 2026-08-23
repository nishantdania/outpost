CREATE TABLE arks (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL
, image_id TEXT NOT NULL DEFAULT 'default', vcpus INTEGER NOT NULL DEFAULT 2, memory_mib INTEGER NOT NULL DEFAULT 4096, disk_gib INTEGER NOT NULL DEFAULT 8, desired_state TEXT NOT NULL DEFAULT 'stopped', status TEXT NOT NULL DEFAULT 'stopped', guest_ip TEXT NOT NULL DEFAULT '', failure TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL DEFAULT '', updated_at TEXT NOT NULL DEFAULT '', ssh_public_key TEXT NOT NULL DEFAULT '');

CREATE TABLE image_tags (
  tag TEXT PRIMARY KEY,
  digest TEXT NOT NULL REFERENCES images(digest) ON DELETE CASCADE,
  updated_at TEXT NOT NULL
);

CREATE TABLE images (
  digest TEXT PRIMARY KEY,
  size_bytes INTEGER NOT NULL,
  created_at TEXT NOT NULL
);

CREATE TABLE schema_migrations (
  version INTEGER PRIMARY KEY,
  applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
