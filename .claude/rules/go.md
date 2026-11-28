---
paths:
  - "backend/**/*.go"
  - "backend/**/*.sql"
  - "backend/go.mod"
---

# Go code standard

The backend is one Go module in `backend/`. Write idiomatic Go: the code that an experienced Go reviewer expects to see, with the standard library first.

Fix the cause of a lint issue. Use `//nolint:<linter> // <reason>` only when the linter is wrong for that line, and write the reason.

## Stack

| Concern | Use |
|---|---|
| HTTP | `net/http` with `ServeMux` method patterns, for example `"GET /api/accounts/{id}"` |
| Logging | `log/slog` |
| SQLite driver | `modernc.org/sqlite` (pure Go, so the binary builds with no cgo) |
| Queries | `sqlc` with the `sqlite` engine. Commit the generated code and edit only the `.sql` source. |
| Migrations | Numbered `.sql` files, embedded with `embed`, applied at start. `PRAGMA user_version` stores the applied number. |
| Money and volume | `github.com/shopspring/decimal`. Store as text in SQLite. |
| xlsx | `github.com/xuri/excelize/v2` |
| Tests | `testing` with `github.com/stretchr/testify` (`require` and `assert`) |

Add a new module only when the standard library cannot do the job in a few lines. Write the reason in the commit message.

## Money and time

- Use `decimal.Decimal` for every amount, price, volume and rate. A `float64` for money or volume is a bug.
- Convert xlsx cell values to `decimal.Decimal` from their text, not through `float64`.
- Keep times in UTC as `time.Time`. Convert to local time only in the frontend.
- Send each time in the API as RFC3339 UTC text. Do not cut a time to a date in Go, because the UTC date can be one day before the Polish date. The frontend makes the date.

## Layout

```
backend/
  cmd/profit-tracker/   main: config, wiring, start
  cmd/demo-exports/     writes the synthetic exports for `make demo`
  internal/<package>/   one package per domain concept
```

- Name packages after the domain concept, with the `GLOSSARY.md` term. The packages now are `xtb`, `lot`, `ledger`, `importer`, `price`, `pricecheck`, `exchange`, `returns`, `valuehistory`, `ikze`, `allaccounts`, `allocation`, `benchmark`, `store`, `httpapi`, `web` and `demo`.
  - `kraken` reads the Kraken ledger CSV and returns its Cash Operations.
  - `kraken/krakentest` builds synthetic Kraken ledger CSV files for the tests.
- Give a package a name that tells what it holds. `util`, `common`, `models` and `helpers` tell nothing.
- Use names that do not repeat the package: `ledger.Calculate`, `price.Provider`.
- Keep the domain packages free of HTTP and SQL. `httpapi` and `store` are the adapters.

## Errors

- Return errors. Use `panic` only in `main` for a broken setup.
- Wrap with context and `%w`: `fmt.Errorf("read cash operations sheet: %w", err)`.
- Write error strings in lowercase with no final punctuation.
- Declare sentinel errors as `var ErrUnknownOperationType = errors.New("unknown cash operation type")`. Check them with `errors.Is`. Check error types with `errors.As`.
- Give a domain error the data the user needs, for example the operation type and the row number.
- Handle the error first and return early. Keep the normal path at the left edge.

## Interfaces and functions

- Declare an interface in the package that uses it, with only the methods that package calls.
- Add an interface only at a real seam. There are two seams: `price.Provider` (v1) and `exchange.Provider` (v2). A type with one implementation stays a concrete type.
- Accept interfaces and return concrete types.
- Pass `ctx context.Context` as the first parameter of every function that does I/O. Keep a context out of structs.
- Prefer a pure function for calculations. The Ledger calculator takes data and returns data.

## Naming and comments

- Write initialisms in one case: `ID`, `XTB`, `URL`, `HTTP`, `accountID`.
- Use short receiver names, the same in all methods of a type: `func (c *Calculator)`.
- Start each doc comment of an exported name with that name. Write comments in Simplified Technical English.
- Comment the reason for a non-obvious line. Link to the ADR when a rule comes from one: `// ADR 0002: merge Fills into one Lot.`

## Tests

- Test external behaviour at the HTTP API seam (see the spec). Use `package httpapi_test`, a real SQLite file in `t.TempDir()`, a fake `price.Provider`, a fake `exchange.Provider` and the fixed clock.
- Build the xlsx fixtures in code with `xtbtest`. Never copy data from the files in `exports/`. They hold personal data.
- Use `require` when the test cannot continue after a failure. Use `assert` for the checks after that.
- Compare decimals with `assert.True(t, want.Equal(got), "want %s, got %s", want, got)`, or compare their `String()` values. `assert.Equal` on two `decimal.Decimal` values compares the internal form and can fail for equal numbers.
- Write table tests as a slice of named cases with `t.Run(tc.name, ...)`.
- Call `t.Helper()` in each helper. Call `t.Parallel()` in each test that has its own database.
- Use the ADR examples as test cases with their exact expected values.
