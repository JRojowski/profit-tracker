-- name: AccountByNumber :one
SELECT id, account_number, name, type, currency
FROM accounts
WHERE account_number = ?;

-- name: CreateAccount :one
INSERT INTO accounts (account_number, name, type, currency)
VALUES (?, ?, ?, ?)
RETURNING id;
