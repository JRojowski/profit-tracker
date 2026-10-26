-- name: UpsertExchangeRate :exec
INSERT INTO exchange_rates (currency, date, rate, table_number, fetched_at)
VALUES (sqlc.arg(currency), sqlc.arg(date), sqlc.arg(rate), sqlc.arg(table_number), sqlc.arg(fetched_at))
ON CONFLICT (currency, date) DO UPDATE
SET rate = excluded.rate, table_number = excluded.table_number, fetched_at = excluded.fetched_at;

-- name: UpsertExchangeRateRange :exec
-- The new range is the union of the stored range and the asked days. The asked days always touch
-- the stored range: a refresh asks from the stored last day to today, or from an earlier start
-- to the day before the stored first day.
INSERT INTO exchange_rate_ranges (currency, first_day, last_day, fetched_at)
VALUES (sqlc.arg(currency), sqlc.arg(first_day), sqlc.arg(last_day), sqlc.arg(fetched_at))
ON CONFLICT (currency) DO UPDATE
SET first_day  = min(exchange_rate_ranges.first_day, excluded.first_day),
    last_day   = max(exchange_rate_ranges.last_day, excluded.last_day),
    fetched_at = excluded.fetched_at;

-- name: ExchangeRateRanges :many
SELECT currency, first_day, last_day, fetched_at
FROM exchange_rate_ranges
ORDER BY currency;

-- name: LatestExchangeRates :many
SELECT r.currency, r.date, r.rate, r.table_number, r.fetched_at
FROM exchange_rates r
WHERE r.date = (SELECT MAX(last.date) FROM exchange_rates last WHERE last.currency = r.currency)
ORDER BY r.currency;

-- name: ExchangeRates :many
SELECT currency, date, rate, table_number
FROM exchange_rates
ORDER BY currency, date;

-- name: EarliestFlowTimes :many
-- The time of the earliest Net Deposits flow of the Accounts of each currency, as RFC 3339 text in UTC.
-- The text has no fraction of a second, so the text order is the time order.
SELECT a.currency, CAST(MIN(c.time) AS TEXT) AS first_time
FROM cash_operations c
JOIN accounts a ON a.id = c.account_id
WHERE c.type IN (sqlc.slice(types))
GROUP BY a.currency
ORDER BY a.currency;
