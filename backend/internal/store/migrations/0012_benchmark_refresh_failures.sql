-- The Benchmarks whose last refresh of the closes failed. See GLOSSARY.md, "Benchmark".
-- A row means that the last refresh failed, with an error of the price source or with a close in a currency that
-- is not the Quote Currency. A successful refresh of the Benchmark deletes its row. A refresh that asks for no day
-- changes nothing. reason is the text of the failure. failed_at is RFC 3339 text in UTC.
CREATE TABLE benchmark_refresh_failures (
    benchmark_id TEXT PRIMARY KEY,
    reason       TEXT NOT NULL,
    failed_at    TEXT NOT NULL
) STRICT;
