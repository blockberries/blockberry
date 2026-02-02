package container

import (
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/blockberries/blockberry/pkg/types"
)

// mockComponent is a simple test component.
type mockComponent struct {
	name       string
	started    bool
	startErr   error
	stopErr    error
	startCount int
	stopCount  int
	mu         sync.Mutex
}

func newMockComponent(name string) *mockComponent {
	return &mockComponent{name: name}
}

func (m *mockComponent) Name() string {
	return m.name
}

func (m *mockComponent) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.startErr != nil {
		return m.startErr
	}
	m.started = true
	m.startCount++
	return nil
}

func (m *mockComponent) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stopErr != nil {
		return m.stopErr
	}
	m.started = false
	m.stopCount++
	return nil
}

func (m *mockComponent) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.started
}

func (m *mockComponent) SetStartError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.startErr = err
}

func (m *mockComponent) SetStopError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopErr = err
}

func (m *mockComponent) GetStartCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.startCount
}

func (m *mockComponent) GetStopCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopCount
}

// mockLifecycleComponent implements LifecycleAware.
type mockLifecycleComponent struct {
	*mockComponent
	onStartCalled bool
	onStopCalled  bool
	onStartErr    error
	onStopErr     error
}

func newMockLifecycleComponent(name string) *mockLifecycleComponent {
	return &mockLifecycleComponent{
		mockComponent: newMockComponent(name),
	}
}

func (m *mockLifecycleComponent) OnStart() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.onStartErr != nil {
		return m.onStartErr
	}
	m.onStartCalled = true
	return nil
}

func (m *mockLifecycleComponent) OnStop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.onStopErr != nil {
		return m.onStopErr
	}
	m.onStopCalled = true
	return nil
}

// mockConfigurableComponent implements ConfigurableComponent.
type mockConfigurableComponent struct {
	*mockComponent
	validateErr error
}

func newMockConfigurableComponent(name string) *mockConfigurableComponent {
	return &mockConfigurableComponent{
		mockComponent: newMockComponent(name),
	}
}

func (m *mockConfigurableComponent) Validate() error {
	return m.validateErr
}

// mockDependentComponent implements Dependent.
type mockDependentComponent struct {
	*mockComponent
	deps []string
}

func newMockDependentComponent(name string, deps ...string) *mockDependentComponent {
	return &mockDependentComponent{
		mockComponent: newMockComponent(name),
		deps:          deps,
	}
}

func (m *mockDependentComponent) Dependencies() []string {
	return m.deps
}

// trackableComponent tracks stop order via callback.
type trackableComponent struct {
	*mockComponent
	onStopCallback func(name string)
}

func newTrackableComponent(name string, callback func(name string)) *trackableComponent {
	return &trackableComponent{
		mockComponent:  newMockComponent(name),
		onStopCallback: callback,
	}
}

func (t *trackableComponent) Stop() error {
	if t.onStopCallback != nil {
		t.onStopCallback(t.name)
	}
	return t.mockComponent.Stop()
}

// Verify interfaces are implemented correctly
var (
	_ types.Component             = (*mockComponent)(nil)
	_ types.Named                 = (*mockComponent)(nil)
	_ types.LifecycleAware        = (*mockLifecycleComponent)(nil)
	_ types.ConfigurableComponent = (*mockConfigurableComponent)(nil)
	_ types.Dependent             = (*mockDependentComponent)(nil)
)

func TestNew(t *testing.T) {
	c := New()
	require.NotNil(t, c)
	assert.NotNil(t, c.components)
	assert.Empty(t, c.components)
	assert.False(t, c.started)
}

func TestRegister(t *testing.T) {
	t.Run("successful registration", func(t *testing.T) {
		c := New()
		comp := newMockComponent("test")

		err := c.Register("test", comp)
		require.NoError(t, err)
		assert.True(t, c.Has("test"))
	})

	t.Run("registration with dependencies", func(t *testing.T) {
		c := New()
		comp1 := newMockComponent("dep1")
		comp2 := newMockComponent("comp2")

		err := c.Register("dep1", comp1)
		require.NoError(t, err)

		err = c.Register("comp2", comp2, "dep1")
		require.NoError(t, err)

		assert.True(t, c.Has("dep1"))
		assert.True(t, c.Has("comp2"))
	})

	t.Run("duplicate registration fails", func(t *testing.T) {
		c := New()
		comp := newMockComponent("test")

		err := c.Register("test", comp)
		require.NoError(t, err)

		err = c.Register("test", comp)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrComponentAlreadyRegistered)
	})

	t.Run("registration after start fails", func(t *testing.T) {
		c := New()
		comp1 := newMockComponent("test1")
		comp2 := newMockComponent("test2")

		err := c.Register("test1", comp1)
		require.NoError(t, err)

		err = c.StartAll()
		require.NoError(t, err)

		err = c.Register("test2", comp2)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrContainerAlreadyStarted)
	})

	t.Run("registration with Dependent interface", func(t *testing.T) {
		c := New()
		dep := newMockComponent("dependency")
		comp := newMockDependentComponent("comp", "dependency")

		err := c.Register("dependency", dep)
		require.NoError(t, err)

		// Registering without explicit deps, should use Dependencies() method
		err = c.Register("comp", comp)
		require.NoError(t, err)

		err = c.StartAll()
		require.NoError(t, err)

		// dependency should be started before comp
		order := c.StartupOrder()
		depIdx := -1
		compIdx := -1
		for i, name := range order {
			if name == "dependency" {
				depIdx = i
			}
			if name == "comp" {
				compIdx = i
			}
		}
		assert.True(t, depIdx < compIdx, "dependency should be started before comp")
	})
}

