-- A Cash Operation in an Import that needs a decision from the user. See GLOSSARY.md, "Import Conflict".
-- The existing_* columns hold the stored data. The new_* columns hold the data of the file.
-- Amounts are decimal text. Times are RFC 3339 text in UTC.
-- decision is NULL until the user selects 'keep-existing' or 'use-new'.
CREATE TABLE import_conflicts (
    id               INTEGER PRIMARY KEY,
    import_id        INTEGER NOT NULL REFERENCES imports (id),
    kind             TEXT    NOT NULL CHECK (kind IN ('changed-data', 'check-mismatch')),
    xtb_id           TEXT    NOT NULL,
    existing_type    TEXT    NOT NULL,
    existing_time    TEXT    NOT NULL,
    existing_amount  TEXT    NOT NULL,
    existing_comment TEXT    NOT NULL,
    new_type         TEXT    NOT NULL,
    new_time         TEXT    NOT NULL,
    new_amount       TEXT    NOT NULL,
    new_comment      TEXT    NOT NULL,
    decision         TEXT    CHECK (decision IN ('keep-existing', 'use-new')),
    UNIQUE (import_id, kind, xtb_id)
) STRICT;

CREATE INDEX import_conflicts_import_id ON import_conflicts (import_id);
