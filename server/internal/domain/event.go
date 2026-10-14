package domain

import (
	"encoding/json"
	"time"
)

// Event is an immutable fact appended to the event log. The log is the
// audit trail, the source for the UI's live stream, and what the platform
// replays to explain (not to rebuild) what happened after a restart. The
// shape follows CloudEvents attribute naming so events can be bridged to
// any CloudEvents consumer without translation.
type Event struct {
	// Seq is a store-assigned, strictly increasing position (per store).
	Seq         int64           `json:"seq"`
	ID          string          `json:"id"`
	Type        EventType       `json:"type"`
	Time        time.Time       `json:"time"`
	GraphID     string          `json:"graphId,omitempty"`
	NodeID      string          `json:"nodeId,omitempty"`
	Replica     int             `json:"replica"`
	ExecutionID string          `json:"executionId,omitempty"`
	MessageID   string          `json:"messageId,omitempty"`
	Actor       string          `json:"actor,omitempty"` // user, instance or "system"
	TraceID     string          `json:"traceId,omitempty"`
	Level       string          `json:"level,omitempty"` // for log events
	Data        json.RawMessage `json:"data,omitempty"`
}

type EventType string

const (
	EvGraphCreated   EventType = "graph.created"
	EvGraphUpdated   EventType = "graph.updated"
	EvGraphDeleted   EventType = "graph.deleted"
	EvGraphStarted   EventType = "graph.started"
	EvGraphStopping  EventType = "graph.stopping"
	EvGraphStopped   EventType = "graph.stopped"
	EvGraphPaused    EventType = "graph.paused"
	EvGraphResumed   EventType = "graph.resumed"
	EvGraphFailed    EventType = "graph.failed"
	EvNodeScaled     EventType = "node.scaled"
	EvReplicaState   EventType = "replica.state_changed"
	EvExecStarted    EventType = "execution.started"
	EvExecCompleted  EventType = "execution.completed"
	EvExecFailed     EventType = "execution.failed"
	EvMsgPublished   EventType = "message.published"
	EvMsgDelivered   EventType = "message.delivered"
	EvMsgAcked       EventType = "message.acked"
	EvMsgNacked      EventType = "message.nacked"
	EvMsgDeadLetter  EventType = "message.dead_lettered"
	EvMsgDropped     EventType = "message.dropped"
	EvMsgHeld        EventType = "message.held"
	EvMsgInjected    EventType = "message.injected"
	EvToolInvoked    EventType = "tool.invoked"
	EvToolCompleted  EventType = "tool.completed"
	EvToolDenied     EventType = "tool.denied"
	EvApprovalReq    EventType = "approval.requested"
	EvApprovalDone   EventType = "approval.decided"
	EvOperatorMsg    EventType = "operator.message"
	EvAgentOutput    EventType = "agent.output" // streamed narration / LLM text
	EvAgentBroadcast EventType = "agent.broadcast"
	EvLog            EventType = "log"
	// Installation configuration (no graph id). Data never carries secrets.
	EvProviderChanged EventType = "provider.changed"
	EvProviderTested  EventType = "provider.tested"

	EvEnvBuildStarted EventType = "environment.build_started"
	EvEnvReady        EventType = "environment.ready"
	EvEnvFailed       EventType = "environment.failed"
	EvEnvReset        EventType = "environment.reset"
)

// NewEvent builds an event with a fresh id and timestamp; data is marshalled.
func NewEvent(t EventType, graph string, data any) *Event {
	ev := &Event{ID: NewID("evt"), Type: t, Time: time.Now().UTC(), GraphID: graph, Actor: "system"}
	if data != nil {
		if b, err := json.Marshal(data); err == nil {
			ev.Data = b
		}
	}
	return ev
}

func (e *Event) WithNode(node string, replica int) *Event {
	e.NodeID, e.Replica = node, replica
	return e
}

func (e *Event) WithExecution(id string) *Event { e.ExecutionID = id; return e }
func (e *Event) WithMessage(id string) *Event   { e.MessageID = id; return e }
func (e *Event) WithActor(a string) *Event      { e.Actor = a; return e }
func (e *Event) WithTrace(id string) *Event     { e.TraceID = id; return e }
