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
