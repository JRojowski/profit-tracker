# 02: Import an export into a new Account

**What to build:** The user uploads an XTB xlsx export in the web interface. The tool shows a preview with the account number and the number of new Cash Operations. For a new account number, the user enters a name, a type (regular, IKE or IKZE) and a currency, and the tool suggests the currency from the file name prefix. The user confirms, and the tool saves the Account and the Cash Operations. The Imports page lists past Imports. This ticket supports the types `Deposit` and `Stock purchase`, with one Fill per Lot. Use the glossary terms from `GLOSSARY.md`.

**Blocked by:** 01 (Walking skeleton).

**Status:** done

- [x] `POST /api/imports` reads the file and returns a preview. Nothing is saved before the confirmation.
- [x] The preview of a new account number contains the suggested currency from the file name prefix (for example `USD_`).
- [x] `POST /api/imports/{id}/confirm` saves the Account, the Instruments, the Cash Operations and the Lots in one transaction.
- [x] A later Import with the same account number goes to the same Account with no questions.
- [x] Money and volume values use exact decimal arithmetic and are stored as decimal text.
- [x] The parser reads purchase Comments with and without the ticker (`OPEN BUY 2 @ 58.29` and `OPEN BUY PENG.US 2 @ 58.29`).
- [x] An unknown Cash Operation type stops the Import with an error that names the type and the row.
- [x] A file that is not an XTB export stops the Import with a clear error.
- [x] The Import stops with an error when the sum of the Cash Operation amounts is not equal to the Total row.
- [x] `GET /api/imports` and the Imports page list the Imports with the date, the file name and the counts.
- [x] API tests cover: a new Account, a known Account, an unknown type, an invalid file, a Total row mismatch.
