// Package api is the HTTP surface of the control plane: the public REST API
// (/api/v1), the live event stream (SSE), the runtime callback API used by
// external agents (/internal/v1), health and metrics, and the web console.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"gopkg.in/yaml.v3"

	"github.com/woliveiras/le-forge/server/internal/config"
	"github.com/woliveiras/le-forge/server/internal/domain"
	"github.com/woliveiras/le-forge/server/internal/engine"
	"github.com/woliveiras/le-forge/server/internal/environment"
	"github.com/woliveiras/le-forge/server/internal/events"
	"github.com/woliveiras/le-forge/server/internal/graph"
	"github.com/woliveiras/le-forge/server/internal/hitl"
	"github.com/woliveiras/le-forge/server/internal/llm"
	"github.com/woliveiras/le-forge/server/internal/llmconfig"
	"github.com/woliveiras/le-forge/server/internal/observability"
	"github.com/woliveiras/le-forge/server/internal/queue"
	"github.com/woliveiras/le-forge/server/internal/secrets"
	"github.com/woliveiras/le-forge/server/internal/storage"
	"github.com/woliveiras/le-forge/server/internal/tools"
	"github.com/woliveiras/le-forge/server/internal/workspace"
)

// Version is set at build time (-ldflags "-X .../api.Version=...").
var Version = "dev"

type Deps struct {
	Config     *config.Config
	Engine     *engine.Engine
	Store      storage.Store
	Broker     queue.Broker
	Bus        *events.Bus
	Approvals  *hitl.Service
	Models     *llm.Registry
	Providers  *llmconfig.Manager
	Tools      *tools.Registry
	Workspaces *workspace.Manager
	// Environments is optional; without it the environment routes answer 404.
	Environments *environment.Manager
	Auth         *Authenticator
	Log          *slog.Logger
}

type Server struct {
	Deps
	mux      *http.ServeMux
	internal *http.ServeMux // callback API; == mux unless server.internalAddr is set
}

func New(d Deps) *Server {
	s := &Server{Deps: d, mux: http.NewServeMux()}
	s.internal = s.mux
	if d.Config.Server.InternalAddr != "" {
		s.internal = http.NewServeMux()
	}
	s.routes()
	return s
}

// Handler serves the public API, console, health and metrics (and the
// callback API too, unless it has a listener of its own).
func (s *Server) Handler() http.Handler { return s.middleware(s.mux) }

// InternalHandler serves the callback API when server.internalAddr is set.
func (s *Server) InternalHandler() http.Handler {
	if s.internal == s.mux {
		return nil
	}
	return s.middleware(s.internal)
}

type route struct {
	pattern string
	role    config.Role // "" = public
	h       func(http.ResponseWriter, *http.Request) error
}