func TestGet(t *testing.T) {
	t.Run("get existing component", func(t *testing.T) {
		c := New()
		comp := newMockComponent("test")

		err := c.Register("test", comp)
		require.NoError(t, err)

		retrieved, err := c.Get("test")
		require.NoError(t, err)
		assert.Equal(t, comp, retrieved)
	})

	t.Run("get non-existent component", func(t *testing.T) {
		c := New()

		_, err := c.Get("nonexistent")
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrComponentNotFound)
	})
}

func TestMustGet(t *testing.T) {
	t.Run("must get existing component", func(t *testing.T) {
		c := New()
		comp := newMockComponent("test")

		err := c.Register("test", comp)
		require.NoError(t, err)

		retrieved := c.MustGet("test")
		assert.Equal(t, comp, retrieved)
	})

	t.Run("must get non-existent component panics", func(t *testing.T) {
		c := New()

		assert.Panics(t, func() {
			c.MustGet("nonexistent")
		})
	})
}

func TestHas(t *testing.T) {
	c := New()
	comp := newMockComponent("test")

	assert.False(t, c.Has("test"))

	err := c.Register("test", comp)
	require.NoError(t, err)

	assert.True(t, c.Has("test"))
	assert.False(t, c.Has("other"))
}

func TestNames(t *testing.T) {
	c := New()

	names := c.Names()
	assert.Empty(t, names)

	comp1 := newMockComponent("test1")
	comp2 := newMockComponent("test2")

	err := c.Register("test1", comp1)
	require.NoError(t, err)
	err = c.Register("test2", comp2)
	require.NoError(t, err)

	names = c.Names()
	assert.Len(t, names, 2)
	assert.Contains(t, names, "test1")
	assert.Contains(t, names, "test2")
}

func TestCount(t *testing.T) {
	c := New()
	assert.Equal(t, 0, c.Count())

	comp1 := newMockComponent("test1")
	err := c.Register("test1", comp1)
	require.NoError(t, err)
	assert.Equal(t, 1, c.Count())

	comp2 := newMockComponent("test2")
	err = c.Register("test2", comp2)
	require.NoError(t, err)
	assert.Equal(t, 2, c.Count())
}

func TestStartAll(t *testing.T) {
	t.Run("start all components", func(t *testing.T) {
		c := New()
		comp1 := newMockComponent("test1")
		comp2 := newMockComponent("test2")

		err := c.Register("test1", comp1)
		require.NoError(t, err)
		err = c.Register("test2", comp2)
		require.NoError(t, err)

		err = c.StartAll()
		require.NoError(t, err)

		assert.True(t, comp1.IsRunning())
		assert.True(t, comp2.IsRunning())
		assert.True(t, c.IsStarted())
	})

	t.Run("start is idempotent", func(t *testing.T) {
		c := New()
		comp := newMockComponent("test")

		err := c.Register("test", comp)
		require.NoError(t, err)

		err = c.StartAll()
		require.NoError(t, err)

		err = c.StartAll()
		require.NoError(t, err)

		assert.Equal(t, 1, comp.GetStartCount())
	})

	t.Run("start with dependency order", func(t *testing.T) {
		c := New()
		// A depends on B, B depends on C
		// Start order should be: C, B, A
		compA := newMockComponent("A")
		compB := newMockComponent("B")
		compC := newMockComponent("C")

		err := c.Register("A", compA, "B")
		require.NoError(t, err)
		err = c.Register("B", compB, "C")
		require.NoError(t, err)
		err = c.Register("C", compC)
		require.NoError(t, err)

		err = c.StartAll()
		require.NoError(t, err)

		order := c.StartupOrder()
		require.Len(t, order, 3)

		// Find indices
		idxA, idxB, idxC := -1, -1, -1
		for i, name := range order {
			switch name {
			case "A":
				idxA = i
			case "B":
				idxB = i
			case "C":
				idxC = i
			}
		}

		// C should be first, then B, then A
		assert.True(t, idxC < idxB, "C should start before B")
		assert.True(t, idxB < idxA, "B should start before A")
	})

	t.Run("start fails on component error", func(t *testing.T) {
		c := New()
		comp := newMockComponent("test")
		comp.SetStartError(errors.New("start failed"))

		err := c.Register("test", comp)
		require.NoError(t, err)

		err = c.StartAll()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "start failed")
		assert.False(t, c.IsStarted())
	})

	t.Run("start calls OnStart for LifecycleAware", func(t *testing.T) {
		c := New()
		comp := newMockLifecycleComponent("test")

		err := c.Register("test", comp)
		require.NoError(t, err)

		err = c.StartAll()
		require.NoError(t, err)

		assert.True(t, comp.onStartCalled)
	})

	t.Run("start validates ConfigurableComponents", func(t *testing.T) {
		c := New()
		comp := newMockConfigurableComponent("test")
		comp.validateErr = errors.New("validation failed")

		err := c.Register("test", comp)
		require.NoError(t, err)

		err = c.StartAll()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "validation failed")
	})

	t.Run("OnStart error stops startup", func(t *testing.T) {
		c := New()
		comp := newMockLifecycleComponent("test")
		comp.onStartErr = errors.New("onstart failed")

		err := c.Register("test", comp)
		require.NoError(t, err)

		err = c.StartAll()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "onstart failed")
	})
}

