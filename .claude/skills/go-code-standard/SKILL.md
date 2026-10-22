---
name: go-code-standard
description: Go conventions for the profit-tracker backend. Use when you write, review, or refactor Go code, Go tests, SQL queries for sqlc, or the backend module layout.
---

# Go code standard

The backend is one Go module in `backend/`. Write idiomatic Go: the code that an experienced Go reviewer expects to see, with the standard library first.

## Completion criterion

Your Go work is done only when all three commands pass in `backend/` with zero issues:

1. `golangci-lint fmt` (gofumpt and goimports, from `.golangci.yml`)
2. `golangci-lint run ./...`
3. `go test -race ./...`

If you changed a `.sql` file, run `sqlc generate` before these commands. A hook formats each `.go` file after an edit, but the commands above are the gate.

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

## Layout

```
backend/
  cmd/profit-tracker/   main: config, wiring, start
  internal/<package>/   one package per domain concept
```

- Name packages after the domain concept, with the `GLOSSARY.md` term: `xtb`, `lot`, `ledger`, `importer`, `price`, `store`, `httpapi`.
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
- Add an interface only at a real seam. The one seam in v1 is `price.Provider`. A type with one implementation stays a concrete type.
- Accept interfaces and return concrete types.
- Pass `ctx context.Context` as the first parameter of every function that does I/O. Keep a context out of structs.
- Prefer a pure function for calculations. The Ledger calculator takes data and returns data.

## Naming and comments

- Write initialisms in one case: `ID`, `XTB`, `URL`, `HTTP`, `accountID`.
- Use short receiver names, the same in all methods of a type: `func (c *Calculator)`.
- Start each doc comment of an exported name with that name. Write comments in Simplified Technical English.
- Comment the reason for a non-obvious line. Link to the ADR when a rule comes from one: `// ADR 0002: merge Fills into one Lot.`

## Tests

- Test external behaviour at the HTTP API seam (see the spec). Use `package httpapi_test`, a real SQLite file in `t.TempDir()`, and a fake `price.Provider`.
- Use `require` when the test cannot continue after a failure. Use `assert` for the checks after that.
- Compare decimals with `assert.True(t, want.Equal(got), "want %s, got %s", want, got)`, or compare their `String()` values. `assert.Equal` on two `decimal.Decimal` values compares the internal form and can fail for equal numbers.
- Write table tests as a slice of named cases with `t.Run(tc.name, ...)`.
- Call `t.Helper()` in each helper. Call `t.Parallel()` in each test that has its own database.
- Use the ADR examples as test cases with their exact expected values.