func (s *Server) routes() {
	R := config.RoleViewer
	O := config.RoleOperator
	A := config.RoleAdmin
	routes := []route{
		{"GET /healthz", "", s.healthz},
		{"GET /readyz", "", s.readyz},
		{"GET /api/v1/info", R, s.info},
		{"GET /api/v1/graphs", R, s.listGraphs},
		{"POST /api/v1/graphs", A, s.createGraph},
		{"POST /api/v1/graphs/validate", R, s.validateGraph},
		{"GET /api/v1/graphs/{id}", R, s.getGraph},
		{"PUT /api/v1/graphs/{id}", A, s.updateGraph},
		{"DELETE /api/v1/graphs/{id}", A, s.deleteGraph},
		{"GET /api/v1/graphs/{id}/versions", R, s.graphVersions},
		{"GET /api/v1/graphs/{id}/export", R, s.exportGraph},
		{"POST /api/v1/graphs/{id}/{action}", O, s.graphAction},
		{"GET /api/v1/graphs/{id}/status", R, s.graphStatus},
		{"GET /api/v1/graphs/{id}/environment", R, s.getEnvironment},
		{"POST /api/v1/graphs/{id}/environment/build", O, s.buildEnvironment},
		{"POST /api/v1/graphs/{id}/environment/reset", A, s.resetEnvironment},
		{"POST /api/v1/graphs/{id}/nodes/{node}/{action}", O, s.nodeAction},
		{"PUT /api/v1/graphs/{id}/nodes/{node}/scale", O, s.scaleNode},
		{"GET /api/v1/graphs/{id}/nodes/{node}/chat", R, s.listChat},
		{"POST /api/v1/graphs/{id}/nodes/{node}/chat", O, s.postChat},
		{"POST /api/v1/graphs/{id}/channels/{channel}/messages", O, s.inject},
		{"GET /api/v1/graphs/{id}/messages", R, s.listMessages},
		{"GET /api/v1/messages/{mid}", R, s.getMessage},
		{"GET /api/v1/graphs/{id}/executions", R, s.listExecutions},
		{"GET /api/v1/executions/{eid}", R, s.getExecution},
		{"GET /api/v1/graphs/{id}/events", R, s.listEvents},
		{"GET /api/v1/events/stream", R, s.stream},
		{"GET /api/v1/approvals", R, s.listApprovals},
		{"POST /api/v1/approvals/{aid}/{decision}", O, s.decide},
		{"GET /api/v1/providers", R, s.providers},
		{"POST /api/v1/providers", A, s.createProvider},
		{"POST /api/v1/providers/test", A, s.testProviderDraft},
		{"GET /api/v1/providers/{name}", R, s.getProvider},
		{"PUT /api/v1/providers/{name}", A, s.updateProvider},
		{"DELETE /api/v1/providers/{name}", A, s.deleteProvider},
		{"POST /api/v1/providers/{name}/test", O, s.testProvider},
		{"GET /api/v1/providers/{name}/models", R, s.models},
		{"GET /api/v1/provider-settings", R, s.providerSettings},
		{"PUT /api/v1/provider-settings", A, s.updateProviderSettings},
		{"GET /api/v1/tools", R, s.toolsList},
		{"GET /api/v1/graphs/{id}/files", R, s.listFiles},
		{"GET /api/v1/graphs/{id}/files/content", R, s.fileContent},
	}
	for _, rt := range routes {
		s.mux.Handle(rt.pattern, s.wrap(rt))
	}
	if s.Config.Server.ProtectMetrics {
		metrics := promhttp.Handler()
		s.mux.Handle("GET /metrics", s.wrap(route{"GET /metrics", config.RoleViewer, func(w http.ResponseWriter, r *http.Request) error {
			metrics.ServeHTTP(w, r)
			return nil
		}}))
	} else {
		s.mux.Handle("GET /metrics", promhttp.Handler())
	}
	s.internalRoutes()
	if dir := s.Config.Server.WebDir; dir != "" {
		s.mux.Handle("GET /", spa(dir))
	}
}

// wrap applies authentication, authorisation and error mapping.
func (s *Server) wrap(rt route) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if rt.role != "" {
			id, err := s.Auth.Authenticate(r)
			if err != nil {
				writeErr(w, http.StatusUnauthorized, "unauthorized", err.Error(), nil)
				return
			}
			if !id.Role.Allows(rt.role) {
				writeErr(w, http.StatusForbidden, "forbidden", fmt.Sprintf("role %s required", rt.role), nil)
				return
			}
			r = r.WithContext(context.WithValue(r.Context(), ctxKey{}, id))
		}
		if err := rt.h(w, r); err != nil {
			s.writeError(w, r, err)
		}
	})
}

