-- daily tells where a price comes from. 1 is a close from the daily history of the price source.
-- 0 is a current price. A current price of today can be a live price during the session.
-- ADR 0005: the tool never changes a close of the daily history. A close of the daily history replaces
-- a current price that the tool fetched on or before its price date. That price can be a live price.
-- Only a current price tells that the current price of an Instrument is current.
ALTER TABLE prices ADD COLUMN daily INTEGER NOT NULL DEFAULT 0 CHECK (daily IN (0, 1));

-- The range of days that the tool asked the price source for, per Instrument. The range includes both days.
-- first_day and last_day are YYYY-MM-DD. Inside the range, a day with no row in prices is a day with no close.
-- An example is a holiday. fetched_at is the time of the last fetch, RFC 3339 text in UTC.
-- The tool records a range only for the days that the price source answered with no error.
-- A change of the price ticker deletes the range and the prices of the Instrument.
CREATE TABLE price_ranges (
    instrument_id INTEGER PRIMARY KEY REFERENCES instruments (id),
    first_day     TEXT NOT NULL,
    last_day      TEXT NOT NULL,
    fetched_at    TEXT NOT NULL
) STRICT;