func TestStopAll(t *testing.T) {
	t.Run("stop all components", func(t *testing.T) {
		c := New()
		comp1 := newMockComponent("test1")
		comp2 := newMockComponent("test2")

		err := c.Register("test1", comp1)
		require.NoError(t, err)
		err = c.Register("test2", comp2)
		require.NoError(t, err)

		err = c.StartAll()
		require.NoError(t, err)

		err = c.StopAll()
		require.NoError(t, err)

		assert.False(t, comp1.IsRunning())
		assert.False(t, comp2.IsRunning())
		assert.False(t, c.IsStarted())
	})

	t.Run("stop is idempotent", func(t *testing.T) {
		c := New()
		comp := newMockComponent("test")

		err := c.Register("test", comp)
		require.NoError(t, err)

		err = c.StartAll()
		require.NoError(t, err)

		err = c.StopAll()
		require.NoError(t, err)

		err = c.StopAll()
		require.NoError(t, err)

		assert.Equal(t, 1, comp.GetStopCount())
	})

	t.Run("stop in reverse dependency order", func(t *testing.T) {
		c := New()
		// A depends on B, B depends on C
		// Stop order should be: A, B, C (reverse of start)
		var stopOrder []string
		var mu sync.Mutex

		callback := func(name string) {
			mu.Lock()
			stopOrder = append(stopOrder, name)
			mu.Unlock()
		}

		compA := newTrackableComponent("A", callback)
		compB := newTrackableComponent("B", callback)
		compC := newTrackableComponent("C", callback)

		err := c.Register("A", compA, "B")
		require.NoError(t, err)
		err = c.Register("B", compB, "C")
		require.NoError(t, err)
		err = c.Register("C", compC)
		require.NoError(t, err)

		err = c.StartAll()
		require.NoError(t, err)

		err = c.StopAll()
		require.NoError(t, err)

		// A should be stopped first, then B, then C
		require.Len(t, stopOrder, 3)
		assert.Equal(t, "A", stopOrder[0])
		assert.Equal(t, "B", stopOrder[1])
		assert.Equal(t, "C", stopOrder[2])
	})

	t.Run("stop continues on error", func(t *testing.T) {
		c := New()
		comp1 := newMockComponent("test1")
		comp2 := newMockComponent("test2")
		comp1.SetStopError(errors.New("stop failed"))

		err := c.Register("test1", comp1)
		require.NoError(t, err)
		err = c.Register("test2", comp2)
		require.NoError(t, err)

		err = c.StartAll()
		require.NoError(t, err)

		err = c.StopAll()
		require.Error(t, err)

		// Both should have been attempted to stop
		assert.False(t, c.IsStarted())
	})

	t.Run("stop calls OnStop for LifecycleAware", func(t *testing.T) {
		c := New()
		comp := newMockLifecycleComponent("test")

		err := c.Register("test", comp)
		require.NoError(t, err)

		err = c.StartAll()
		require.NoError(t, err)

		err = c.StopAll()
		require.NoError(t, err)

		assert.True(t, comp.onStopCalled)
	})
}