func (s *Server) writeError(w http.ResponseWriter, r *http.Request, err error) {
	var ve *engine.ValidationError
	var pve *llmconfig.ValidationError
	switch {
	case errors.As(err, &ve):
		writeErr(w, http.StatusUnprocessableEntity, "invalid", err.Error(), ve.Report)
	case errors.As(err, &pve):
		writeErr(w, http.StatusUnprocessableEntity, "invalid", err.Error(), map[string]any{"valid": false, "errors": pve.Errors})
	case errors.Is(err, secrets.ErrNoKey):
		writeErr(w, http.StatusConflict, "secret_key_missing", err.Error(), nil)
	case errors.Is(err, engine.ErrNotFound), errors.Is(err, storage.ErrNotFound):
		writeErr(w, http.StatusNotFound, "not_found", err.Error(), nil)
	case errors.Is(err, engine.ErrConflict), errors.Is(err, hitl.ErrNotPending), errors.Is(err, storage.ErrConflict):
		writeErr(w, http.StatusConflict, "conflict", err.Error(), nil)
	case errors.Is(err, engine.ErrInvalid), errors.Is(err, errBadRequest):
		writeErr(w, http.StatusBadRequest, "invalid", err.Error(), nil)
	case errors.Is(err, tools.ErrDenied), errors.Is(err, tools.ErrRejected):
		writeErr(w, http.StatusForbidden, "forbidden", err.Error(), nil)
	case errors.Is(err, environment.ErrBuildRunning):
		writeErr(w, http.StatusConflict, "conflict", err.Error(), nil)
	case errors.Is(err, environment.ErrNotDefined):
		writeErr(w, http.StatusBadRequest, "invalid", err.Error(), nil)
	case errors.Is(err, tools.ErrUnknownTool):
		writeErr(w, http.StatusNotFound, "not_found", err.Error(), nil)
	case errors.Is(err, tools.ErrRateLimited):
		writeErr(w, http.StatusTooManyRequests, "rate_limited", err.Error(), nil)
	case errors.Is(err, context.Canceled):
		// client went away
	default:
		s.Log.Error("request failed", "method", r.Method, "path", r.URL.Path, "err", err)
		writeErr(w, http.StatusInternalServerError, "internal", "internal error", nil)
	}
}

var errBadRequest = errors.New("bad request")

func badRequest(format string, a ...any) error {
	return fmt.Errorf("%w: %s", errBadRequest, fmt.Sprintf(format, a...))
}

func writeErr(w http.ResponseWriter, code int, kind, msg string, details any) {
	body := map[string]any{"error": msg, "code": kind}
	if details != nil {
		body["details"] = details
	}
	writeJSON(w, code, body)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

// middleware: panic recovery, tracing, metrics, security headers, CORS,
// body limits, access logging.
func (s *Server) middleware(next http.Handler) http.Handler {
	prop := propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &statusWriter{ResponseWriter: w, code: 200}
		defer func() {
			if p := recover(); p != nil {
				s.Log.Error("panic", "panic", p, "stack", string(debug.Stack()))
				writeErr(rw, http.StatusInternalServerError, "internal", "internal error", nil)
			}
		}()
		h := rw.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		if origin := r.Header.Get("Origin"); origin != "" && s.allowedOrigin(origin) {
			h.Set("Access-Control-Allow-Origin", origin)
			h.Set("Vary", "Origin")
			h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, If-Match")
			h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			if r.Method == http.MethodOptions {
				rw.WriteHeader(http.StatusNoContent)
				return
			}
		}
		if r.Body != nil {
			r.Body = http.MaxBytesReader(rw, r.Body, s.Config.Server.MaxBodyBytes)
		}
		ctx := prop.Extract(r.Context(), propagation.HeaderCarrier(r.Header))
		route := r.Method + " " + r.URL.Path
		if _, pat := s.mux.Handler(r); pat != "" {
			route = pat
		}
		var span trace.Span
		if !strings.HasPrefix(r.URL.Path, "/api/v1/events/stream") && !strings.HasPrefix(r.URL.Path, "/internal/") && r.URL.Path != "/metrics" && r.URL.Path != "/healthz" {
			ctx, span = observability.Start(ctx, route, attribute.String("http.method", r.Method), attribute.String("http.route", route))
			defer span.End()
		}
		next.ServeHTTP(rw, r.WithContext(ctx))
		if span != nil {
			span.SetAttributes(attribute.Int("http.status_code", rw.code))
		}
		if strings.HasPrefix(route, "GET /api/v1/events/stream") {
			return
		}
		d := time.Since(start)
		observability.APIRequests.WithLabelValues(r.Method, route, strconv.Itoa(rw.code)).Inc()
		observability.APIDuration.WithLabelValues(r.Method, route).Observe(d.Seconds())
		if r.URL.Path != "/healthz" && r.URL.Path != "/metrics" && (strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/internal/")) {
			s.Log.Debug("http", "method", r.Method, "path", r.URL.Path, "status", rw.code, "ms", d.Milliseconds())
		}
	})
}

