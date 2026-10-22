-- One security that the broker sells. See GLOSSARY.md, "Instrument".
CREATE TABLE instruments (
    id       INTEGER PRIMARY KEY,
    ticker   TEXT    NOT NULL UNIQUE,
    name     TEXT    NOT NULL,
    category TEXT    NOT NULL
) STRICT;

-- One upload of one broker export file. See GLOSSARY.md, "Import".
-- A row in the state 'preview' holds the file until the user confirms it.
-- It has no Account until the confirmation, because the Account can be new.
CREATE TABLE imports (
    id                      INTEGER PRIMARY KEY,
    account_id              INTEGER REFERENCES accounts (id),
    account_number          TEXT    NOT NULL,
    file_name               TEXT    NOT NULL,
    file                    BLOB,
    state                   TEXT    NOT NULL CHECK (state IN ('preview', 'needs-review', 'saved', 'cancelled')),
    created_at              TEXT    NOT NULL,
    imported_at             TEXT,
    new_cash_operations     INTEGER NOT NULL DEFAULT 0,
    skipped_cash_operations INTEGER NOT NULL DEFAULT 0
) STRICT;

-- One entry of the Ledger of an Account. See GLOSSARY.md, "Cash Operation".
-- Amounts, volumes and prices are decimal text. Times are RFC 3339 text in UTC.
CREATE TABLE cash_operations (
    id            INTEGER PRIMARY KEY,
    account_id    INTEGER NOT NULL REFERENCES accounts (id),
    xtb_id        TEXT    NOT NULL,
    type          TEXT    NOT NULL,
    instrument_id INTEGER REFERENCES instruments (id),
    time          TEXT    NOT NULL,
    amount        TEXT    NOT NULL,
    comment       TEXT    NOT NULL,
    position_id   TEXT,
    volume        TEXT,
    price         TEXT,
    import_id     INTEGER NOT NULL REFERENCES imports (id),
    UNIQUE (account_id, xtb_id)
) STRICT;

-- One or more Fills that the broker tracks as one unit. See GLOSSARY.md, "Lot".
-- position_id is the XTB position ID, which is the Lot ID.
CREATE TABLE lots (
    id            INTEGER PRIMARY KEY,
    account_id    INTEGER NOT NULL REFERENCES accounts (id),
    position_id   TEXT    NOT NULL,
    instrument_id INTEGER NOT NULL REFERENCES instruments (id),
    UNIQUE (account_id, position_id)
) STRICT;

-- The Fills of a Lot. Each Fill is one purchase Cash Operation. See GLOSSARY.md, "Fill".
CREATE TABLE lot_fills (
    lot_id            INTEGER NOT NULL REFERENCES lots (id),
    cash_operation_id INTEGER NOT NULL UNIQUE REFERENCES cash_operations (id),
    PRIMARY KEY (lot_id, cash_operation_id)
) STRICT;
