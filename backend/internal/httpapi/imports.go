package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/shopspring/decimal"

	"github.com/jrojowski/profit-tracker/backend/internal/importer"
	"github.com/jrojowski/profit-tracker/backend/internal/store"
)

// maxUploadBytes is the largest export file that the API accepts. A full-history export is far smaller.
const maxUploadBytes = 32 << 20

// previewResponse is the body of POST /api/imports.
type previewResponse struct {
	ID                    int64            `json:"id"`
	FileName              string           `json:"fileName"`
	AccountNumber         string           `json:"accountNumber"`
	Account               *accountResponse `json:"account"`
	SuggestedCurrency     string           `json:"suggestedCurrency"`
	NewCashOperations     int              `json:"newCashOperations"`
	SkippedCashOperations int              `json:"skippedCashOperations"`
	// State is "preview", or "needs-review" if there are Import Conflicts.
	State string `json:"state"`
	// Conflicts is an empty array, not null, if there are no Import Conflicts.
	Conflicts []importConflictResponse `json:"conflicts"`
	// Warnings is an empty array, not null, if there are no warnings. A warning never stops the Import.
	Warnings []warningResponse `json:"warnings"`
}

// warningResponse is one warning of the Import preview.
// The time is RFC 3339 text in UTC. The amount is decimal text with the sign of the export.
type warningResponse struct {
	Kind  string `json:"kind"`
	XTBID string `json:"xtbId"`
	Time  string `json:"time"`
	// Direction is "in" for a Transfer In and "out" for a Transfer Out.
	Direction string `json:"direction"`
	Amount    string `json:"amount"`
}

// importConflictResponse is one Import Conflict.
type importConflictResponse struct {
	ID   int64  `json:"id"`
	Kind string `json:"kind"`
	// XTBID is the XTB ID of the Cash Operation. For the kind "check-mismatch", it is the sale.
	XTBID string `json:"xtbId"`
	// Existing is the stored data. It is null for the kind "check-mismatch".
	Existing *cashOperationDataResponse `json:"existing"`
	// New is the data of the file. For the kind "check-mismatch", it is the sale.
	New cashOperationDataResponse `json:"new"`
	// Check is the Realized Profit check. It is null for the kind "changed-data".
	Check *checkResponse `json:"check"`
	// Decision is null until the user decides.
	Decision *string `json:"decision"`
}

// checkResponse is the Realized Profit check of one sale. The values are decimal text.
type checkResponse struct {
	// Calculated is the Realized Profit from the Ledger. It is exact, with no rounding.
	Calculated string `json:"calculated"`
	// ClosedPositions is the Profit/Loss in the Closed Positions sheet.
	ClosedPositions string `json:"closedPositions"`
}

// cashOperationDataResponse is the data of a Cash Operation that an Import Conflict compares.
// The time is RFC 3339 text in UTC. The amount, the volume and the price are decimal text.
type cashOperationDataResponse struct {
	Type    string `json:"type"`
	Time    string `json:"time"`
	Amount  string `json:"amount"`
	Comment string `json:"comment"`
	// Ticker and PositionID are "" for an operation with none.
	Ticker     string `json:"ticker"`
	PositionID string `json:"positionId"`
	// Volume and Price are null for an operation with none.
	Volume *string `json:"volume"`
	Price  *string `json:"price"`
}

// decisionRequest is the body of POST /api/imports/{id}/conflicts/{conflictId}.
type decisionRequest struct {
	Decision string `json:"decision"`
}

// importResponse is one saved Import.
type importResponse struct {
	ID                    int64  `json:"id"`
	FileName              string `json:"fileName"`
	ImportedAt            string `json:"importedAt"`
	AccountID             int64  `json:"accountId"`
	AccountNumber         string `json:"accountNumber"`
	AccountName           string `json:"accountName"`
	NewCashOperations     int    `json:"newCashOperations"`
	SkippedCashOperations int    `json:"skippedCashOperations"`
}

// confirmRequest is the body of POST /api/imports/{id}/confirm.
type confirmRequest struct {
	// Account is the data of a new Account. A known account number ignores it.
	Account *struct {
		Name     string `json:"name"`
		Type     string `json:"type"`
		Currency string `json:"currency"`
	} `json:"account"`
}

// previewImport reads the uploaded export and returns the Import preview.
func (a *api) previewImport(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "send the export as the multipart form field \"file\"")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusBadRequest, "read the uploaded file: "+err.Error())
		return
	}

	preview, err := a.importer.Preview(r.Context(), header.Filename, data)
	if err != nil {
		a.writeImportError(w, r, err)
		return
	}

	resp := previewResponse{
		ID:                    preview.ID,
		FileName:              preview.FileName,
		AccountNumber:         preview.AccountNumber,
		SuggestedCurrency:     preview.SuggestedCurrency,
		NewCashOperations:     preview.NewCashOperations,
		SkippedCashOperations: preview.SkippedCashOperations,
		State:                 preview.State,
		Conflicts:             make([]importConflictResponse, 0, len(preview.Conflicts)),
		Warnings:              make([]warningResponse, 0, len(preview.Warnings)),
	}
	for _, c := range preview.Conflicts {
		resp.Conflicts = append(resp.Conflicts, toImportConflictResponse(c))
	}
	for _, w := range preview.Warnings {
		resp.Warnings = append(resp.Warnings, warningResponse{
			Kind:      w.Kind,
			XTBID:     w.XTBID,
			Time:      w.Time.UTC().Format(time.RFC3339),
			Direction: string(w.Direction),
			Amount:    formatAmount(w.Amount),
		})
	}
	if acc := preview.Account; acc != nil {
		accResp := toAccountResponse(*acc)
		resp.Account = &accResp
	}
	writeJSON(w, http.StatusOK, resp)
}

