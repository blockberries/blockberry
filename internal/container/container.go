// Package container provides a simple dependency injection container
// for managing component lifecycle and dependencies in blockberry.
package container

import (
	"errors"
	"fmt"
	"sync"

	"github.com/blockberries/blockberry/pkg/types"
)

// Common errors for the container.
var (
	// ErrComponentNotFound is returned when a component is not found.
	ErrComponentNotFound = errors.New("component not found")

	// ErrComponentAlreadyRegistered is returned when a component is already registered.
	ErrComponentAlreadyRegistered = errors.New("component already registered")

	// ErrCircularDependency is returned when a circular dependency is detected.
	ErrCircularDependency = errors.New("circular dependency detected")

	// ErrContainerAlreadyStarted is returned when trying to register after start.
	ErrContainerAlreadyStarted = errors.New("container already started")

	// ErrContainerNotStarted is returned when trying to stop before start.
	ErrContainerNotStarted = errors.New("container not started")

	// ErrDependencyNotFound is returned when a dependency is not found.
	ErrDependencyNotFound = errors.New("dependency not found")
)

// componentEntry holds a component and its metadata.
type componentEntry struct {
	name         string
	component    types.Component
	dependencies []string
}

// Container manages component lifecycle and dependencies.
// Components are started in dependency order and stopped in reverse order.
type Container struct {
	// Component registry
	components map[string]*componentEntry

	// Startup order (topologically sorted)
	order []string

	// Lifecycle state
	started bool

	mu sync.RWMutex
}

// New creates a new Container.
func New() *Container {
	return &Container{
		components: make(map[string]*componentEntry),
	}
}

// Register adds a component with its dependencies.
// Dependencies are component names that must be started before this component.
// Returns an error if the container is already started or the component is already registered.
func (c *Container) Register(name string, component types.Component, deps ...string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.started {
		return fmt.Errorf("%w: cannot register %s", ErrContainerAlreadyStarted, name)
	}

	if _, exists := c.components[name]; exists {
		return fmt.Errorf("%w: %s", ErrComponentAlreadyRegistered, name)
	}

	// If the component implements Dependent, merge those dependencies
	if dep, ok := component.(types.Dependent); ok {
		deps = append(deps, dep.Dependencies()...)
	}

	c.components[name] = &componentEntry{
		name:         name,
		component:    component,
		dependencies: deps,
	}

	return nil
}

// Get retrieves a component by name.
// Returns an error if the component is not found.
func (c *Container) Get(name string) (types.Component, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.components[name]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrComponentNotFound, name)
	}

	return entry.component, nil
}

// MustGet retrieves a component by name and panics if not found.
// Use this only when you're certain the component exists.
func (c *Container) MustGet(name string) types.Component {
	component, err := c.Get(name)
	if err != nil {
		panic(err)
	}
	return component
}

// Has returns true if a component with the given name is registered.
func (c *Container) Has(name string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, exists := c.components[name]
	return exists
}

// Names returns the names of all registered components.
func (c *Container) Names() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	names := make([]string, 0, len(c.components))
	for name := range c.components {
		names = append(names, name)
	}
	return names
}

// Count returns the number of registered components.
func (c *Container) Count() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.components)
}

// StartAll starts all components in dependency order.
// Components are started according to their dependencies - a component's
// dependencies are always started before it.
// Returns the first error encountered; already-started components remain running.
func (c *Container) StartAll() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.started {
		return nil // Already started, idempotent
	}

	// Compute startup order
	order, err := c.topologicalSort()
	if err != nil {
		return err
	}
	c.order = order

	// Call OnStart for LifecycleAware components
	for _, name := range c.order {
		entry := c.components[name]
		if lc, ok := entry.component.(types.LifecycleAware); ok {
			if err := lc.OnStart(); err != nil {
				return fmt.Errorf("OnStart failed for %s: %w", name, err)
			}
		}
	}

	// Validate ConfigurableComponents
	for _, name := range c.order {
		entry := c.components[name]
		if cfg, ok := entry.component.(types.ConfigurableComponent); ok {
			if err := cfg.Validate(); err != nil {
				return fmt.Errorf("validation failed for %s: %w", name, err)
			}
		}
	}

	// Start all components in order
	for _, name := range c.order {
		entry := c.components[name]
		if err := entry.component.Start(); err != nil {
			return fmt.Errorf("failed to start %s: %w", name, err)
		}
	}

	c.started = true
	return nil
}

