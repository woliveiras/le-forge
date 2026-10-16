// Package observability wires metrics (Prometheus), traces and logs
// (OpenTelemetry/OTLP) for the whole platform. Metric names are part of the
// public contract (dashboards and alerts depend on them): add, never rename.
package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const ns = "leforge"

var (
	EnvironmentCleanup        = promauto.NewCounterVec(prometheus.CounterOpts{Namespace: ns, Name: "environment_cleanup_total", Help: "Shared resource cleanup requests by outcome."}, []string{"outcome"})
	EnvironmentCleanupSeconds = promauto.NewHistogram(prometheus.HistogramOpts{Namespace: ns, Name: "environment_cleanup_duration_seconds", Help: "Shared resource cleanup request duration."})

	SandboxLifecycle        = promauto.NewCounterVec(prometheus.CounterOpts{Namespace: ns, Name: "sandbox_lifecycle_total", Help: "Sandbox suspend/resume operations by bounded outcome."}, []string{"action", "outcome"})
	SandboxLifecycleSeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{Namespace: ns, Name: "sandbox_lifecycle_duration_seconds", Help: "Sandbox suspend/resume latency including capacity admission."}, []string{"action"})

	SandboxAdmissions       = promauto.NewCounterVec(prometheus.CounterOpts{Namespace: ns, Name: "sandbox_capacity_transitions_total", Help: "Sandbox admission transitions by bounded state."}, []string{"state"})
	SandboxAdmissionSeconds = promauto.NewHistogram(prometheus.HistogramOpts{Namespace: ns, Name: "sandbox_admission_duration_seconds", Help: "Time spent waiting for sandbox capacity, including inventory and reservation."})

	EnvironmentBuilds = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: ns, Name: "environment_builds_total",
		Help: "Project environment builds by outcome (ready, failed).",
	}, []string{"outcome"})
	EnvironmentBuildSeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: ns, Name: "environment_build_duration_seconds",
		Help:    "Duration of project environment builds.",
		Buckets: []float64{5, 15, 30, 60, 120, 300, 600, 1200, 2400, 3600, 7200},
	}, []string{"outcome"})

	MessagesPublished = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: ns, Name: "messages_published_total",
		Help: "Messages published to a channel.",
	}, []string{"graph", "channel", "type"})

	MessagesConsumed = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: ns, Name: "messages_consumed_total",
		Help: "Messages consumed by a node, by outcome (acked, nacked, dead, duplicate).",
	}, []string{"graph", "node", "outcome"})

	MessagesDropped = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: ns, Name: "messages_dropped_total",
		Help: "Emitted messages that matched no route or exceeded the hop limit.",
	}, []string{"graph", "node", "reason"})

	QueuePending = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: ns, Name: "queue_pending",
		Help: "Messages waiting in a consumer group.",
	}, []string{"graph", "channel", "group"})

	QueueInFlight = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: ns, Name: "queue_inflight",
		Help: "Messages delivered and not yet acknowledged in a consumer group.",
	}, []string{"graph", "channel", "group"})

	MessageLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: ns, Name: "message_queue_latency_seconds",
		Help:    "Time from publish to the start of the turn that consumed the message.",
		Buckets: prometheus.ExponentialBuckets(0.01, 3, 12),
	}, []string{"graph", "node"})

	Executions = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: ns, Name: "executions_total",
		Help: "Agent turns by final status.",
	}, []string{"graph", "node", "status"})

	ExecutionDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: ns, Name: "execution_duration_seconds",
		Help:    "Duration of agent turns.",
		Buckets: prometheus.ExponentialBuckets(0.05, 2.5, 12),
	}, []string{"graph", "node"})

	Replicas = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: ns, Name: "replicas",
		Help: "Replicas per node by state.",
	}, []string{"graph", "node", "state"})

	LLMRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: ns, Name: "llm_requests_total",
		Help: "LLM requests by provider, model and outcome.",
	}, []string{"provider", "model", "outcome"})

	LLMDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: ns, Name: "llm_request_duration_seconds",
		Help:    "LLM request latency.",
		Buckets: prometheus.ExponentialBuckets(0.05, 2, 12),
	}, []string{"provider", "model"})

	LLMTokens = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: ns, Name: "llm_tokens_total",
		Help: "Tokens consumed by direction (input, output).",
	}, []string{"provider", "model", "direction"})

	ToolInvocations = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: ns, Name: "tool_invocations_total",
		Help: "Tool invocations by outcome (ok, error, denied, rejected, rate_limited).",
	}, []string{"tool", "outcome"})

	ToolDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: ns, Name: "tool_duration_seconds",
		Help:    "Tool invocation latency.",
		Buckets: prometheus.ExponentialBuckets(0.005, 3, 12),
	}, []string{"tool"})

	ApprovalsPending = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: ns, Name: "approvals_pending",
		Help: "Approvals waiting for a human.",
	})

	Approvals = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: ns, Name: "approvals_total",
		Help: "Approvals by kind and final status.",
	}, []string{"kind", "status"})

	APIRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: ns, Name: "api_requests_total",
		Help: "HTTP API requests.",
	}, []string{"method", "route", "code"})

	APIDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: ns, Name: "api_request_duration_seconds",
		Help:    "HTTP API latency.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "route"})
)
