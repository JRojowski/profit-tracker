-- name: InstrumentPriceSources :many
SELECT *
FROM instruments
ORDER BY ticker;

-- name: InstrumentPriceSource :one
SELECT *
FROM instruments
WHERE ticker = ?;

-- name: SetPriceTicker :one
UPDATE instruments
SET price_ticker = sqlc.narg(price_ticker)
WHERE ticker = sqlc.arg(ticker)
RETURNING *;

-- name: SetQuoteCurrency :one
UPDATE instruments
SET quote_currency = sqlc.narg(quote_currency), quote_currency_manual = sqlc.arg(quote_currency_manual)
WHERE ticker = sqlc.arg(ticker)
RETURNING *;
