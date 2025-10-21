package plugin

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// mockFactoryForTest implements Factory interface for testing
type mockFactoryForTest struct {
	setupError    error
	destroyError  error
	reloadError   error
	canDeleteFunc func(Plugin) bool
	setupDelay    time.Duration
	destroyDelay  time.Duration
	reloadDelay   time.Duration
	setupCount    int32
	destroyCount  int32
	reloadCount   int32
}

func (f *mockFactoryForTest) Setup(cfg any) (Plugin, error) {
	atomic.AddInt32(&f.setupCount, 1)
	if f.setupDelay > 0 {
		time.Sleep(f.setupDelay)
	}
	if f.setupError != nil {
		return nil, f.setupError
	}
	return &mockPlugin{
		factoryName: "mock_factory",
		config:      cfg.(map[string]any),
	}, nil
}

func (f *mockFactoryForTest) Destroy(p Plugin) error {
	atomic.AddInt32(&f.destroyCount, 1)
	if f.destroyDelay > 0 {
		time.Sleep(f.destroyDelay)
	}
	if f.destroyError != nil {
		return f.destroyError
	}
	if mp, ok := p.(*mockPlugin); ok {
		atomic.AddInt32(&mp.destroyCount, 1)
	}
	return nil
}

func (f *mockFactoryForTest) Reload(p Plugin, cfg any) error {
	atomic.AddInt32(&f.reloadCount, 1)
	if f.reloadDelay > 0 {
		time.Sleep(f.reloadDelay)
	}
	if f.reloadError != nil {
		return f.reloadError
	}
	if mp, ok := p.(*mockPlugin); ok {
		atomic.AddInt32(&mp.reloadCount, 1)
		mp.config = cfg.(map[string]any)
	}
	return nil
}

func (f *mockFactoryForTest) CanDelete(p Plugin) bool {
	if f.canDeleteFunc != nil {
		return f.canDeleteFunc(p)
	}
	// Default: check if plugin has active tasks
	if mp, ok := p.(*mockPlugin); ok {
		return atomic.LoadInt32(&mp.activeTaskCnt) == 0
	}
	return true
}

// TestFactory_Setup tests Factory.Setup method
func TestFactory_Setup(t *testing.T) {
	tests := []struct {
		name        string
		factory     *mockFactoryForTest
		config      map[string]any
		expectError bool
	}{
		{
			name:        "successful setup",
			factory:     &mockFactoryForTest{},
			config:      map[string]any{"host": "localhost", "port": 3306},
			expectError: false,
		},
		{
			name:        "setup with error",
			factory:     &mockFactoryForTest{setupError: errors.New("connection failed")},
			config:      map[string]any{"host": "invalid"},
			expectError: true,
		},
		{
			name:        "setup with delay (simulate slow initialization)",
			factory:     &mockFactoryForTest{setupDelay: 10 * time.Millisecond},
			config:      map[string]any{"host": "localhost"},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugin, err := tt.factory.Setup(tt.config)
			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				if plugin != nil {
					t.Error("Expected nil plugin on error")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
				if plugin == nil {
					t.Error("Expected non-nil plugin")
				}
				if atomic.LoadInt32(&tt.factory.setupCount) != 1 {
					t.Errorf("Expected setupCount=1, got %d", tt.factory.setupCount)
				}
			}
		})
	}
}

// TestFactory_Destroy tests Factory.Destroy method
func TestFactory_Destroy(t *testing.T) {
	tests := []struct {
		name        string
		factory     *mockFactoryForTest
		expectError bool
	}{
		{
			name:        "successful destroy",
			factory:     &mockFactoryForTest{},
			expectError: false,
		},
		{
			name:        "destroy with error",
			factory:     &mockFactoryForTest{destroyError: errors.New("cleanup failed")},
			expectError: true,
		},
		{
			name:        "destroy with delay (simulate slow cleanup)",
			factory:     &mockFactoryForTest{destroyDelay: 10 * time.Millisecond},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugin := &mockPlugin{factoryName: "test", config: map[string]any{}}
			err := tt.factory.Destroy(plugin)
			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
				if atomic.LoadInt32(&tt.factory.destroyCount) != 1 {
					t.Errorf("Expected destroyCount=1, got %d", tt.factory.destroyCount)
				}
				if atomic.LoadInt32(&plugin.destroyCount) != 1 {
					t.Errorf("Expected plugin.destroyCount=1, got %d", plugin.destroyCount)
				}
			}
		})
	}
}

