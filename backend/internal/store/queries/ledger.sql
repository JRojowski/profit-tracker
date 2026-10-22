-- name: CashOperationIDs :many
SELECT xtb_id
FROM cash_operations
WHERE account_id = ?;

-- name: UpsertInstrument :one
INSERT INTO instruments (ticker, name, category)
VALUES (?, ?, ?)
ON CONFLICT (ticker) DO UPDATE SET ticker = excluded.ticker
RETURNING id;

-- name: InsertCashOperation :one
INSERT INTO cash_operations (account_id, xtb_id, type, instrument_id, time, amount, comment, position_id, volume, price, import_id)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (account_id, xtb_id) DO NOTHING
RETURNING id;

-- name: UpsertLot :one
INSERT INTO lots (account_id, position_id, instrument_id)
VALUES (?, ?, ?)
ON CONFLICT (account_id, position_id) DO UPDATE SET position_id = excluded.position_id
RETURNING id;

-- name: InsertLotFill :exec
INSERT INTO lot_fills (lot_id, cash_operation_id)
VALUES (?, ?);
