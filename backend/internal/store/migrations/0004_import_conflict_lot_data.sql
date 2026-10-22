-- An Import Conflict also compares the ticker, the position ID, the volume and the price.
-- The ticker and the position ID are '' for an operation with none. The volume and the price are
-- decimal text, or NULL for an operation with none.
-- Old rows get '' and NULL. If such a row is a purchase, its recorded data does not agree with the
-- Ledger, so its confirmation fails and the user uploads the file again.
ALTER TABLE import_conflicts ADD COLUMN existing_ticker TEXT NOT NULL DEFAULT '';
ALTER TABLE import_conflicts ADD COLUMN existing_position_id TEXT NOT NULL DEFAULT '';
ALTER TABLE import_conflicts ADD COLUMN existing_volume TEXT;
ALTER TABLE import_conflicts ADD COLUMN existing_price TEXT;
ALTER TABLE import_conflicts ADD COLUMN new_ticker TEXT NOT NULL DEFAULT '';
ALTER TABLE import_conflicts ADD COLUMN new_position_id TEXT NOT NULL DEFAULT '';
ALTER TABLE import_conflicts ADD COLUMN new_volume TEXT;
ALTER TABLE import_conflicts ADD COLUMN new_price TEXT;
