package types

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// mockComponent implements Component for testing.
type mockComponent struct {
	running bool
	started bool
	stopped bool
}

func (m *mockComponent) Start() error {
	m.running = true
	m.started = true
	return nil
}

func (m *mockComponent) Stop() error {
	m.running = false
	m.stopped = true
	return nil
}

func (m *mockComponent) IsRunning() bool {
	return m.running
}

// mockConfigurableComponent implements ConfigurableComponent.
type mockConfigurableComponent struct {
	mockComponent
	valid bool
}

func (m *mockConfigurableComponent) Validate() error {
	if !m.valid {
		return errors.New("invalid configuration")
	}
	return nil
}

// mockFullComponent implements all optional interfaces.
type mockFullComponent struct {
	mockComponent
	name         string
	dependencies []string
	healthy      bool
	onStartErr   error
	onStopErr    error
}

func (m *mockFullComponent) Name() string {
	return m.name
}

func (m *mockFullComponent) Dependencies() []string {
	return m.dependencies
}

func (m *mockFullComponent) HealthCheck() error {
	if !m.healthy {
		return errors.New("unhealthy")
	}
	return nil
}

func (m *mockFullComponent) OnStart() error {
	return m.onStartErr
}

func (m *mockFullComponent) OnStop() error {
	return m.onStopErr
}

func TestComponent_BasicLifecycle(t *testing.T) {
	c := &mockComponent{}

	t.Run("initially not running", func(t *testing.T) {
		require.False(t, c.IsRunning())
	})

	t.Run("running after start", func(t *testing.T) {
		err := c.Start()
		require.NoError(t, err)
		require.True(t, c.IsRunning())
		require.True(t, c.started)
	})

	t.Run("not running after stop", func(t *testing.T) {
		err := c.Stop()
		require.NoError(t, err)
		require.False(t, c.IsRunning())
		require.True(t, c.stopped)
	})
}

func TestConfigurableComponent_Validation(t *testing.T) {
	t.Run("valid configuration", func(t *testing.T) {
		c := &mockConfigurableComponent{valid: true}
		require.NoError(t, c.Validate())
	})

	t.Run("invalid configuration", func(t *testing.T) {
		c := &mockConfigurableComponent{valid: false}
		require.Error(t, c.Validate())
	})
}

func TestGetComponentInfo_BasicComponent(t *testing.T) {
	c := &mockComponent{}

	info := GetComponentInfo(c)
	require.False(t, info.Running)
	require.Empty(t, info.Name)
	require.Empty(t, info.Dependencies)
	require.Nil(t, info.Health)

	// Start the component
	_ = c.Start()
	info = GetComponentInfo(c)
	require.True(t, info.Running)
}

func TestGetComponentInfo_FullComponent(t *testing.T) {
	c := &mockFullComponent{
		name:         "test-component",
		dependencies: []string{"dep1", "dep2"},
		healthy:      true,
	}

	info := GetComponentInfo(c)
	require.Equal(t, "test-component", info.Name)
	require.Equal(t, []string{"dep1", "dep2"}, info.Dependencies)
	require.Nil(t, info.Health)

	// Make unhealthy
	c.healthy = false
	info = GetComponentInfo(c)
	require.Error(t, info.Health)
}

func TestGetComponentInfo_Running(t *testing.T) {
	c := &mockFullComponent{name: "test"}

	// Initially not running
	info := GetComponentInfo(c)
	require.False(t, info.Running)

	// Start
	_ = c.Start()
	info = GetComponentInfo(c)
	require.True(t, info.Running)

	// Stop
	_ = c.Stop()
	info = GetComponentInfo(c)
	require.False(t, info.Running)
}

func TestNamedInterface(t *testing.T) {
	c := &mockFullComponent{name: "my-reactor"}

	// Cast to Named interface
	named, ok := interface{}(c).(Named)
	require.True(t, ok)
	require.Equal(t, "my-reactor", named.Name())
}

func TestDependentInterface(t *testing.T) {
	c := &mockFullComponent{
		dependencies: []string{"network", "blockstore"},
	}

	// Cast to Dependent interface
	dep, ok := interface{}(c).(Dependent)
	require.True(t, ok)
	require.Equal(t, []string{"network", "blockstore"}, dep.Dependencies())
}

func TestHealthCheckerInterface(t *testing.T) {
	t.Run("healthy component", func(t *testing.T) {
		c := &mockFullComponent{healthy: true}
		hc, ok := interface{}(c).(HealthChecker)
		require.True(t, ok)
		require.NoError(t, hc.HealthCheck())
	})

	t.Run("unhealthy component", func(t *testing.T) {
		c := &mockFullComponent{healthy: false}
		hc, ok := interface{}(c).(HealthChecker)
		require.True(t, ok)
		require.Error(t, hc.HealthCheck())
	})
}

func TestLifecycleAwareInterface(t *testing.T) {
	t.Run("successful lifecycle", func(t *testing.T) {
		c := &mockFullComponent{}
		lc, ok := interface{}(c).(LifecycleAware)
		require.True(t, ok)
		require.NoError(t, lc.OnStart())
		require.NoError(t, lc.OnStop())
	})

	t.Run("OnStart error", func(t *testing.T) {
		c := &mockFullComponent{
			onStartErr: errors.New("start failed"),
		}
		lc, ok := interface{}(c).(LifecycleAware)
		require.True(t, ok)
		require.Error(t, lc.OnStart())
	})

	t.Run("OnStop error", func(t *testing.T) {
		c := &mockFullComponent{
			onStopErr: errors.New("stop failed"),
		}
		lc, ok := interface{}(c).(LifecycleAware)
		require.True(t, ok)
		require.Error(t, lc.OnStop())
	})
}

// Verify interfaces are satisfied at compile time.
var (
	_ Component             = (*mockComponent)(nil)
	_ ConfigurableComponent = (*mockConfigurableComponent)(nil)
	_ Named                 = (*mockFullComponent)(nil)
	_ Dependent             = (*mockFullComponent)(nil)
	_ HealthChecker         = (*mockFullComponent)(nil)
	_ LifecycleAware        = (*mockFullComponent)(nil)
)
