CREATE TABLE hosts (
    hostname TEXT PRIMARY KEY COLLATE NOCASE,
    outpost_id TEXT NOT NULL REFERENCES outposts(id) ON DELETE CASCADE,
    port INTEGER NOT NULL CHECK (port BETWEEN 1 AND 65535)
);
CREATE INDEX hosts_outpost_id ON hosts(outpost_id);