// TestFactory_Reload tests Factory.Reload method
func TestFactory_Reload(t *testing.T) {
	tests := []struct {
		name        string
		factory     *mockFactoryForTest
		oldConfig   map[string]any
		newConfig   map[string]any
		expectError bool
	}{
		{
			name:        "successful reload",
			factory:     &mockFactoryForTest{},
			oldConfig:   map[string]any{"timeout": 30},
			newConfig:   map[string]any{"timeout": 60},
			expectError: false,
		},
		{
			name:        "reload with error",
			factory:     &mockFactoryForTest{reloadError: errors.New("reload failed")},
			oldConfig:   map[string]any{"timeout": 30},
			newConfig:   map[string]any{"timeout": 60},
			expectError: true,
		},
		{
			name:        "reload with delay (simulate slow reconfiguration)",
			factory:     &mockFactoryForTest{reloadDelay: 10 * time.Millisecond},
			oldConfig:   map[string]any{"pool_size": 10},
			newConfig:   map[string]any{"pool_size": 20},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugin := &mockPlugin{factoryName: "test", config: tt.oldConfig}
			err := tt.factory.Reload(plugin, tt.newConfig)
			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
				if atomic.LoadInt32(&tt.factory.reloadCount) != 1 {
					t.Errorf("Expected reloadCount=1, got %d", tt.factory.reloadCount)
				}
				if atomic.LoadInt32(&plugin.reloadCount) != 1 {
					t.Errorf("Expected plugin.reloadCount=1, got %d", plugin.reloadCount)
				}
				// Verify config was updated
				if plugin.config["timeout"] != tt.newConfig["timeout"] &&
					plugin.config["pool_size"] != tt.newConfig["pool_size"] {
					t.Error("Plugin config was not updated")
				}
			}
		})
	}
}

// TestFactory_CanDelete tests Factory.CanDelete method
func TestFactory_CanDelete(t *testing.T) {
	tests := []struct {
		name           string
		factory        *mockFactoryForTest
		activeTaskCnt  int32
		expectedResult bool
	}{
		{
			name:           "can delete - no active tasks",
			factory:        &mockFactoryForTest{},
			activeTaskCnt:  0,
			expectedResult: true,
		},
		{
			name:           "cannot delete - has active tasks",
			factory:        &mockFactoryForTest{},
			activeTaskCnt:  5,
			expectedResult: false,
		},
		{
			name: "custom canDelete logic - always allow",
			factory: &mockFactoryForTest{
				canDeleteFunc: func(p Plugin) bool { return true },
			},
			activeTaskCnt:  10,
			expectedResult: true,
		},
		{
			name: "custom canDelete logic - always deny",
			factory: &mockFactoryForTest{
				canDeleteFunc: func(p Plugin) bool { return false },
			},
			activeTaskCnt:  0,
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugin := &mockPlugin{
				factoryName:   "test",
				config:        map[string]any{},
				activeTaskCnt: tt.activeTaskCnt,
			}
			result := tt.factory.CanDelete(plugin)
			if result != tt.expectedResult {
				t.Errorf("Expected CanDelete=%v, got %v", tt.expectedResult, result)
			}
		})
	}
}

