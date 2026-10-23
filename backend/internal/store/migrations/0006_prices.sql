-- The quote currency of an Instrument is the currency of its price at the exchange.
-- NULL tells that the tool does not know it. The tool shows no value for such an Instrument.
-- An Import sets it from the ticker suffix when it adds the Instrument. A later Import does not change it.
ALTER TABLE instruments ADD COLUMN quote_currency TEXT;

-- The same suffix rule as price.QuoteCurrency fills the Instruments that exist.
UPDATE instruments
SET quote_currency = CASE substr(ticker, -3)
    WHEN '.US' THEN 'USD'
    WHEN '.PL' THEN 'PLN'
    WHEN '.DE' THEN 'EUR'
    WHEN '.UK' THEN 'GBP'
END;

-- One close price of an Instrument per price date.
-- date is the price date as YYYY-MM-DD in the exchange time. close is decimal text in the currency.
-- source is the price provider, for example 'yahoo'. fetched_at is RFC 3339 text in UTC.
CREATE TABLE prices (
    id            INTEGER PRIMARY KEY,
    instrument_id INTEGER NOT NULL REFERENCES instruments (id),
    date          TEXT    NOT NULL,
    close         TEXT    NOT NULL,
    currency      TEXT    NOT NULL,
    source        TEXT    NOT NULL,
    fetched_at    TEXT    NOT NULL,
    UNIQUE (instrument_id, date)
) STRICT;