func (s *Server) allowedOrigin(o string) bool {
	for _, a := range s.Config.Server.CORSOrigins {
		if a == "*" || a == o {
			return true
		}
	}
	return false
}

type statusWriter struct {
	http.ResponseWriter
	code    int
	written bool
}

func (w *statusWriter) WriteHeader(c int) {
	if !w.written {
		w.code = c
		w.written = true
	}
	w.ResponseWriter.WriteHeader(c)
}

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// --- health ---------------------------------------------------------------

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) error {
	writeJSON(w, 200, map[string]string{"status": "ok"})
	return nil
}

func (s *Server) readyz(w http.ResponseWriter, r *http.Request) error {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	checks := map[string]string{"storage": "ok", "queue": "ok", "sandbox": "not configured"}
	code := 200
	if err := s.Store.Ping(ctx); err != nil {
		checks["storage"], code = err.Error(), 503
	}
	if err := s.Broker.Ping(ctx); err != nil {
		checks["queue"], code = err.Error(), 503
	}
	// The sandbox daemon only matters to docker-isolated agents, so it is
	// reported but does not make the platform unready.
	if d := s.Config.Isolation.Docker; d.Host != "" {
		if _, err := tools.PingDocker(ctx, d.Bin, d.Host); err != nil {
			checks["sandbox"] = err.Error()
		} else {
			checks["sandbox"] = "ok"
		}
	}
	writeJSON(w, code, checks)
	return nil
}

func (s *Server) info(w http.ResponseWriter, r *http.Request) error {
	rts := []string{}
	for k := range s.Engine.KnownRuntimes() {
		rts = append(rts, string(k))
	}
	writeJSON(w, 200, map[string]any{
		"version": Version, "instance": s.Config.Instance,
		"queue": s.Broker.Name(), "storage": s.Store.Name(),
		"providers": s.Models.Names(), "defaultProvider": s.Models.DefaultName(),
		"runtimes": rts, "grafanaUrl": s.Config.Observability.GrafanaURL,
		"identity": identityFrom(r.Context()),
	})
	return nil
}

// --- graphs ---------------------------------------------------------------

type graphSummary struct {
	Graph  *domain.Graph       `json:"graph"`
	Record *domain.GraphRecord `json:"record"`
}

func (s *Server) listGraphs(w http.ResponseWriter, r *http.Request) error {
	gs, err := s.Store.ListGraphs(r.Context())
	if err != nil {
		return err
	}
	out := []graphSummary{}
	for _, g := range gs {
		rec, err := s.Store.GetGraphRecord(r.Context(), g.Metadata.ID)
		if err != nil {
			rec = &domain.GraphRecord{ID: g.Metadata.ID, State: domain.GraphStopped, DesiredState: domain.GraphStopped}
		}
		out = append(out, graphSummary{g, rec})
	}
	writeJSON(w, 200, out)
	return nil
}

// decodeGraph accepts JSON or YAML (by Content-Type, or sniffed). Keys the
// graph schema does not define are returned as issues rather than dropped.
func decodeGraph(r *http.Request) (*domain.Graph, []graph.Issue, error) {
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, nil, badRequest("read body: %v", err)
	}
	ct := r.Header.Get("Content-Type")
	trim := strings.TrimSpace(string(b))
	isYAML := strings.Contains(ct, "yaml") || (!strings.HasPrefix(trim, "{") && trim != "")
	g, unknown, err := graph.Decode(b, isYAML)
	if err != nil {
		if isYAML {
			return nil, nil, badRequest("invalid YAML: %v", err)
		}
		return nil, nil, badRequest("invalid JSON: %v", err)
	}
	return g, unknown, nil
}

// unknownFieldsError reports undefined keys like any other validation
// finding (422 with field paths), so editors can point at them.
func unknownFieldsError(unknown []graph.Issue) error {
	return &engine.ValidationError{Report: &graph.Report{Valid: false, Errors: unknown, Warnings: []graph.Issue{}}}
}

