package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

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
	}
	if acc := preview.Account; acc != nil {
		accResp := toAccountResponse(*acc)
		resp.Account = &accResp
	}
	writeJSON(w, http.StatusOK, resp)
}

// confirmImport saves the Import in preview.
func (a *api) confirmImport(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("import %q not found", r.PathValue("id")))
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

// writeImportError maps an error of the import service to a status.
func (a *api) writeImportError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, importer.ErrRejected), errors.Is(err, importer.ErrInvalidAccount):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, importer.ErrImportNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, importer.ErrAlreadyConfirmed):
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
