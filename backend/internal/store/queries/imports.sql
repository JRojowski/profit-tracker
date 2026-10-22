-- name: CreatePreview :one
INSERT INTO imports (account_number, file_name, file, state, created_at, new_cash_operations, skipped_cash_operations)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: PendingImport :one
SELECT id, account_number, file_name, file, state
FROM imports
WHERE id = ?;

-- name: ImportState :one
SELECT state
FROM imports
WHERE id = ?;

-- name: MarkImportSaved :execrows
UPDATE imports
SET account_id = ?, state = 'saved', imported_at = ?, file = NULL,
    new_cash_operations = ?, skipped_cash_operations = ?
WHERE id = ? AND state IN ('preview', 'needs-review');

-- name: CancelImport :execrows
UPDATE imports
SET state = 'cancelled', file = NULL
WHERE id = ? AND state IN ('preview', 'needs-review');

-- name: SavedImport :one
SELECT i.id, i.file_name, i.imported_at, a.id AS account_id, a.account_number, a.name AS account_name,
       i.new_cash_operations, i.skipped_cash_operations
FROM imports i
JOIN accounts a ON a.id = i.account_id
WHERE i.id = ? AND i.state = 'saved';

-- name: SavedImports :many
SELECT i.id, i.file_name, i.imported_at, a.id AS account_id, a.account_number, a.name AS account_name,
       i.new_cash_operations, i.skipped_cash_operations
FROM imports i
JOIN accounts a ON a.id = i.account_id
WHERE i.state = 'saved'
ORDER BY i.imported_at DESC, i.id DESC;

-- name: InsertImportConflict :exec
INSERT INTO import_conflicts (import_id, kind, xtb_id,
                              existing_type, existing_time, existing_amount, existing_comment,
                              existing_ticker, existing_position_id, existing_volume, existing_price,
                              new_type, new_time, new_amount, new_comment,
                              new_ticker, new_position_id, new_volume, new_price,
                              check_calculated, check_closed_positions)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ImportConflictKind :one
SELECT kind
FROM import_conflicts
WHERE id = ? AND import_id = ?;

-- name: ImportConflicts :many
SELECT *
FROM import_conflicts
WHERE import_id = ?
ORDER BY id;

-- name: DecideImportConflict :one
UPDATE import_conflicts
SET decision = ?
WHERE id = ? AND import_id = ?
RETURNING *;