// TestFactory_Lifecycle tests complete plugin lifecycle
func TestFactory_Lifecycle(t *testing.T) {
	factory := &mockFactoryForTest{}

	// Step 1: Setup
	config := map[string]any{"host": "localhost", "port": 6379}
	plugin, err := factory.Setup(config)
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	if atomic.LoadInt32(&factory.setupCount) != 1 {
		t.Errorf("Expected setupCount=1, got %d", factory.setupCount)
	}

	// Step 2: Simulate active tasks
	mp := plugin.(*mockPlugin)
	atomic.StoreInt32(&mp.activeTaskCnt, 3)
	if factory.CanDelete(plugin) {
		t.Error("Should not be able to delete plugin with active tasks")
	}

	// Step 3: Reload configuration
	newConfig := map[string]any{"host": "localhost", "port": 6380}
	err = factory.Reload(plugin, newConfig)
	if err != nil {
		t.Fatalf("Reload failed: %v", err)
	}
	if atomic.LoadInt32(&factory.reloadCount) != 1 {
		t.Errorf("Expected reloadCount=1, got %d", factory.reloadCount)
	}

	// Step 4: Clear active tasks
	atomic.StoreInt32(&mp.activeTaskCnt, 0)
	if !factory.CanDelete(plugin) {
		t.Error("Should be able to delete plugin with no active tasks")
	}

	// Step 5: Destroy
	err = factory.Destroy(plugin)
	if err != nil {
		t.Fatalf("Destroy failed: %v", err)
	}
	if atomic.LoadInt32(&factory.destroyCount) != 1 {
		t.Errorf("Expected destroyCount=1, got %d", factory.destroyCount)
	}
}

// TestFactory_ConcurrentOperations tests concurrent factory operations
// Critical for Linux multi-core game servers with hot-reload
func TestFactory_ConcurrentOperations(t *testing.T) {
	factory := &mockFactoryForTest{}
	const numGoroutines = 100

	// Setup initial plugin
	config := map[string]any{"host": "localhost"}
	plugin, err := factory.Setup(config)
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	var wg sync.WaitGroup

	// Concurrent Reload operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			newConfig := map[string]any{"host": "localhost", "id": id}
			_ = factory.Reload(plugin, newConfig)
		}(i)
	}

	// Concurrent CanDelete checks
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = factory.CanDelete(plugin)
		}()
	}

	wg.Wait()

	// Verify reload was called multiple times
	reloadCount := atomic.LoadInt32(&factory.reloadCount)
	if reloadCount != numGoroutines {
		t.Errorf("Expected reloadCount=%d, got %d", numGoroutines, reloadCount)
	}

	// Final cleanup
	err = factory.Destroy(plugin)
	if err != nil {
		t.Fatalf("Destroy failed: %v", err)
	}
}

// BenchmarkFactory_Setup benchmarks plugin setup performance
func BenchmarkFactory_Setup(b *testing.B) {
	factory := &mockFactoryForTest{}
	config := map[string]any{"host": "localhost", "port": 3306}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = factory.Setup(config)
	}
}

// BenchmarkFactory_Reload benchmarks plugin reload performance
// Critical for hot-reload latency in production game servers
func BenchmarkFactory_Reload(b *testing.B) {
	factory := &mockFactoryForTest{}
	plugin := &mockPlugin{factoryName: "test", config: map[string]any{}}
	newConfig := map[string]any{"timeout": 60}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = factory.Reload(plugin, newConfig)
	}
}

// BenchmarkFactory_CanDelete benchmarks CanDelete check performance
func BenchmarkFactory_CanDelete(b *testing.B) {
	factory := &mockFactoryForTest{}
	plugin := &mockPlugin{factoryName: "test", config: map[string]any{}}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = factory.CanDelete(plugin)
	}
}

// BenchmarkFactory_Destroy benchmarks plugin destroy performance
func BenchmarkFactory_Destroy(b *testing.B) {
	factory := &mockFactoryForTest{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		plugin := &mockPlugin{factoryName: "test", config: map[string]any{}}
		b.StartTimer()
		_ = factory.Destroy(plugin)
	}
}
