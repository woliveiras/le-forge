package api

import (
	"encoding/json"
	"errors"
	"github.com/woliveiras/le-forge/server/internal/taskauth"
	"io"
	"net/http"
	"strconv"
	"strings"
)

func (s *Server) taskAuthorization(w http.ResponseWriter, r *http.Request) error {
	m := taskauth.Manager{Store: s.Store}
	id := r.PathValue("tid")
	if r.Method == http.MethodGet {
		v, err := m.Review(r.Context(), id)
		if err != nil {
			return err
		}
		writeJSON(w, 200, v)
		return nil
	}
	var in struct {
		Mode    taskauth.Mode `json:"mode"`
		Scope   string        `json:"scope"`
		Confirm bool          `json:"confirm"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 2048)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if dec.Decode(&in) != nil {
		return badRequest("invalid authorization request")
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		return badRequest("expected one object")
	}
	rev, err := strconv.ParseInt(strings.Trim(r.Header.Get("If-Match"), `"`), 10, 64)
	if err != nil || rev < 0 {
		return badRequest("If-Match revision is required, including zero for initial consent")
	}
	rec, err := m.Save(r.Context(), id, in.Mode, in.Scope, rev, identityFrom(r.Context()).Name, in.Confirm)
	if err != nil {
		if errors.Is(err, taskauth.ErrInvalid) {
			return badRequest("%s", err)
		}
		return err
	}
	writeJSON(w, 200, rec)
	return nil
}
