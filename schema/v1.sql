-- Speclite schema v1
-- Applied when PRAGMA user_version = 0

PRAGMA journal_mode = WAL;
PRAGMA foreign_keys = ON;

CREATE TABLE specs (
  id          TEXT    PRIMARY KEY,
  title       TEXT    NOT NULL,
  kind        TEXT    NOT NULL,
  status      TEXT    NOT NULL DEFAULT 'draft'
              CHECK (status IN ('draft','review','fixed','implemented','verified','deprecated')),
  version     INTEGER NOT NULL DEFAULT 1,
  body        TEXT    NOT NULL DEFAULT '',
  hash        TEXT    NOT NULL,
  created_at  TEXT    NOT NULL,
  updated_at  TEXT    NOT NULL
);

CREATE INDEX idx_specs_kind   ON specs(kind);
CREATE INDEX idx_specs_status ON specs(status);

CREATE TABLE relations (
  from_id   TEXT NOT NULL REFERENCES specs(id) ON DELETE CASCADE,
  relation  TEXT NOT NULL
            CHECK (relation IN ('depends_on','implements','verifies','supersedes','related_to')),
  to_id     TEXT NOT NULL REFERENCES specs(id) ON DELETE RESTRICT,
  PRIMARY KEY (from_id, relation, to_id)
);

CREATE INDEX idx_relations_to_id ON relations(to_id);

CREATE TABLE constraints (
  id          TEXT PRIMARY KEY,
  target_id   TEXT NOT NULL REFERENCES specs(id) ON DELETE CASCADE,
  language    TEXT NOT NULL,
  expression  TEXT NOT NULL,
  created_at  TEXT NOT NULL
);

CREATE INDEX idx_constraints_target ON constraints(target_id);

CREATE TABLE event_log (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  event_type   TEXT    NOT NULL,
  spec_id      TEXT,
  payload_json TEXT    NOT NULL DEFAULT '{}',
  created_at   TEXT    NOT NULL
);

CREATE INDEX idx_event_log_spec_id    ON event_log(spec_id);
CREATE INDEX idx_event_log_event_type ON event_log(event_type);
CREATE INDEX idx_event_log_created_at ON event_log(created_at);

CREATE VIRTUAL TABLE specs_fts USING fts5(
  id    UNINDEXED,
  title,
  body,
  content='specs',
  content_rowid='rowid'
);

CREATE TRIGGER specs_ai AFTER INSERT ON specs BEGIN
  INSERT INTO specs_fts(rowid, id, title, body)
  VALUES (new.rowid, new.id, new.title, new.body);
END;

CREATE TRIGGER specs_ad AFTER DELETE ON specs BEGIN
  INSERT INTO specs_fts(specs_fts, rowid, id, title, body)
  VALUES ('delete', old.rowid, old.id, old.title, old.body);
END;

CREATE TRIGGER specs_au AFTER UPDATE ON specs BEGIN
  INSERT INTO specs_fts(specs_fts, rowid, id, title, body)
  VALUES ('delete', old.rowid, old.id, old.title, old.body);
  INSERT INTO specs_fts(rowid, id, title, body)
  VALUES (new.rowid, new.id, new.title, new.body);
END;

PRAGMA user_version = 1;
