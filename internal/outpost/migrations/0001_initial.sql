CREATE TABLE outposts (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    image_id TEXT NOT NULL DEFAULT 'default',
    vcpus INTEGER NOT NULL DEFAULT 2,
    memory_mib INTEGER NOT NULL DEFAULT 4096,
    disk_gib INTEGER NOT NULL DEFAULT 8,
    desired_state TEXT NOT NULL DEFAULT 'stopped',
    status TEXT NOT NULL DEFAULT 'stopped',
    guest_ip TEXT NOT NULL DEFAULT '',
    failure TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX outposts_name_unique ON outposts(name COLLATE NOCASE);
CREATE UNIQUE INDEX outpost_guest_ip ON outposts(guest_ip) WHERE guest_ip != '';

CREATE TABLE vm_runtime (
    outpost_id TEXT PRIMARY KEY REFERENCES outposts(id) ON DELETE RESTRICT,
    tap TEXT NOT NULL DEFAULT '',
    gateway TEXT NOT NULL DEFAULT '',
    pid INTEGER NOT NULL DEFAULT 0,
    start_time TEXT NOT NULL DEFAULT '',
    ufw_route INTEGER NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX runtime_tap ON vm_runtime(tap) WHERE tap != '';
CREATE UNIQUE INDEX runtime_gateway ON vm_runtime(gateway) WHERE gateway != '';

CREATE TABLE images (
    digest TEXT PRIMARY KEY,
    size_bytes INTEGER NOT NULL,
    created_at TEXT NOT NULL
);
CREATE TABLE image_tags (
    tag TEXT PRIMARY KEY,
    digest TEXT NOT NULL REFERENCES images(digest) ON DELETE CASCADE,
    updated_at TEXT NOT NULL
);
