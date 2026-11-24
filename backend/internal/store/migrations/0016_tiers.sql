-- One Tier of Rebalancing. See GLOSSARY.md, "Tier".
-- target_share is a decimal percentage as text, for example '25' or '12.5'.
-- shared_part_name is the name of the Shared Part. It is empty if the user gave none.
CREATE TABLE tiers (
    id               INTEGER PRIMARY KEY,
    order_number     INTEGER NOT NULL UNIQUE,
    name             TEXT    NOT NULL UNIQUE,
    target_share     TEXT    NOT NULL,
    shared_part_name TEXT    NOT NULL DEFAULT ''
) STRICT;

-- The current assignment of an Instrument to a Tier. An Instrument has one Tier at most.
-- A target_share makes the Instrument an Instrument Target. NULL puts it in the Shared Part.
-- The deletion of a Tier deletes its assignments.
CREATE TABLE tier_instruments (
    ticker       TEXT    NOT NULL PRIMARY KEY REFERENCES instruments (ticker),
    tier_id      INTEGER NOT NULL REFERENCES tiers (id) ON DELETE CASCADE,
    target_share TEXT
) STRICT;
