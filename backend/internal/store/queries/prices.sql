-- name: UpsertPrice :exec
-- A current price replaces a stored current price of the same day. It never replaces a close of the daily history.
INSERT INTO prices (instrument_id, date, close, currency, source, fetched_at)
VALUES ((SELECT i.id FROM instruments i WHERE i.ticker = sqlc.arg(ticker)),
        sqlc.arg(date), sqlc.arg(close), sqlc.arg(currency), sqlc.arg(source), sqlc.arg(fetched_at))
ON CONFLICT (instrument_id, date) DO UPDATE
SET close = excluded.close, currency = excluded.currency, source = excluded.source, fetched_at = excluded.fetched_at
WHERE prices.daily = 0;

-- name: InsertDailyPrice :exec
-- A close of the daily history replaces a stored current price that the tool fetched on or before its price date
-- in UTC. That price can be a live price during the session. A close keeps a current price that the tool
-- fetched after its price date. That price is the close, and it tells that the current price is current.
-- A close never replaces a close of the daily history (ADR 0005).
INSERT INTO prices (instrument_id, date, close, currency, source, fetched_at, daily)
VALUES ((SELECT i.id FROM instruments i WHERE i.ticker = sqlc.arg(ticker)),
        sqlc.arg(date), sqlc.arg(close), sqlc.arg(currency), sqlc.arg(source), sqlc.arg(fetched_at), 1)
ON CONFLICT (instrument_id, date) DO UPDATE
SET close = excluded.close, currency = excluded.currency, source = excluded.source, fetched_at = excluded.fetched_at,
    daily = 1
WHERE prices.daily = 0 AND substr(prices.fetched_at, 1, 10) <= prices.date;

-- name: UpsertPriceRange :exec
-- The new range is the union of the stored range and the asked days. The asked days always touch the stored range.
-- A refresh asks from the day after the stored last day to yesterday.
-- It also asks from an earlier first Fill to the day before the stored first day.
INSERT INTO price_ranges (instrument_id, first_day, last_day, fetched_at)
VALUES ((SELECT i.id FROM instruments i WHERE i.ticker = sqlc.arg(ticker)),
        sqlc.arg(first_day), sqlc.arg(last_day), sqlc.arg(fetched_at))
ON CONFLICT (instrument_id) DO UPDATE
SET first_day  = min(price_ranges.first_day, excluded.first_day),
    last_day   = max(price_ranges.last_day, excluded.last_day),
    fetched_at = excluded.fetched_at;

-- name: PriceRanges :many
SELECT i.ticker, r.first_day, r.last_day, r.fetched_at
FROM price_ranges r
JOIN instruments i ON i.id = r.instrument_id
ORDER BY i.ticker;

-- name: DeleteInstrumentPrices :exec
DELETE FROM prices
WHERE instrument_id = (SELECT i.id FROM instruments i WHERE i.ticker = ?);

-- name: DeleteInstrumentPriceRange :exec
DELETE FROM price_ranges
WHERE instrument_id = (SELECT i.id FROM instruments i WHERE i.ticker = ?);

-- name: LastPrices :many
SELECT i.ticker, p.date, p.close, p.currency, p.source, p.fetched_at
FROM prices p
JOIN instruments i ON i.id = p.instrument_id
WHERE p.date = (SELECT MAX(last.date) FROM prices last WHERE last.instrument_id = p.instrument_id)
ORDER BY i.ticker;

-- name: Prices :many
SELECT i.ticker, p.date, p.close, p.currency, p.source, p.fetched_at
FROM prices p
JOIN instruments i ON i.id = p.instrument_id
ORDER BY i.ticker, p.date;

-- name: TickersFetchedSince :many
-- Only a current price tells that the current price is current. A close of the daily history does not.
SELECT DISTINCT i.ticker
FROM prices p
JOIN instruments i ON i.id = p.instrument_id
WHERE p.fetched_at >= ? AND p.daily = 0
ORDER BY i.ticker;

-- name: FirstFillTimes :many
-- The time of the first Fill of each Instrument on any Account, as RFC 3339 text in UTC.
-- The text has no fraction of a second, so the text order is the time order.
SELECT i.ticker, CAST(MIN(c.time) AS TEXT) AS first_time
FROM lot_fills f
JOIN lots l ON l.id = f.lot_id
JOIN instruments i ON i.id = l.instrument_id
JOIN cash_operations c ON c.id = f.cash_operation_id
GROUP BY i.ticker
ORDER BY i.ticker;

-- name: TradePrices :many
-- The XTB price of one unit of each Fill and each sale on any Account, with the time of the Cash Operation.
-- The Instrument of a sale is the Instrument of the Lot that it closes.
SELECT i.ticker, c.time, c.price
FROM lot_fills f
JOIN lots l ON l.id = f.lot_id
JOIN instruments i ON i.id = l.instrument_id
JOIN cash_operations c ON c.id = f.cash_operation_id
WHERE c.price IS NOT NULL
UNION ALL
SELECT i.ticker, c.time, c.price
FROM cash_operations c
JOIN lots l ON l.account_id = c.account_id AND l.position_id = c.position_id
JOIN instruments i ON i.id = l.instrument_id
WHERE c.type = 'Stock sell' AND c.price IS NOT NULL
ORDER BY 1, 2;
