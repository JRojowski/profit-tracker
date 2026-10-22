# 10: Manual price ticker mapping

**What to build:** The user sets the price source ticker for one Instrument when the default mapping is wrong. The tool then uses this ticker for that Instrument.

**Blocked by:** 09 (Prices and Unrealized Profit).

**Status:** ready-for-agent

- [ ] `PUT /api/instruments/{ticker}/price-ticker` stores the manual ticker. An empty value removes it.
- [ ] The price service uses the manual ticker before the default mapping.
- [ ] The Holdings table lets the user edit the price ticker of an Instrument.
- [ ] An API test with the fake provider checks that the manual ticker is used.