// StopAll stops all components in reverse dependency order.
// Components are stopped in reverse order - dependents are stopped before
// their dependencies.
// Returns the first error encountered; all components will be stopped regardless.
func (c *Container) StopAll() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.started {
		return nil // Not started, idempotent
	}

	var firstErr error

	// Call OnStop for LifecycleAware components (in reverse order)
	for i := len(c.order) - 1; i >= 0; i-- {
		name := c.order[i]
		entry := c.components[name]
		if lc, ok := entry.component.(types.LifecycleAware); ok {
			if err := lc.OnStop(); err != nil && firstErr == nil {
				firstErr = fmt.Errorf("OnStop failed for %s: %w", name, err)
			}
		}
	}

	// Stop all components in reverse order
	for i := len(c.order) - 1; i >= 0; i-- {
		name := c.order[i]
		entry := c.components[name]
		if err := entry.component.Stop(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("failed to stop %s: %w", name, err)
		}
	}

	c.started = false
	return firstErr
}

// IsStarted returns true if the container has been started.
func (c *Container) IsStarted() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.started
}

// StartupOrder returns the computed startup order after StartAll is called.
// Returns nil if StartAll has not been called yet.
func (c *Container) StartupOrder() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.order == nil {
		return nil
	}
	// Return a copy to prevent mutation
	result := make([]string, len(c.order))
	copy(result, c.order)
	return result
}

// topologicalSort performs a topological sort of components based on dependencies.
// Returns an error if a circular dependency is detected or a dependency is missing.
// Must be called with c.mu held.
func (c *Container) topologicalSort() ([]string, error) {
	// Build adjacency list and in-degree map
	inDegree := make(map[string]int)
	graph := make(map[string][]string) // dependency -> dependents

	// Initialize all components with 0 in-degree
	for name := range c.components {
		inDegree[name] = 0
		graph[name] = nil
	}

	// Build graph: for each component, add edges from its dependencies to it
	for name, entry := range c.components {
		for _, dep := range entry.dependencies {
			if _, exists := c.components[dep]; !exists {
				return nil, fmt.Errorf("%w: %s depends on %s", ErrDependencyNotFound, name, dep)
			}
			graph[dep] = append(graph[dep], name)
			inDegree[name]++
		}
	}

	// Kahn's algorithm for topological sort
	var queue []string
	for name, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, name)
		}
	}

	var order []string
	for len(queue) > 0 {
		// Pop from queue
		current := queue[0]
		queue = queue[1:]
		order = append(order, current)

		// Reduce in-degree of dependents
		for _, dependent := range graph[current] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	// Check for cycles
	if len(order) != len(c.components) {
		// Find components involved in the cycle
		var cyclic []string
		for name, degree := range inDegree {
			if degree > 0 {
				cyclic = append(cyclic, name)
			}
		}
		return nil, fmt.Errorf("%w: involving %v", ErrCircularDependency, cyclic)
	}

	return order, nil
}

// ComponentInfo returns information about all registered components.
func (c *Container) ComponentInfo() []types.ComponentInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	infos := make([]types.ComponentInfo, 0, len(c.components))
	for name, entry := range c.components {
		info := types.GetComponentInfo(entry.component)
		info.Name = name // Use registered name
		info.Dependencies = entry.dependencies
		infos = append(infos, info)
	}
	return infos
}
