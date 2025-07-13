package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// ActiveConnections tracks the number of active WebSocket connections.
	ActiveConnections = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "gateway_active_connections",
		Help: "The total number of active WebSocket connections.",
	})

	// MessagesReceived counts the total number of messages received, labeled by OCPP version and message type.
	MessagesReceived = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "gateway_messages_received_total",
		Help: "Total number of messages received from charge points.",
	}, []string{"ocpp_version", "message_type"})

	// EventsPublished counts the total number of events published to Kafka, labeled by event type.
	EventsPublished = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "gateway_events_published_total",
		Help: "Total number of events published to the message broker.",
	}, []string{"event_type"})

	// CommandsConsumed counts the total number of commands consumed from Kafka, labeled by command name.
	CommandsConsumed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "gateway_commands_consumed_total",
		Help: "Total number of commands consumed from the message broker.",
	}, []string{"command_name"})

	// MessageProcessingDuration observes the duration of message processing, labeled by message type.
	MessageProcessingDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "gateway_message_processing_duration_seconds",
		Help:    "Histogram of message processing times.",
		Buckets: prometheus.LinearBuckets(0.01, 0.01, 10), // 10 buckets, starting at 0.01s, 0.01s increment
	}, []string{"message_type"})
)

// RegisterMetrics registers all the defined Prometheus metrics.
// In this implementation, we use promauto which automatically registers the metrics.
// This function is kept for conceptual clarity and potential future use if we stop using promauto.
func RegisterMetrics() {
	// With promauto, registration is automatic.
	// This function is conceptually a placeholder.
}