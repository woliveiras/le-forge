package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/woliveiras/le-forge/server/internal/engine"
	"github.com/woliveiras/le-forge/server/internal/graph"
	"github.com/woliveiras/le-forge/server/internal/taskconfig"
)

func taskError(err error) error {
	if errors.Is(err, taskconfig.ErrInvalid) {
		return &engine.ValidationError{Report: &graph.Report{Errors: []graph.Issue{{Path: "task", Message: err.Error()}}}}
	}
	return err
}
func (s *Server) listTasks(w http.ResponseWriter, r *http.Request) error {
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return badRequest("invalid limit")
		}
		limit = n
	}
	page, err := (taskconfig.Manager{Store: s.Store}).List(r.Context(), r.URL.Query().Get("after"), limit)
	if err != nil {
		return taskError(err)
	}
	writeJSON(w, 200, page)
	return nil
}
func (s *Server) getTask(w http.ResponseWriter, r *http.Request) error {
	d, err := (taskconfig.Manager{Store: s.Store}).Get(r.Context(), r.PathValue("tid"))
	if err != nil {
		return err
	}
	w.Header().Set("ETag", strconv.Quote(strconv.FormatInt(d.Revision, 10)))
	writeJSON(w, 200, d)
	return nil
}
func (s *Server) saveTask(w http.ResponseWriter, r *http.Request) error {
	var in taskconfig.Input
	r.Body = http.MaxBytesReader(w, r.Body, 24*1024)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		return badRequest("invalid task JSON")
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return badRequest("expected one JSON object")
	}
	revision := int64(0)
	if r.Method == http.MethodPut {
		var err error
		revision, err = ifMatchRevision(r)
		if err != nil {
			return err
		}
		if revision < 1 {
			return badRequest("If-Match is required")
		}
	}
	d, err := (taskconfig.Manager{Store: s.Store}).Save(r.Context(), r.PathValue("tid"), in, revision, identityFrom(r.Context()).Name)
	if err != nil {
		return taskError(err)
	}
	status := 200
	if r.Method == http.MethodPost {
		status = 201
	}
	w.Header().Set("ETag", strconv.Quote(strconv.FormatInt(d.Revision, 10)))
	writeJSON(w, status, d)
	return nil
}
func (s *Server) reviewTask(w http.ResponseWriter, r *http.Request) error {
	d, err := (taskconfig.Manager{Store: s.Store}).Get(r.Context(), r.PathValue("tid"))
	if err != nil {
		return err
	}
	g, err := s.Store.GetGraph(r.Context(), d.GraphID, 0)
	if err != nil {
		return err
	}
	report := s.Engine.Validate(g)
	blockers := []string{"Task execution admission is not available yet. Saving or reviewing does not start agents."}
	if g.Metadata.Version != d.GraphVersion {
		blockers = append(blockers, "The workflow changed. Review the current version and save the draft again.")
	}
	n := g.Node(d.NodeID)
	if n == nil || n.Disabled {
		blockers = append(blockers, "The selected agent is missing or disabled.")
	}
	// Validation includes current runtime/provider/environment checks; this response
	// is a preview, never permission to start or proof of future availability.
	writeJSON(w, 200, map[string]any{"task": d, "currentGraphVersion": g.Metadata.Version, "validation": report, "blockers": blockers, "canStart": false})
	return nil
}
