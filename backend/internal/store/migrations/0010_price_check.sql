-- The state of the price check of an Instrument. See GLOSSARY.md, "Price Warning".
-- source_currency is the currency that the price source gave in its last answer with a price, after the change
-- of a price in pence (GBp) to GBP. NULL tells that the tool does not know it.
-- price_no_data tells that the last answer of the price source was "no data" for the price ticker. 1 is "no data".
-- price_confirmed tells that the user confirmed that the price ticker and the Quote Currency are correct.
-- The confirmation removes only the Price Warning "price mismatch". 1 is confirmed.
-- A change of the price ticker clears the three columns. A change of the Quote Currency clears price_confirmed.
ALTER TABLE instruments ADD COLUMN source_currency TEXT;
ALTER TABLE instruments ADD COLUMN price_no_data INTEGER NOT NULL DEFAULT 0 CHECK (price_no_data IN (0, 1));
ALTER TABLE instruments ADD COLUMN price_confirmed INTEGER NOT NULL DEFAULT 0 CHECK (price_confirmed IN (0, 1));