func (s *Server) createGraph(w http.ResponseWriter, r *http.Request) error {
	g, unknown, err := decodeGraph(r)
	if err != nil {
		return err
	}
	if len(unknown) > 0 {
		return unknownFieldsError(unknown)
	}
	// Derive the id the way normalisation does, without touching the
	// document (it is stored as written).
	id := g.Metadata.ID
	if id == "" {
		id = graph.Slug(g.Metadata.Name)
	}
	if _, err := s.Store.GetGraph(r.Context(), id, 0); err == nil {
		return fmt.Errorf("%w: graph %q already exists (use PUT to update)", engine.ErrConflict, id)
	}
	g, rec, err := s.Engine.SaveGraph(r.Context(), g, 0, identityFrom(r.Context()).Name)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, graphSummary{g, rec})
	return nil
}

func (s *Server) validateGraph(w http.ResponseWriter, r *http.Request) error {
	g, unknown, err := decodeGraph(r)
	if err != nil {
		return err
	}
	rep := s.Engine.Validate(g)
	if len(unknown) > 0 {
		rep.Errors = append(unknown, rep.Errors...)
		rep.Valid = false
	}
	writeJSON(w, 200, rep)
	return nil
}

func (s *Server) getGraph(w http.ResponseWriter, r *http.Request) error {
	v, _ := strconv.Atoi(r.URL.Query().Get("version"))
	// The document as written: editors must not save defaults back into nodes.
	g, rec, err := s.Engine.GetGraphDocument(r.Context(), r.PathValue("id"), v)
	if err != nil {
		return err
	}
	writeJSON(w, 200, graphSummary{g, rec})
	return nil
}

func (s *Server) updateGraph(w http.ResponseWriter, r *http.Request) error {
	g, unknown, err := decodeGraph(r)
	if err != nil {
		return err
	}
	if len(unknown) > 0 {
		return unknownFieldsError(unknown)
	}
	id := r.PathValue("id")
	if g.Metadata.ID == "" {
		g.Metadata.ID = id
	}
	if g.Metadata.ID != id {
		return badRequest("metadata.id %q does not match the URL", g.Metadata.ID)
	}
	if _, err := s.Store.GetGraph(r.Context(), id, 0); err != nil {
		return engine.ErrNotFound
	}
	expected := 0
	if im := strings.Trim(r.Header.Get("If-Match"), `" W/`); im != "" {
		expected, err = strconv.Atoi(im)
		if err != nil {
			return badRequest("If-Match must be a version number")
		}
	}
	g, rec, err := s.Engine.SaveGraph(r.Context(), g, expected, identityFrom(r.Context()).Name)
	if err != nil {
		return err
	}
	writeJSON(w, 200, graphSummary{g, rec})
	return nil
}

