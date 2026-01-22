package prometheus

import (
	"sync"
)

// MetricsRecorder provides thread-safe methods for recording metrics
type MetricsRecorder struct {
	mu sync.RWMutex
}

var defaultRecorder = &MetricsRecorder{}

// RecordTraffic records traffic for a user
func RecordTraffic(userHash string, sent, recv int) {
	if sent > 0 {
		UserUploadBytesTotal.WithLabelValues(userHash).Add(float64(sent))
		UploadBytesTotal.Add(float64(sent))
	}
	if recv > 0 {
		UserDownloadBytesTotal.WithLabelValues(userHash).Add(float64(recv))
		DownloadBytesTotal.Add(float64(recv))
	}
}

// SetUserSpeed sets the current speed for a user
func SetUserSpeed(userHash string, uploadSpeed, downloadSpeed uint64) {
	UserUploadSpeed.WithLabelValues(userHash).Set(float64(uploadSpeed))
	UserDownloadSpeed.WithLabelValues(userHash).Set(float64(downloadSpeed))
}

// RecordConnectionOpen records a new connection opening
func RecordConnectionOpen(userHash string) {
	ActiveConnections.Inc()
	ConnectionsTotal.Inc()
	if userHash != "" {
		UserActiveConnections.WithLabelValues(userHash).Inc()
	}
}

// RecordConnectionClose records a connection closing
func RecordConnectionClose(userHash string) {
	ActiveConnections.Dec()
	if userHash != "" {
		UserActiveConnections.WithLabelValues(userHash).Dec()
	}
}

// RecordAuthAttempt records an authentication attempt
func RecordAuthAttempt(success bool) {
	if success {
		AuthenticationAttempts.WithLabelValues("success").Inc()
	} else {
		AuthenticationAttempts.WithLabelValues("failure").Inc()
	}
}

// SetUserCount sets the total user count
func SetUserCount(count int) {
	TotalUsers.Set(float64(count))
}

// SetUserActiveIPs sets the number of active IPs for a user
func SetUserActiveIPs(userHash string, count int) {
	UserActiveIPs.WithLabelValues(userHash).Set(float64(count))
}

// RecordRelayConnectionError records a relay connection error
func RecordRelayConnectionError() {
	RelayConnectionErrors.Inc()
}

// RecordRelayPacketError records a relay packet error
func RecordRelayPacketError() {
	RelayPacketErrors.Inc()
}

// RecordRedirectedConnection records a redirected connection
func RecordRedirectedConnection() {
	RedirectedConnections.Inc()
}

// CleanupUserMetrics cleans up metrics for a deleted user
func CleanupUserMetrics(userHash string) {
	UserUploadBytesTotal.DeleteLabelValues(userHash)
	UserDownloadBytesTotal.DeleteLabelValues(userHash)
	UserUploadSpeed.DeleteLabelValues(userHash)
	UserDownloadSpeed.DeleteLabelValues(userHash)
	UserActiveConnections.DeleteLabelValues(userHash)
	UserActiveIPs.DeleteLabelValues(userHash)
}
