-- The currency of the fee of a crypto purchase: the Account currency (EUR) or the crypto asset. See ADR 0009.
-- A fee in EUR is a part of the amount. A fee in the crypto asset decreases the volume.
-- NULL with a fee tells that the fee is in the crypto asset. The Imports before this migration stored only such fees.
ALTER TABLE cash_operations ADD COLUMN fee_currency TEXT;
