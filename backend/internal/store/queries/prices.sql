-- name: UpsertPrice :exec
INSERT INTO prices (instrument_id, date, close, currency, source, fetched_at)
VALUES ((SELECT i.id FROM instruments i WHERE i.ticker = sqlc.arg(ticker)),
        sqlc.arg(date), sqlc.arg(close), sqlc.arg(currency), sqlc.arg(source), sqlc.arg(fetched_at))
ON CONFLICT (instrument_id, date) DO UPDATE
SET close = excluded.close, currency = excluded.currency, source = excluded.source, fetched_at = excluded.fetched_at;

-- name: InstrumentQuoteCurrencies :many
SELECT ticker, quote_currency
FROM instruments
ORDER BY ticker;

-- name: LastPrices :many
SELECT i.ticker, p.date, p.close, p.currency, p.source, p.fetched_at
FROM prices p
JOIN instruments i ON i.id = p.instrument_id
WHERE p.date = (SELECT MAX(last.date) FROM prices last WHERE last.instrument_id = p.instrument_id)
ORDER BY i.ticker;

-- name: TickersFetchedSince :many
SELECT DISTINCT i.ticker
FROM prices p
JOIN instruments i ON i.id = p.instrument_id
WHERE p.fetched_at >= ?
ORDER BY i.ticker;
