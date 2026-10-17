package api

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/woliveiras/le-forge/server/internal/mcpconfig"
	"github.com/woliveiras/le-forge/server/internal/storage"
	"github.com/woliveiras/le-forge/server/internal/tools"
)

func (s *Server) mcpList(w http.ResponseWriter, r *http.Request) error {
	if s.MCP == nil {
		return storage.ErrNotFound
	}
	v, e := s.MCP.List(r.Context())
	if e != nil {
		return e
	}
	writeJSON(w, 200, v)
	return nil
}
func (s *Server) mcpGet(w http.ResponseWriter, r *http.Request) error {
	if s.MCP == nil {
		return storage.ErrNotFound
	}
	v, e := s.MCP.Get(r.Context(), r.PathValue("connection"))
	if e != nil {
		return e
	}
	writeJSON(w, 200, v)
	return nil
}
func (s *Server) mcpPut(w http.ResponseWriter, r *http.Request) error {
	if s.MCP == nil {
		return storage.ErrNotFound
	}
	var in mcpconfig.Input
	d := json.NewDecoder(io.LimitReader(r.Body, 32769))
	d.DisallowUnknownFields()
	if d.Decode(&in) != nil {
		return badRequest("invalid MCP connection body")
	}
	if d.Decode(new(any)) != io.EOF {
		return badRequest("invalid MCP connection body")
	}
	rev := int64(0)
	status := http.StatusCreated
	if r.Method == http.MethodPut {
		var e error
		rev, e = ifMatchRevision(r)
		if e != nil {
			return e
		}
		if rev == 0 {
			return badRequest("If-Match revision required")
		}
		if in.ID != r.PathValue("connection") {
			return badRequest("connection id cannot change")
		}
		status = 200
	}
	v, e := s.MCP.Put(r.Context(), in, rev, identityFrom(r.Context()).Name)
	if e != nil {
		return e
	}
	writeJSON(w, status, v)
	return nil
}
func (s *Server) mcpRefresh(w http.ResponseWriter, r *http.Request) error {
	if s.MCP == nil {
		return storage.ErrNotFound
	}
	v, e := s.MCP.Refresh(r.Context(), r.PathValue("connection"), identityFrom(r.Context()).Name)
	if e != nil {
		return e
	}
	writeJSON(w, 200, v)
	return nil
}

func (s *Server) mcpTools(w http.ResponseWriter, r *http.Request) error {
	if s.MCP == nil {
		return storage.ErrNotFound
	}
	views, e := s.MCP.List(r.Context())
	if e != nil {
		return e
	}
	out := []tools.ToolInfo{}
	for _, v := range views {
		if !v.Enabled || v.Status != "ready" {
			continue
		}
		for _, t := range v.Tools {
			out = append(out, tools.ToolInfo{Name: "mcp." + v.ID + "." + t.Name, Capability: "MCP remote tool", Description: t.Description, Schema: t.InputSchema})
		}
	}
	writeJSON(w, 200, out)
	return nil
}
