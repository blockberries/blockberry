package abi

// Component is the base interface for all manageable components.
// All pluggable components in blockberry (mempools, consensus engines, stores)
// must implement this interface for consistent lifecycle management.
type Component interface {
	// Start initializes and starts the component.
	// Must be idempotent (safe to call multiple times).
	// Returns an error if the component fails to start.
	Start() error

	// Stop gracefully shuts down the component.
	// Must be idempotent and safe to call even if not started.
	// Returns an error if the component fails to stop cleanly.
	Stop() error

	// IsRunning returns true if the component is currently running.
	IsRunning() bool
}

// Named components can report their name for logging and debugging.
type Named interface {
	// Name returns the component name for identification.
	Name() string
}

// HealthChecker components can report their health status.
type HealthChecker interface {
	// Health returns the current health status of the component.
	Health() HealthStatus
}

// HealthStatus represents the health of a component.
type HealthStatus struct {
	// Status is the overall health state.
	Status Health

	// Message provides additional context about the health state.
	Message string

	// Details contains component-specific health information.
	Details map[string]any
}

// Health represents the health state of a component.
type Health int

const (
	// HealthUnknown indicates the health state is not yet determined.
	HealthUnknown Health = iota

	// HealthHealthy indicates the component is functioning normally.
	HealthHealthy

	// HealthDegraded indicates the component is functioning but with issues.
	HealthDegraded

	// HealthUnhealthy indicates the component is not functioning properly.
	HealthUnhealthy
)

// String returns a string representation of the health state.
func (h Health) String() string {
	switch h {
	case HealthHealthy:
		return "healthy"
	case HealthDegraded:
		return "degraded"
	case HealthUnhealthy:
		return "unhealthy"
	default:
		return "unknown"
	}
}

// Dependent components declare their dependencies for startup ordering.
type Dependent interface {
	// Dependencies returns the names of components this component depends on.
	// The service registry will ensure all dependencies are started first.
	Dependencies() []string
}

// LifecycleAware components receive lifecycle notifications.
type LifecycleAware interface {
	// OnStart is called after all components are created but before Start().
	OnStart() error

	// OnStop is called before Stop() to prepare for shutdown.
	OnStop() error
}

// Resettable components can be reset to their initial state.
type Resettable interface {
	// Reset clears all state and returns the component to its initial condition.
	// Useful for testing and recovery scenarios.
	Reset() error
}