func (s *Server) deleteGraph(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	actor := identityFrom(r.Context()).Name
	hadEnv := false
	if s.Environments != nil {
		hadEnv, _ = s.Environments.Exists(r.Context(), id)
	}
	if err := s.Engine.DeleteGraph(r.Context(), id, actor); err != nil {
		return err
	}
	// The graph is gone; its environment images and caches go too. Failures
	// leave orphans in the sandbox daemon only, so they are logged.
	if hadEnv {
		if err := s.Environments.Reset(r.Context(), id, actor); err != nil {
			s.Log.Warn("environment cleanup after graph deletion", "graph", id, "err", err)
		}
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) graphVersions(w http.ResponseWriter, r *http.Request) error {
	vs, err := s.Store.ListGraphVersions(r.Context(), r.PathValue("id"))
	if err != nil {
		return err
	}
	writeJSON(w, 200, vs)
	return nil
}

func (s *Server) exportGraph(w http.ResponseWriter, r *http.Request) error {
	v, _ := strconv.Atoi(r.URL.Query().Get("version"))
	g, _, err := s.Engine.GetGraphDocument(r.Context(), r.PathValue("id"), v)
	if err != nil {
		return err
	}
	if r.URL.Query().Get("format") == "json" {
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.json"`, g.Metadata.ID))
		writeJSON(w, 200, g)
		return nil
	}
	b, err := yaml.Marshal(g)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/yaml")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.yaml"`, g.Metadata.ID))
	_, err = w.Write(b)
	return err
}

func (s *Server) graphAction(w http.ResponseWriter, r *http.Request) error {
	id, action := r.PathValue("id"), r.PathValue("action")
	switch action {
	case "start", "stop", "pause", "resume", "kill":
	default:
		return engine.ErrNotFound
	}
	if err := s.Engine.GraphAction(r.Context(), id, action, identityFrom(r.Context()).Name); err != nil {
		return err
	}
	return s.graphStatus(w, r)
}

func (s *Server) graphStatus(w http.ResponseWriter, r *http.Request) error {
	st, err := s.Engine.Status(r.Context(), r.PathValue("id"))
	if err != nil {
		return err
	}
	writeJSON(w, 200, st)
	return nil
}

func (s *Server) nodeAction(w http.ResponseWriter, r *http.Request) error {
	action := r.PathValue("action")
	switch action {
	case "start", "stop", "pause", "resume", "kill":
	default:
		return engine.ErrNotFound
	}
	if err := s.Engine.NodeAction(r.Context(), r.PathValue("id"), r.PathValue("node"), action, identityFrom(r.Context()).Name); err != nil {
		return err
	}
	return s.graphStatus(w, r)
}

func (s *Server) scaleNode(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		Replicas int `json:"replicas"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return badRequest("invalid body: %v", err)
	}
	if err := s.Engine.Scale(r.Context(), r.PathValue("id"), r.PathValue("node"), body.Replicas, identityFrom(r.Context()).Name); err != nil {
		return err
	}
	return s.graphStatus(w, r)
}

func (s *Server) listChat(w http.ResponseWriter, r *http.Request) error {
	ms, err := s.Store.ListOperatorMessages(r.Context(), r.PathValue("id"), r.PathValue("node"), 200)
	if err != nil {
		return err
	}
	if ms == nil {
		ms = []*domain.OperatorMessage{}
	}
	writeJSON(w, 200, ms)
	return nil
}

func (s *Server) postChat(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		Text    string `json:"text"`
		Replica *int   `json:"replica"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return badRequest("invalid body: %v", err)
	}
	rep := -1
	if body.Replica != nil {
		rep = *body.Replica
	}
	m, err := s.Engine.SendOperatorMessage(r.Context(), r.PathValue("id"), r.PathValue("node"), rep, body.Text, identityFrom(r.Context()).Name)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, m)
	return nil
}

func (s *Server) inject(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		Type     string          `json:"type"`
		Payload  json.RawMessage `json:"payload"`
		Target   string          `json:"target"`
		Metadata map[string]any  `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return badRequest("invalid body: %v", err)
	}
	m, err := s.Engine.Inject(r.Context(), r.PathValue("id"), r.PathValue("channel"), body.Type, body.Payload, body.Target, body.Metadata, identityFrom(r.Context()).Name)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, m)
	return nil
}

func qint(r *http.Request, k string, def int) int {
	if v, err := strconv.Atoi(r.URL.Query().Get(k)); err == nil {
		return v
	}
	return def
}

func (s *Server) listMessages(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	ms, err := s.Store.ListMessages(r.Context(), storage.MessageFilter{
		GraphID: r.PathValue("id"), Channel: q.Get("channel"), NodeID: q.Get("node"),
		CorrelationID: q.Get("correlationId"), Status: domain.MessageStatus(q.Get("status")), Limit: qint(r, "limit", 100),
	})
	if err != nil {
		return err
	}
	if ms == nil {
		ms = []*domain.MessageRecord{}
	}
	writeJSON(w, 200, ms)
	return nil
}

func (s *Server) getMessage(w http.ResponseWriter, r *http.Request) error {
	ms, err := s.Store.GetMessage(r.Context(), r.PathValue("mid"))
	if err != nil {
		return err
	}
	writeJSON(w, 200, ms)
	return nil
}

