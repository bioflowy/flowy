// Package server provenance handlers: list and detail endpoints for the audit trail.
package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/bioflowy/flowy/internal/api"
)

// handleListProvenance handles GET /projects/{slug}/provenance.
// Returns a summary list of provenance records for the project.
// Optional query parameters: ?entry=<name> and ?user=<username>.
func (s *Server) handleListProvenance(w http.ResponseWriter, r *http.Request) {
	proj := projectFromContext(r.Context())
	if proj == nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	entryFilter := r.URL.Query().Get("entry")
	userFilter := r.URL.Query().Get("user")

	items, err := s.store.ListProvenance(r.Context(), proj.ID, entryFilter, userFilter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("list provenance: %v", err))
		return
	}

	resp := make([]api.ProvenanceSummaryResponse, len(items))
	for i, item := range items {
		resp[i] = api.ProvenanceSummaryResponse{
			RunID:          item.RunID,
			RegistryRef:    item.RegistryRef,
			RegistryDigest: item.RegistryDigest,
			Username:       item.Username,
			RunState:       item.RunState,
			CreatedAt:      item.CreatedAt.Format(time.RFC3339),
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleGetProvenance handles GET /projects/{slug}/provenance/{run_id}.
// Returns the full provenance record including inputs and outputs.
func (s *Server) handleGetProvenance(w http.ResponseWriter, r *http.Request) {
	proj := projectFromContext(r.Context())
	if proj == nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	runID := chi.URLParam(r, "run_id")

	detail, err := s.store.GetProvenance(r.Context(), proj.ID, runID)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("provenance not found: %s", runID))
		return
	}

	resp := api.ProvenanceDetailResponse{
		RunID:             detail.RunID,
		RegistryRef:       detail.RegistryRef,
		RegistryDigest:    detail.RegistryDigest,
		Username:          detail.Username,
		RunState:          detail.RunState,
		RegistryEntryName: detail.RegistryEntryName,
		RegistryEntryKind: detail.RegistryEntryKind,
		CreatedAt:         detail.CreatedAt.Format(time.RFC3339),
		UpdatedAt:         detail.UpdatedAt.Format(time.RFC3339),
	}

	// Unmarshal inputs JSON from runs table.
	if detail.InputsJSON != "" {
		var inputs map[string]interface{}
		if err := json.Unmarshal([]byte(detail.InputsJSON), &inputs); err == nil {
			resp.Inputs = inputs
		}
	}

	// Unmarshal outputs JSON from runs table (may be empty if run not completed).
	if detail.OutputsJSON != "" {
		var outputs map[string]interface{}
		if err := json.Unmarshal([]byte(detail.OutputsJSON), &outputs); err == nil {
			resp.Outputs = outputs
		}
	}

	writeJSON(w, http.StatusOK, resp)
}
