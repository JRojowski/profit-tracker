-- An Import Conflict of the kind 'check-mismatch' is a new sale whose Realized Profit differs by more than
-- 0.01 from the Profit/Loss in the Closed Positions sheet. See ADR 0001.
-- Such a conflict has no stored data: the existing_* columns are NULL. The new_* columns hold the sale.
-- check_calculated is the Realized Profit from the Ledger. check_closed_positions is the broker value.
-- Both are decimal text. The only decision for this kind is 'accept'.
-- SQLite cannot change a CHECK constraint or a NOT NULL column. Thus this migration builds the table again.
CREATE TABLE import_conflicts_new (
    id                     INTEGER PRIMARY KEY,
    import_id              INTEGER NOT NULL REFERENCES imports (id),
    kind                   TEXT    NOT NULL CHECK (kind IN ('changed-data', 'check-mismatch')),
    xtb_id                 TEXT    NOT NULL,
    existing_type          TEXT,
    existing_time          TEXT,
    existing_amount        TEXT,
    existing_comment       TEXT,
    existing_ticker        TEXT,
    existing_position_id   TEXT,
    existing_volume        TEXT,
    existing_price         TEXT,
    new_type               TEXT    NOT NULL,
    new_time               TEXT    NOT NULL,
    new_amount             TEXT    NOT NULL,
    new_comment            TEXT    NOT NULL,
    new_ticker             TEXT    NOT NULL DEFAULT '',
    new_position_id        TEXT    NOT NULL DEFAULT '',
    new_volume             TEXT,
    new_price              TEXT,
    check_calculated       TEXT,
    check_closed_positions TEXT,
    decision               TEXT    CHECK (decision IN ('keep-existing', 'use-new', 'accept')),
    UNIQUE (import_id, kind, xtb_id),
    CHECK (kind <> 'changed-data' OR (
        existing_type IS NOT NULL AND existing_time IS NOT NULL AND existing_amount IS NOT NULL
        AND existing_comment IS NOT NULL AND existing_ticker IS NOT NULL AND existing_position_id IS NOT NULL
        AND (decision IS NULL OR decision IN ('keep-existing', 'use-new')))),
    CHECK (kind <> 'check-mismatch' OR (
        check_calculated IS NOT NULL AND check_closed_positions IS NOT NULL
        AND (decision IS NULL OR decision = 'accept')))
) STRICT;

INSERT INTO import_conflicts_new (id, import_id, kind, xtb_id,
                                  existing_type, existing_time, existing_amount, existing_comment,
                                  existing_ticker, existing_position_id, existing_volume, existing_price,
                                  new_type, new_time, new_amount, new_comment,
                                  new_ticker, new_position_id, new_volume, new_price, decision)
SELECT id, import_id, kind, xtb_id,
       existing_type, existing_time, existing_amount, existing_comment,
       existing_ticker, existing_position_id, existing_volume, existing_price,
       new_type, new_time, new_amount, new_comment,
       new_ticker, new_position_id, new_volume, new_price, decision
FROM import_conflicts;

DROP TABLE import_conflicts;

ALTER TABLE import_conflicts_new RENAME TO import_conflicts;

CREATE INDEX import_conflicts_import_id ON import_conflicts (import_id);
