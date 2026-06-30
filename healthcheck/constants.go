package healthcheck

const (
	registryStatusStopped = "stopped"
	detailKeyPanic        = "panic"
	detailKeyState        = "state"
	detailKeyExitCode     = "exitCode"
	detailStateBuilding   = "building"
	detailStateRunning    = "running"
	detailStateBuilt      = "built"
	detailStateCompleted  = "completed"
	serverErrorAction     = "Server error. Check application logs for details."

	errorTypeTimeout           = "timeout"
	errorTypeConnectionRefused = "connection_refused"
	errorTypeCircuitBreaker    = "circuit_breaker"
	errorTypePanic             = "panic"
	errorTypeServerError       = "server_error"
	errorTypeAuthError         = "auth_error"
	errorTypeProcessError      = "process_error"

	profileDevelopment = "development"
	profileProduction  = "production"
	profileStaging     = "staging"
	logFormatJSON      = "json"

	healthPath  = "/health"
	healthzPath = "/healthz"
	readyPath   = "/ready"
)