func (s *Server) listExecutions(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	xs, err := s.Store.ListExecutions(r.Context(), storage.ExecutionFilter{
		GraphID: r.PathValue("id"), NodeID: q.Get("node"), Status: domain.ExecutionStatus(q.Get("status")), Limit: qint(r, "limit", 50),
	})
	if err != nil {
		return err
	}
	if xs == nil {
		xs = []*domain.Execution{}
	}
	writeJSON(w, 200, xs)
	return nil
}

func (s *Server) getExecution(w http.ResponseWriter, r *http.Request) error {
	x, err := s.Store.GetExecution(r.Context(), r.PathValue("eid"))
	if err != nil {
		return err
	}
	writeJSON(w, 200, x)
	return nil
}

func eventFilter(r *http.Request, graphID string) storage.EventFilter {
	q := r.URL.Query()
	f := storage.EventFilter{GraphID: graphID, NodeID: q.Get("node"), Limit: qint(r, "limit", 200), Ascending: true}
	f.AfterSeq, _ = strconv.ParseInt(q.Get("after"), 10, 64)
	if t := q.Get("types"); t != "" {
		for _, x := range strings.Split(t, ",") {
			f.Types = append(f.Types, domain.EventType(strings.TrimSpace(x)))
		}
	}
	return f
}

func (s *Server) listEvents(w http.ResponseWriter, r *http.Request) error {
	f := eventFilter(r, r.PathValue("id"))
	if r.URL.Query().Get("after") == "" {
		// Without a resume point, return the latest page (still ascending).
		f.Ascending = false
		evs, err := s.Store.ListEvents(r.Context(), f)
		if err != nil {
			return err
		}
		for i, j := 0, len(evs)-1; i < j; i, j = i+1, j-1 {
			evs[i], evs[j] = evs[j], evs[i]
		}
		if evs == nil {
			evs = []*domain.Event{}
		}
		writeJSON(w, 200, evs)
		return nil
	}
	evs, err := s.Store.ListEvents(r.Context(), f)
	if err != nil {
		return err
	}
	if evs == nil {
		evs = []*domain.Event{}
	}
	writeJSON(w, 200, evs)
	return nil
}

// stream is the SSE endpoint. Clients resume with ?after=<seq> or the
// standard Last-Event-ID header; without either they start at "now".
func (s *Server) stream(w http.ResponseWriter, r *http.Request) error {
	fl, ok := w.(http.Flusher)
	if !ok {
		return errors.New("streaming unsupported")
	}
	graphID := r.URL.Query().Get("graph")
	f := eventFilter(r, graphID)
	f.Limit = 500
	if lei := r.Header.Get("Last-Event-ID"); lei != "" {
		f.AfterSeq, _ = strconv.ParseInt(lei, 10, 64)
	} else if r.URL.Query().Get("after") == "" {
		f.AfterSeq = s.Bus.LatestSeq(r.Context(), graphID)
	}
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(200)
	_, _ = fmt.Fprintf(w, "retry: 3000\n: connected\n\n")
	fl.Flush()

	ctx := r.Context()
	err := s.Bus.Subscribe(ctx, f, func(ev *domain.Event) error {
		if ev == nil {
			// Heartbeat: a comment keeps proxies from closing an idle stream.
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return err
			}
			fl.Flush()
			return nil
		}
		b, err := json.Marshal(ev)
		if err != nil {
			return nil
		}
		if _, err := fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", ev.Seq, ev.Type, b); err != nil {
			return err
		}
		fl.Flush()
		return nil
	})
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

func (s *Server) listApprovals(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	as, err := s.Store.ListApprovals(r.Context(), storage.ApprovalFilter{
		GraphID: q.Get("graph"), Status: domain.ApprovalStatus(q.Get("status")), Limit: qint(r, "limit", 200),
	})
	if err != nil {
		return err
	}
	if as == nil {
		as = []*domain.Approval{}
	}
	writeJSON(w, 200, as)
	return nil
}

