package httpapi_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHealth_ReturnsOKWithAppliedSchemaVersion(t *testing.T) {
	t.Parallel()
	srv := startServer(t)

	var got struct {
		Status        string `json:"status"`
		SchemaVersion int    `json:"schemaVersion"`
	}
	status := srv.getJSON(t, "/api/health", &got)

	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, "ok", got.Status)
	assert.Positive(t, got.SchemaVersion, "the migrations must run at start")
}
