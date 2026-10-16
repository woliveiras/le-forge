package api

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/woliveiras/le-forge/server/internal/tools"
	"github.com/woliveiras/le-forge/server/internal/workspace"
)

func (s *Server) inspectGit(w http.ResponseWriter, r *http.Request) error {
	root, err := s.browseRoot(r)
	if err != nil {
		return err
	}
	view, path := r.PathValue("view"), r.URL.Query().Get("path")
	if _, err := workspace.InspectionArgs(view, path); err != nil {
		return badRequest("%v", err)
	}
	if _, _, err := browsePath(root, path); err != nil {
		return badRequest("invalid workspace path")
	}
	g, _, err := s.Engine.GetGraph(r.Context(), r.PathValue("id"), 0)
	if err != nil {
		return err
	}
	base, err := s.Workspaces.GraphDir(g)
	if err != nil {
		return err
	}
	result, err := s.Workspaces.Inspect(r.Context(), root, base, view, path)
	if err != nil {
		return err
	}
	result.GraphID = g.Metadata.ID
	result.NodeID = r.URL.Query().Get("node")
	if result.NodeID != "" {
		rep, _ := strconv.Atoi(r.URL.Query().Get("replica"))
		result.Replica = &rep
	}
	writeJSON(w, 200, result)
	return nil
}

// browsePath retains the existing absolute-within-workspace contract while
// keeping repository internals private, including aliases through symlinks.
func browsePath(root, path string) (string, string, error) {
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if part == ".." || strings.EqualFold(part, ".git") {
			return "", "", fmt.Errorf("invalid workspace path")
		}
	}
	abs, rel, err := tools.ResolvePath(root, path)
	if err != nil {
		return "", "", err
	}
	for _, part := range strings.Split(rel, "/") {
		if strings.EqualFold(part, ".git") {
			return "", "", fmt.Errorf("repository internals are private")
		}
	}
	return abs, rel, nil
}
