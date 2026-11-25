-- name: Tiers :many
SELECT id, order_number, name, target_share, shared_part_name
FROM tiers
ORDER BY order_number;

-- name: TierInstruments :many
SELECT ticker, tier_id, target_share
FROM tier_instruments
ORDER BY ticker;

-- name: DeleteTiers :exec
DELETE FROM tiers;

-- name: InsertTier :one
INSERT INTO tiers (order_number, name, target_share, shared_part_name)
VALUES (?, ?, ?, ?)
RETURNING id;

-- name: InsertTierInstrument :exec
INSERT INTO tier_instruments (ticker, tier_id, target_share)
VALUES (?, ?, ?);

-- name: TierAccountInstruments :many
-- The Instruments with one or more Fills on a Tier Account: a regular XTB Account in PLN or USD.
SELECT DISTINCT i.ticker, i.name
FROM lots l
JOIN accounts a ON a.id = l.account_id
JOIN instruments i ON i.id = l.instrument_id
WHERE a.type = 'regular' AND a.provider = 'xtb' AND a.currency IN ('PLN', 'USD')
ORDER BY i.ticker;
