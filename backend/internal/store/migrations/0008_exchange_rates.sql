-- One Exchange Rate of NBP table A per currency per table date.
-- currency is the ISO code, for example 'USD'. date is the table date as YYYY-MM-DD in Polish time.
-- rate is the NBP average rate (mid) as decimal text: the PLN amount for 1 unit of the currency.
-- table_number is the NBP table number, for example '204/A/NBP/2026'. fetched_at is RFC 3339 text in UTC.
CREATE TABLE exchange_rates (
    id           INTEGER PRIMARY KEY,
    currency     TEXT NOT NULL,
    date         TEXT NOT NULL,
    rate         TEXT NOT NULL,
    table_number TEXT NOT NULL,
    fetched_at   TEXT NOT NULL,
    UNIQUE (currency, date)
) STRICT;

-- The range of days that the tool asked the NBP for, per currency. The range includes both days.
-- first_day and last_day are YYYY-MM-DD in Polish time. Inside the range, a day with no row in exchange_rates
-- is a day with no table, for example a weekend. fetched_at is the time of the last fetch, RFC 3339 text in UTC.
-- The tool records a range only for the days that the NBP answered with no error.
CREATE TABLE exchange_rate_ranges (
    currency   TEXT PRIMARY KEY,
    first_day  TEXT NOT NULL,
    last_day   TEXT NOT NULL,
    fetched_at TEXT NOT NULL
) STRICT;
