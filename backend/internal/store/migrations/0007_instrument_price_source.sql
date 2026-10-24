-- The manual price ticker of an Instrument, for example 'VUSA.L'.
-- The user sets it when the default mapping (price.Ticker) gives a wrong price ticker.
-- NULL tells that the tool uses the default mapping. An Import never changes it.
ALTER TABLE instruments ADD COLUMN price_ticker TEXT;

-- quote_currency_manual tells that quote_currency is a manual value. 1 is manual, 0 is the default from the suffix.
-- The tool keeps one quote_currency column and a flag, not a second nullable column. Thus the price service
-- and the Holdings read one value, and only the display needs the flag.
-- An Import sets quote_currency only when it adds the Instrument, so it never overwrites a manual value.
ALTER TABLE instruments ADD COLUMN quote_currency_manual INTEGER NOT NULL DEFAULT 0 CHECK (quote_currency_manual IN (0, 1));
