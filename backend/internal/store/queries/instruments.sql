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

-- name: ResetPriceCheck :one
-- A new price ticker needs a new price check. The answers and the confirmation of the old price ticker do not apply.
UPDATE instruments
SET source_currency = NULL, price_no_data = 0, price_confirmed = 0
WHERE ticker = ?
RETURNING *;

-- name: SetQuoteCurrency :one
-- A change of the Quote Currency removes the confirmation of the user. The same value keeps it.
UPDATE instruments
SET quote_currency        = sqlc.narg(quote_currency),
    quote_currency_manual = sqlc.arg(quote_currency_manual),
    price_confirmed       = CASE WHEN quote_currency IS sqlc.narg(quote_currency) THEN price_confirmed ELSE 0 END
WHERE ticker = sqlc.arg(ticker)
RETURNING *;

-- name: SetPriceConfirmed :one
UPDATE instruments
SET price_confirmed = ?
WHERE ticker = ?
RETURNING *;

-- name: SetSourceCurrency :exec
-- An answer with a price tells the currency of the price source, and that the price source has data.
UPDATE instruments
SET source_currency = ?, price_no_data = 0
WHERE ticker = ?;

-- name: SetPriceNoData :exec
UPDATE instruments
SET price_no_data = 1
WHERE ticker = ?;
