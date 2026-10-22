-- name: AccountByNumber :one
SELECT id, account_number, name, type, currency
FROM accounts
WHERE account_number = ?;

-- name: CreateAccount :one
INSERT INTO accounts (account_number, name, type, currency)
VALUES (?, ?, ?, ?)
RETURNING id;

-- name: Accounts :many
SELECT id, account_number, name, type, currency
FROM accounts
ORDER BY name, id;

-- name: AccountByID :one
SELECT id, account_number, name, type, currency
FROM accounts
WHERE id = ?;

-- name: RenameAccount :one
UPDATE accounts
SET name = ?
WHERE id = ?
RETURNING id, account_number, name, type, currency;
