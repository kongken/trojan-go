package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"
)

const (
	// Namespace for all metrics
	Namespace = "trojan"
)

var (
	// User traffic metrics
	UserUploadBytesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "user_upload_bytes_total",
			Help:      "Total uploaded bytes per user",
		},
		[]string{"user_hash"},
	)

	UserDownloadBytesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "user_download_bytes_total",
			Help:      "Total downloaded bytes per user",
		},
		[]string{"user_hash"},
	)

	// User speed metrics (gauges since speeds change)
	UserUploadSpeed = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "user_upload_speed_bytes",
			Help:      "Current upload speed in bytes per second per user",
		},
		[]string{"user_hash"},
	)

	UserDownloadSpeed = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "user_download_speed_bytes",
			Help:      "Current download speed in bytes per second per user",
		},
		[]string{"user_hash"},
	)

	// Connection metrics
	ActiveConnections = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "active_connections",
			Help:      "Number of currently active connections",
		},
	)

	ConnectionsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "connections_total",
			Help:      "Total number of connections since start",
		},
	)

	// User connection metrics
	UserActiveConnections = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "user_active_connections",
			Help:      "Number of active connections per user",
		},
		[]string{"user_hash"},
	)

	UserActiveIPs = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "user_active_ips",
			Help:      "Number of active IPs per user",
		},
		[]string{"user_hash"},
	)

	// Authentication metrics
	AuthenticationAttempts = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "authentication_attempts_total",
			Help:      "Total authentication attempts",
		},
		[]string{"result"}, // "success" or "failure"
	)

	// User count
	TotalUsers = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "total_users",
			Help:      "Total number of registered users",
		},
	)

	// Traffic total (aggregated)
	UploadBytesTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "upload_bytes_total",
			Help:      "Total uploaded bytes",
		},
	)

	DownloadBytesTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "download_bytes_total",
			Help:      "Total downloaded bytes",
		},
	)

	// Relay metrics
	RelayConnectionErrors = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "relay_connection_errors_total",
			Help:      "Total relay connection errors",
		},
	)

	RelayPacketErrors = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "relay_packet_errors_total",
			Help:      "Total relay packet errors",
		},
	)

	// Redirector metrics
	RedirectedConnections = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "redirected_connections_total",
			Help:      "Total number of connections redirected to fallback",
		},
	)

	// Server uptime
	ServerStartTime = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "server_start_time_seconds",
			Help:      "Unix timestamp of server start time",
		},
	)
)

// RegisterAll registers all metrics with the provided registry
func RegisterAll(registry *prometheus.Registry) {
	registry.MustRegister(
		// User traffic
		UserUploadBytesTotal,
		UserDownloadBytesTotal,
		UserUploadSpeed,
		UserDownloadSpeed,

		// Connections
		ActiveConnections,
		ConnectionsTotal,
		UserActiveConnections,
		UserActiveIPs,

		// Authentication
		AuthenticationAttempts,

		// User count
		TotalUsers,

		// Total traffic
		UploadBytesTotal,
		DownloadBytesTotal,

		// Relay
		RelayConnectionErrors,
		RelayPacketErrors,

		// Redirector
		RedirectedConnections,

		// Server info
		ServerStartTime,
	)
}

// RegisterDefault registers all metrics with the default registry
func RegisterDefault() {
	RegisterAll(prometheus.DefaultRegisterer.(*prometheus.Registry))
}

func init() {
	// Register with default prometheus registry
	prometheus.MustRegister(
		// User traffic
		UserUploadBytesTotal,
		UserDownloadBytesTotal,
		UserUploadSpeed,
		UserDownloadSpeed,

		// Connections
		ActiveConnections,
		ConnectionsTotal,
		UserActiveConnections,
		UserActiveIPs,

		// Authentication
		AuthenticationAttempts,

		// User count
		TotalUsers,

		// Total traffic
		UploadBytesTotal,
		DownloadBytesTotal,

		// Relay
		RelayConnectionErrors,
		RelayPacketErrors,

		// Redirector
		RedirectedConnections,

		// Server info
		ServerStartTime,
	)
}
