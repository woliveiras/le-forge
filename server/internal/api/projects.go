package api

import (
	"encoding/json"
	"errors"
	"github.com/woliveiras/le-forge/server/internal/config"
	"github.com/woliveiras/le-forge/server/internal/projectconfig"
	"io"
	"net/http"
)

func (s *Server) projectRoutes() {
	s.mux.Handle("PUT /api/v1/projects/{pid}/preferences", s.wrap(route{"PUT /api/v1/projects/{pid}/preferences", config.RoleAdmin, s.projectPreferences}))
	s.mux.Handle("GET /api/v1/projects", s.wrap(route{"GET /api/v1/projects", config.RoleViewer, s.listProjects}))
	s.mux.Handle("POST /api/v1/projects", s.wrap(route{"POST /api/v1/projects", config.RoleAdmin, s.createProject}))
}
func (s *Server) listProjects(w http.ResponseWriter, r *http.Request) error {
	ps, err := (projectconfig.Manager{Store: s.Store}).List(r.Context())
	if err != nil {
		return err
	}
	writeJSON(w, 200, ps)
	return nil
}
func (s *Server) createProject(w http.ResponseWriter, r *http.Request) error {
	var in projectconfig.Input
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		return badRequest("invalid project JSON")
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		return badRequest("expected one JSON object")
	}
	p, err := (projectconfig.Manager{Store: s.Store}).Create(r.Context(), in, identityFrom(r.Context()).Name)
	if errors.Is(err, projectconfig.ErrInvalid) {
		return badRequest("%s", err.Error())
	}
	if err != nil {
		return err
	}
	writeJSON(w, 201, p)
	return nil
}

func (s *Server) projectPreferences(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		ExecutionMode string `json:"executionMode"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		return badRequest("invalid preference JSON")
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		return badRequest("expected one JSON object")
	}
	if in.ExecutionMode == "local" && !s.Config.NativeProfile {
		return badRequest("local preference requires native Forge")
	}
	revision, err := ifMatchRevision(r)
	if err != nil {
		return err
	}
	if revision < 1 {
		return badRequest("If-Match is required")
	}
	p, err := (projectconfig.Manager{Store: s.Store}).SetExecutionPreference(r.Context(), r.PathValue("pid"), in.ExecutionMode, revision, identityFrom(r.Context()).Name)
	if errors.Is(err, projectconfig.ErrInvalid) {
		return badRequest("invalid execution preference")
	}
	if err != nil {
		return err
	}
	writeJSON(w, 200, p)
	return nil
}
