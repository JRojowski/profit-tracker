-- One broker Account. See GLOSSARY.md, "Account".
CREATE TABLE accounts (
    id             INTEGER PRIMARY KEY,
    account_number TEXT    NOT NULL UNIQUE,
    name           TEXT    NOT NULL,
    type           TEXT    NOT NULL CHECK (type IN ('regular', 'IKE', 'IKZE')),
    currency       TEXT    NOT NULL
) STRICT;
