-- name: InsertBenchmarkPrice :exec
-- A stored close never changes, as a close of the daily history of an Instrument.
INSERT INTO benchmark_prices (benchmark_id, date, close, currency, source, fetched_at)
VALUES (sqlc.arg(benchmark_id), sqlc.arg(date), sqlc.arg(close), sqlc.arg(currency), sqlc.arg(source),
        sqlc.arg(fetched_at))
ON CONFLICT (benchmark_id, date) DO NOTHING;

-- name: UpsertBenchmarkPriceRange :exec
-- The new range is the union of the stored range and the asked days. The asked days always touch the stored range.
INSERT INTO benchmark_price_ranges (benchmark_id, first_day, last_day, fetched_at)
VALUES (sqlc.arg(benchmark_id), sqlc.arg(first_day), sqlc.arg(last_day), sqlc.arg(fetched_at))
ON CONFLICT (benchmark_id) DO UPDATE
SET first_day  = min(benchmark_price_ranges.first_day, excluded.first_day),
    last_day   = max(benchmark_price_ranges.last_day, excluded.last_day),
    fetched_at = excluded.fetched_at;

-- name: BenchmarkPriceRanges :many
SELECT benchmark_id, first_day, last_day, fetched_at
FROM benchmark_price_ranges
ORDER BY benchmark_id;

-- name: BenchmarkPrices :many
SELECT benchmark_id, date, close, currency
FROM benchmark_prices
ORDER BY benchmark_id, date;

-- name: FirstCashOperationTime :one
-- The time of the first Cash Operation of all Accounts, as RFC 3339 text in UTC, or an empty text if there is none.
-- The text has no fraction of a second, so the text order is the time order.
SELECT CAST(COALESCE(MIN(time), '') AS TEXT) AS first_time
FROM cash_operations;
