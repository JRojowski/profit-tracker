# 06: Lots from multiple Fills

**What to build:** The tool builds Lots from the Fills of fractional orders by ADR 0002. It reads the Lot volumes from Closed Positions and Open Positions (IDs and volumes only, never prices) and selects case A, B or C. When the Fills of an order match no case, the Import stops with an error.

**Blocked by:** 05 (Sales and Realized Profit).

**Status:** ready-for-agent

- [ ] The parser reads purchase Comments with the order volume (`OPEN BUY 0.6169/1.6169 @ 47.755` and `OPEN BUY FPS.US 1/1.27 @ 39.24`).
- [ ] The reader returns the Lot IDs and volumes from Open Positions.
- [ ] Case A: Fills with one Position ID give one Lot.
- [ ] Case B: one ID with the full order volume and other IDs with volume 0 give one merged Lot.
- [ ] Case C: each ID with the volume of its own Fills gives one Lot per ID.
- [ ] Any other result stops the Import with an error that names the Instrument and the order.
- [ ] API tests reproduce ADR 0002 Examples A (volume 1.27, amount 49.87), B (profit 108.45, open volume 1.1773, open cost 240.96) and C (profits 0.18 and 0.44).
