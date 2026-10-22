# 06: Lots from multiple Fills

**What to build:** The tool builds Lots from the Fills of fractional orders by ADR 0002. It reads the Lot volumes from Closed Positions and Open Positions (IDs and volumes only, never prices) and selects case A, B or C. When the Fills of an order match no case, the Import stops with an error.

**Blocked by:** 05 (Sales and Realized Profit).

**Status:** ready-for-agent

- [x] The parser reads purchase Comments with the order volume (`OPEN BUY 0.6169/1.6169 @ 47.755` and `OPEN BUY FPS.US 1/1.27 @ 39.24`).
- [x] The reader returns the Lot IDs and volumes from Open Positions.
- [x] Case A: Fills with one Position ID give one Lot.
- [x] Case B: one ID with the full order volume and other IDs with volume 0 give one merged Lot.
- [x] Case C: each ID with the volume of its own Fills gives one Lot per ID.
- [x] Any other result stops the Import with an error that names the Instrument and the order.
- [x] API tests reproduce ADR 0002 Examples A (volume 1.27, amount 49.87), B (profit 108.45, open volume 1.1773, open cost 240.96) and C (profits 0.18 and 0.44).