// confirmImport saves the Import in preview.
func (a *api) confirmImport(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id", "import")
	if !ok {
		return
	}

	var req confirmRequest
	// The body is optional. A known account number needs no body.
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "the body is not valid JSON: "+err.Error())
		return
	}
	var newAccount importer.NewAccount
	if req.Account != nil {
		newAccount = importer.NewAccount{Name: req.Account.Name, Type: req.Account.Type, Currency: req.Account.Currency}
	}

	saved, err := a.importer.Confirm(r.Context(), id, newAccount)
	if err != nil {
		a.writeImportError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toImportResponse(saved))
}

// decideConflict records the decision of the user for one Import Conflict.
func (a *api) decideConflict(w http.ResponseWriter, r *http.Request) {
	importID, ok := pathID(w, r, "id", "import")
	if !ok {
		return
	}
	conflictID, ok := pathID(w, r, "conflictId", "import conflict")
	if !ok {
		return
	}
	var req decisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "the body is not valid JSON: "+err.Error())
		return
	}

	conflict, err := a.importer.Decide(r.Context(), importID, conflictID, req.Decision)
	if err != nil {
		a.writeImportError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toImportConflictResponse(conflict))
}

// cancelImport cancels the Import. Nothing goes into the Ledger.
func (a *api) cancelImport(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id", "import")
	if !ok {
		return
	}
	if err := a.importer.Cancel(r.Context(), id); err != nil {
		a.writeImportError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// pathID returns the ID in the path value with the name. An ID that is not a number is an unknown object.
// It writes the error response and returns false if the ID is not valid.
func pathID(w http.ResponseWriter, r *http.Request, name, object string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("%s %q not found", object, r.PathValue(name)))
		return 0, false
	}
	return id, true
}

// listImports returns the saved Imports, the newest first.
func (a *api) listImports(w http.ResponseWriter, r *http.Request) {
	imports, err := a.importer.Imports(r.Context())
	if err != nil {
		a.writeInternalError(w, r, err)
		return
	}
	resp := make([]importResponse, 0, len(imports))
	for _, imp := range imports {
		resp = append(resp, toImportResponse(imp))
	}
	writeJSON(w, http.StatusOK, resp)
}

func toImportResponse(imp store.Import) importResponse {
	return importResponse{
		ID:                    imp.ID,
		FileName:              imp.FileName,
		ImportedAt:            imp.ImportedAt.UTC().Format(time.RFC3339),
		AccountID:             imp.AccountID,
		AccountNumber:         imp.AccountNumber,
		AccountName:           imp.AccountName,
		NewCashOperations:     imp.NewCashOperations,
		SkippedCashOperations: imp.SkippedCashOperations,
	}
}

func toImportConflictResponse(c store.ImportConflict) importConflictResponse {
	resp := importConflictResponse{
		ID:    c.ID,
		Kind:  c.Kind,
		XTBID: c.XTBID,
		New:   toCashOperationDataResponse(c.New),
	}
	if c.Existing != nil {
		existing := toCashOperationDataResponse(*c.Existing)
		resp.Existing = &existing
	}
	if c.Check != nil {
		resp.Check = &checkResponse{Calculated: formatAmount(c.Check.Calculated), ClosedPositions: formatAmount(c.Check.ClosedPositions)}
	}
	if c.Decision != "" {
		resp.Decision = &c.Decision
	}
	return resp
}

func toCashOperationDataResponse(d store.CashOperationData) cashOperationDataResponse {
	return cashOperationDataResponse{
		Type:       d.Type,
		Time:       d.Time.UTC().Format(time.RFC3339),
		Amount:     formatAmount(d.Amount),
		Comment:    d.Comment,
		Ticker:     d.Ticker,
		PositionID: d.PositionID,
		Volume:     nullDecimalText(d.Volume),
		Price:      nullDecimalText(d.Price),
	}
}

// nullDecimalText returns the decimal text of d, or nil if d is not valid.
func nullDecimalText(d decimal.NullDecimal) *string {
	if !d.Valid {
		return nil
	}
	text := d.Decimal.String()
	return &text
}

// writeImportError maps an error of the import service to a status.
func (a *api) writeImportError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, importer.ErrRejected), errors.Is(err, importer.ErrInvalidAccount), errors.Is(err, importer.ErrInvalidDecision):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, importer.ErrImportNotFound), errors.Is(err, importer.ErrConflictNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, importer.ErrImportClosed), errors.Is(err, importer.ErrOpenConflicts), errors.Is(err, importer.ErrLedgerChanged):
		writeError(w, http.StatusConflict, err.Error())
	default:
		a.writeInternalError(w, r, err)
	}
}

// writeInternalError logs err and sends a 500 error with no internal detail.
func (a *api) writeInternalError(w http.ResponseWriter, r *http.Request, err error) {
	a.logger.ErrorContext(r.Context(), "API request failed", "method", r.Method, "path", r.URL.Path, "error", err)
	writeError(w, http.StatusInternalServerError, "internal error, see the server log")
}

// writeError writes an error body with the status.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}