func TestCircularDependency(t *testing.T) {
	t.Run("detect simple cycle", func(t *testing.T) {
		c := New()
		compA := newMockComponent("A")
		compB := newMockComponent("B")

		// A depends on B, B depends on A
		err := c.Register("A", compA, "B")
		require.NoError(t, err)
		err = c.Register("B", compB, "A")
		require.NoError(t, err)

		err = c.StartAll()
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrCircularDependency)
	})

	t.Run("detect longer cycle", func(t *testing.T) {
		c := New()
		compA := newMockComponent("A")
		compB := newMockComponent("B")
		compC := newMockComponent("C")

		// A -> B -> C -> A
		err := c.Register("A", compA, "B")
		require.NoError(t, err)
		err = c.Register("B", compB, "C")
		require.NoError(t, err)
		err = c.Register("C", compC, "A")
		require.NoError(t, err)

		err = c.StartAll()
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrCircularDependency)
	})

	t.Run("self dependency", func(t *testing.T) {
		c := New()
		comp := newMockComponent("A")

		// A depends on itself
		err := c.Register("A", comp, "A")
		require.NoError(t, err)

		err = c.StartAll()
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrCircularDependency)
	})
}

func TestMissingDependency(t *testing.T) {
	c := New()
	comp := newMockComponent("A")

	// A depends on B, but B is not registered
	err := c.Register("A", comp, "B")
	require.NoError(t, err)

	err = c.StartAll()
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrDependencyNotFound)
}

func TestStartupOrder(t *testing.T) {
	t.Run("returns nil before start", func(t *testing.T) {
		c := New()
		comp := newMockComponent("test")

		err := c.Register("test", comp)
		require.NoError(t, err)

		order := c.StartupOrder()
		assert.Nil(t, order)
	})

	t.Run("returns copy after start", func(t *testing.T) {
		c := New()
		comp := newMockComponent("test")

		err := c.Register("test", comp)
		require.NoError(t, err)

		err = c.StartAll()
		require.NoError(t, err)

		order := c.StartupOrder()
		require.Len(t, order, 1)
		assert.Equal(t, "test", order[0])

		// Modifying returned slice shouldn't affect container
		order[0] = "modified"
		order2 := c.StartupOrder()
		assert.Equal(t, "test", order2[0])
	})
}

func TestComponentInfo(t *testing.T) {
	c := New()
	comp1 := newMockComponent("test1")
	comp2 := newMockComponent("test2")

	err := c.Register("test1", comp1)
	require.NoError(t, err)
	err = c.Register("test2", comp2, "test1")
	require.NoError(t, err)

	infos := c.ComponentInfo()
	assert.Len(t, infos, 2)

	// Find test2 info
	var test2Info *types.ComponentInfo
	for i := range infos {
		if infos[i].Name == "test2" {
			test2Info = &infos[i]
			break
		}
	}
	require.NotNil(t, test2Info)
	assert.Contains(t, test2Info.Dependencies, "test1")
}

func TestConcurrentAccess(t *testing.T) {
	c := New()

	// Register some components
	for i := 0; i < 10; i++ {
		comp := newMockComponent("test")
		name := "comp" + string(rune('0'+i))
		err := c.Register(name, comp)
		require.NoError(t, err)
	}

	// Concurrent reads
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = c.Names()
			_ = c.Count()
			_ = c.Has("comp0")
			_, _ = c.Get("comp0")
		}()
	}
	wg.Wait()
}

func TestComplexDependencyGraph(t *testing.T) {
	// Build a more complex dependency graph:
	//     A
	//    / \
	//   B   C
	//    \ / \
	//     D   E
	//      \ /
	//       F
	c := New()

	compA := newMockComponent("A")
	compB := newMockComponent("B")
	compC := newMockComponent("C")
	compD := newMockComponent("D")
	compE := newMockComponent("E")
	compF := newMockComponent("F")

	// Register in arbitrary order
	err := c.Register("D", compD, "B", "C")
	require.NoError(t, err)
	err = c.Register("A", compA)
	require.NoError(t, err)
	err = c.Register("F", compF, "D", "E")
	require.NoError(t, err)
	err = c.Register("B", compB, "A")
	require.NoError(t, err)
	err = c.Register("E", compE, "C")
	require.NoError(t, err)
	err = c.Register("C", compC, "A")
	require.NoError(t, err)

	err = c.StartAll()
	require.NoError(t, err)

	order := c.StartupOrder()
	require.Len(t, order, 6)

	// Build index map
	idx := make(map[string]int)
	for i, name := range order {
		idx[name] = i
	}

	// Verify dependencies come before dependents
	assert.True(t, idx["A"] < idx["B"], "A before B")
	assert.True(t, idx["A"] < idx["C"], "A before C")
	assert.True(t, idx["B"] < idx["D"], "B before D")
	assert.True(t, idx["C"] < idx["D"], "C before D")
	assert.True(t, idx["C"] < idx["E"], "C before E")
	assert.True(t, idx["D"] < idx["F"], "D before F")
	assert.True(t, idx["E"] < idx["F"], "E before F")
}