func (s *Server) decide(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		Comment string `json:"comment"`
		Answer  string `json:"answer"`
	}
	if r.ContentLength != 0 {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	approve := false
	switch r.PathValue("decision") {
	case "approve":
		approve = true
	case "reject":
	default:
		return engine.ErrNotFound
	}
	a, err := s.Approvals.Decide(r.Context(), r.PathValue("aid"), approve, identityFrom(r.Context()).Name, body.Comment, body.Answer)
	if err != nil {
		return err
	}
	writeJSON(w, 200, a)
	return nil
}

func (s *Server) models(w http.ResponseWriter, r *http.Request) error {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	ms, err := s.Models.Models(ctx, r.PathValue("name"))
	if err != nil {
		return fmt.Errorf("%w: %v", engine.ErrNotFound, err)
	}
	if ms == nil {
		ms = []llm.ModelInfo{}
	}
	writeJSON(w, 200, ms)
	return nil
}

func (s *Server) toolsList(w http.ResponseWriter, r *http.Request) error {
	writeJSON(w, 200, s.Tools.Describe())
	return nil
}

// --- workspace browser ----------------------------------------------------

func (s *Server) browseRoot(r *http.Request) (string, error) {
	g, _, err := s.Engine.GetGraph(r.Context(), r.PathValue("id"), 0)
	if err != nil {
		return "", err
	}
	root, err := s.Workspaces.GraphDir(g)
	if err != nil {
		return "", err
	}
	// A worktree replica's files live in its own checkout.
	if node := r.URL.Query().Get("node"); node != "" {
		if n := g.Node(node); n != nil && n.Workspace.Mode == domain.WorkspaceWorktree {
			rep := qint(r, "replica", 0)
			root = filepath.Join(s.Workspaces.Root, ".worktrees", g.Metadata.ID, fmt.Sprintf("%s-%d", node, rep))
		}
	}
	return root, nil
}

func (s *Server) listFiles(w http.ResponseWriter, r *http.Request) error {
	root, err := s.browseRoot(r)
	if err != nil {
		return err
	}
	abs, rel, err := tools.ResolvePath(root, r.URL.Query().Get("path"))
	if err != nil {
		return badRequest("%v", err)
	}
	entries, err := os.ReadDir(abs)
	if errors.Is(err, os.ErrNotExist) {
		writeJSON(w, 200, []any{})
		return nil
	}
	if err != nil {
		return badRequest("%v", err)
	}
	type file struct {
		Name    string    `json:"name"`
		Path    string    `json:"path"`
		Dir     bool      `json:"dir"`
		Size    int64     `json:"size"`
		ModTime time.Time `json:"modTime"`
	}
	out := []file{}
	for _, e := range entries {
		if e.Name() == ".git" {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, file{e.Name(), filepath.ToSlash(filepath.Join(rel, e.Name())), e.IsDir(), info.Size(), info.ModTime()})
		if len(out) >= 2000 {
			break
		}
	}
	writeJSON(w, 200, out)
	return nil
}

func (s *Server) fileContent(w http.ResponseWriter, r *http.Request) error {
	root, err := s.browseRoot(r)
	if err != nil {
		return err
	}
	abs, _, err := tools.ResolvePath(root, r.URL.Query().Get("path"))
	if err != nil {
		return badRequest("%v", err)
	}
	f, err := os.Open(abs)
	if err != nil {
		return fmt.Errorf("%w: %v", engine.ErrNotFound, err)
	}
	defer f.Close()
	// Served as plain text with nosniff so agent-written HTML can never
	// execute in the console's origin.
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Security-Policy", "default-src 'none'")
	_, err = io.Copy(w, io.LimitReader(f, 4<<20))
	return err
}

// spa serves the built console with an index.html fallback.
func spa(dir string) http.Handler {
	fs := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := filepath.Join(dir, filepath.Clean("/"+r.URL.Path))
		if st, err := os.Stat(p); err != nil || st.IsDir() {
			if r.URL.Path != "/" && !strings.HasSuffix(r.URL.Path, "/") {
				r.URL.Path = "/"
			}
		}
		if r.URL.Path == "/" || strings.HasSuffix(r.URL.Path, ".html") {
			w.Header().Set("Cache-Control", "no-cache")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		fs.ServeHTTP(w, r)
	})
}
