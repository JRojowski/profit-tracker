-- One daily close of a Benchmark per price date. See GLOSSARY.md, "Benchmark".
-- A Benchmark is not an Instrument, so benchmark_id is the identifier from the list in the code, for example 'sp500'.
-- date is the price date as YYYY-MM-DD in the exchange time. close is decimal text in the currency.
-- ADR 0008: the close is split-adjusted. The tool stores only a close in the Quote Currency of the Benchmark.
-- source is the price provider, for example 'yahoo'. fetched_at is RFC 3339 text in UTC.
CREATE TABLE benchmark_prices (
    benchmark_id TEXT NOT NULL,
    date         TEXT NOT NULL,
    close        TEXT NOT NULL,
    currency     TEXT NOT NULL,
    source       TEXT NOT NULL,
    fetched_at   TEXT NOT NULL,
    PRIMARY KEY (benchmark_id, date)
) STRICT;

-- The range of days that the tool asked the price source for, per Benchmark. The range includes both days.
-- first_day and last_day are YYYY-MM-DD. Inside the range, a day with no row in benchmark_prices is a day
-- with no close, for example a holiday. fetched_at is the time of the last fetch, RFC 3339 text in UTC.
-- The tool records a range only for the days that the price source answered with no error and in the Quote Currency.
CREATE TABLE benchmark_price_ranges (
    benchmark_id TEXT PRIMARY KEY,
    first_day    TEXT NOT NULL,
    last_day     TEXT NOT NULL,
    fetched_at   TEXT NOT NULL
) STRICT;
