package api

import (
	"errors"
	"github.com/woliveiras/le-forge/server/internal/domain"
	"github.com/woliveiras/le-forge/server/internal/executioncontrol"
	"net/http"
)

func (s *Server) executionTree(w http.ResponseWriter, r *http.Request) error {
	tree, err := (executioncontrol.Manager{Store: s.Store}).Tree(r.Context(), r.PathValue("eid"))
	if err != nil {
		return err
	}
	writeJSON(w, 200, tree)
	return nil
}
func (s *Server) cancelExecution(w http.ResponseWriter, r *http.Request) error {
	var input struct {
		Scope   string `json:"scope"`
		Confirm bool   `json:"confirm"`
	}
	if err := delegationDecode(w, r, &input); err != nil {
		return err
	}
	if !input.Confirm {
		return badRequest("explicit confirmation is required")
	}
	actor := identityFrom(r.Context()).Name
	result, err := (executioncontrol.Manager{Store: s.Store}).Cancel(r.Context(), r.PathValue("eid"), input.Scope, actor)
	if errors.Is(err, executioncontrol.ErrInvalid) {
		return badRequest("scope must be tree, or child for a delegated execution")
	}
	if err != nil {
		return err
	}
	if result.Created {
		x, err := s.Store.GetExecution(r.Context(), result.Request.ExecutionID)
		if err != nil {
			return err
		}
		s.Bus.Publish(r.Context(), domain.NewEvent("execution.cancel_requested", x.GraphID, result.Request).WithExecution(x.ID).WithNode(x.NodeID, x.Replica).WithActor(actor))
	}
	status := http.StatusAccepted
	if result.AlreadyTerminal {
		status = http.StatusOK
	}
	writeJSON(w, status, result)
	return nil
}
