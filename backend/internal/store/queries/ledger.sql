-- name: KnownCashOperations :many
SELECT xtb_id, type, time, amount, comment
FROM cash_operations
WHERE account_id = ?;

-- name: CashOperationByXTBID :one
SELECT id, type, time, amount, comment
FROM cash_operations
WHERE account_id = ? AND xtb_id = ?;

-- name: UpdateCashOperation :exec
UPDATE cash_operations
SET type = ?, instrument_id = ?, time = ?, amount = ?, comment = ?, position_id = ?, volume = ?, price = ?, import_id = ?
WHERE id = ?;

-- name: DeleteLotFill :exec
DELETE FROM lot_fills
WHERE cash_operation_id = ?;

-- name: DeleteEmptyLots :exec
DELETE FROM lots
WHERE account_id = ? AND id NOT IN (SELECT lot_id FROM lot_fills);

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

-- name: LedgerCashOperations :many
SELECT type, amount, position_id
FROM cash_operations
WHERE account_id = ?
ORDER BY time, id;

-- name: LedgerFills :many
SELECT l.position_id, i.ticker, i.name AS instrument_name, c.volume, c.amount
FROM lots l
JOIN instruments i ON i.id = l.instrument_id
JOIN lot_fills f ON f.lot_id = l.id
JOIN cash_operations c ON c.id = f.cash_operation_id
WHERE l.account_id = ?
ORDER BY l.position_id, c.time, c.id;

-- name: MixedLots :many
SELECT l.position_id
FROM lots l
JOIN lot_fills f ON f.lot_id = l.id
JOIN cash_operations c ON c.id = f.cash_operation_id
WHERE l.account_id = ?
GROUP BY l.id, l.position_id
HAVING COUNT(DISTINCT c.instrument_id) > 1
ORDER BY l.position_id;

-- name: SetLotInstruments :exec
UPDATE lots
SET instrument_id = (
    SELECT c.instrument_id
    FROM lot_fills f
    JOIN cash_operations c ON c.id = f.cash_operation_id
    WHERE f.lot_id = lots.id
    LIMIT 1
)
WHERE lots.account_id = ?;
