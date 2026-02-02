package types

// Component is the base interface for all pluggable components.
// All blockberry components (reactors, stores, handlers) should implement this interface
// to enable consistent lifecycle management and dependency injection.
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

// ConfigurableComponent can be configured and validated before starting.
// Use this for components that have configuration that needs validation.
type ConfigurableComponent interface {
	Component

	// Validate checks the component's configuration.
	// Should be called before Start() to catch configuration errors early.
	// Returns nil if configuration is valid, error otherwise.
	Validate() error
}

// LifecycleAware components receive lifecycle events from the container.
// This allows components to perform setup/teardown that depends on other components.
type LifecycleAware interface {
	// OnStart is called after all components are created but before Start().
	// Use this for initialization that depends on other components being available.
	OnStart() error

	// OnStop is called before Stop() to prepare for shutdown.
	// Use this for cleanup that must happen before the component stops.
	OnStop() error
}

// Named provides a name for a component used in logging and debugging.
type Named interface {
	// Name returns the component name for identification.
	Name() string
}

// Dependent declares dependencies on other components.
// The container uses this to determine startup order.
type Dependent interface {
	// Dependencies returns the names of components this component depends on.
	// The container will ensure all dependencies are started before this component.
	Dependencies() []string
}

// HealthChecker provides health status for monitoring.
type HealthChecker interface {
	// HealthCheck returns nil if the component is healthy, error otherwise.
	// This is used for liveness and readiness probes.
	HealthCheck() error
}

// ComponentInfo provides metadata about a component.
type ComponentInfo struct {
	// Name is the unique identifier for this component.
	Name string

	// Type is the type of component (e.g., "reactor", "store", "handler").
	Type string

	// Running indicates if the component is currently running.
	Running bool

	// Dependencies lists the names of components this depends on.
	Dependencies []string

	// Health is the current health status (nil = healthy).
	Health error
}

// GetComponentInfo extracts ComponentInfo from a Component.
// This is a helper function for introspection and debugging.
func GetComponentInfo(c Component) ComponentInfo {
	info := ComponentInfo{
		Running: c.IsRunning(),
	}

	if named, ok := c.(Named); ok {
		info.Name = named.Name()
	}

	if dep, ok := c.(Dependent); ok {
		info.Dependencies = dep.Dependencies()
	}

	if hc, ok := c.(HealthChecker); ok {
		info.Health = hc.HealthCheck()
	}

	return info
}
