package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/woliveiras/le-forge/server/internal/domain"
	"github.com/woliveiras/le-forge/server/internal/engine"
	"github.com/woliveiras/le-forge/server/internal/environment"
)

// Project environments (spec.environment): state of the build for the
// latest saved graph version, explicit builds and resets. The definition
// itself is part of the graph document and saved with PUT /graphs/{id}.

func (s *Server) getEnvironment(w http.ResponseWriter, r *http.Request) error {
	if s.Environments == nil {
		return engine.ErrNotFound
	}
	g, _, err := s.Engine.GetGraph(r.Context(), r.PathValue("id"), 0)
	if err != nil {
		return err
	}
	v, err := s.Environments.Get(r.Context(), g)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, v)
	return nil
}

type buildRequest struct {
	// Force rebuilds even when the environment is up to date.
	Force bool `json:"force"`
}

func (s *Server) buildEnvironment(w http.ResponseWriter, r *http.Request) error {
	if s.Environments == nil {
		return engine.ErrNotFound
	}
	var req buildRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		return badRequest("invalid JSON: %v", err)
	}
	g, _, err := s.Engine.GetGraph(r.Context(), r.PathValue("id"), 0)
	if err != nil {
		return err
	}
	v, err := s.Environments.Build(r.Context(), g, req.Force, identityFrom(r.Context()).Name)
	if err != nil {
		return err
	}
	// 202 while a build runs in the background; 200 when the environment
	// was already up to date and nothing was started.
	code := http.StatusOK
	if v.State == environment.StateBuilding {
		code = http.StatusAccepted
	}
	writeJSON(w, code, v)
	return nil
}

func (s *Server) resetEnvironment(w http.ResponseWriter, r *http.Request) error {
	if s.Environments == nil {
		return engine.ErrNotFound
	}
	id := r.PathValue("id")
	g, rec, err := s.Engine.GetGraph(r.Context(), id, 0)
	if err != nil {
		return err
	}
	// Running agents use the image and cache volumes being removed.
	if rec.State != domain.GraphStopped && rec.State != domain.GraphFailed {
		return fmt.Errorf("%w: graph %s is %s; stop it before resetting its environment", engine.ErrConflict, id, rec.State)
	}
	if err := s.Environments.Reset(r.Context(), id, identityFrom(r.Context()).Name); err != nil {
		return err
	}
	v, err := s.Environments.Get(r.Context(), g)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, v)
	return nil
}
