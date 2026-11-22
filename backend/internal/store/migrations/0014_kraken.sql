-- The fee of a crypto purchase in the crypto asset, as decimal text. See ADR 0009.
-- NULL tells that the Cash Operation has no fee in a crypto asset, for example each XTB Cash Operation.
ALTER TABLE cash_operations ADD COLUMN fee TEXT;

-- The default price ticker and the default Quote Currency that the parser of the Provider gives,
-- for example 'BTC-EUR' and 'EUR' for the Kraken asset 'BTC'. An Import sets them only when it adds the Instrument.
-- NULL tells that the tool uses the default mapping from the suffix of the XTB ticker (price.Ticker and price.QuoteCurrency).
-- The manual price ticker (price_ticker) and a manual Quote Currency have priority over these values.
ALTER TABLE instruments ADD COLUMN default_price_ticker TEXT;
ALTER TABLE instruments ADD COLUMN default_quote_currency TEXT;
