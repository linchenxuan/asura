package plugin

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
)

// TestRegisterPluginIns_FirstRegistration tests first time plugin registration
func TestRegisterPluginIns_FirstRegistration(t *testing.T) {
	// Reset plugin manager state
	_pluginLock.Lock()
	_pluginMgr.insMap = make(map[string]map[string]map[string]Plugin)
	_pluginLock.Unlock()

	plugin := &mockPlugin{
		factoryName: "test_factory",
		config:      map[string]any{"name": "test"},
	}

	err := registerPluginIns("database", "mysql", "default", plugin)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify plugin is registered
	_pluginLock.RLock()
	defer _pluginLock.RUnlock()

	if _, ok := _pluginMgr.insMap["database"]; !ok {
		t.Fatal("Factory type 'database' not registered")
	}
	if _, ok := _pluginMgr.insMap["database"]["mysql"]; !ok {
		t.Fatal("Factory name 'mysql' not registered")
	}
	if _, ok := _pluginMgr.insMap["database"]["mysql"]["default"]; !ok {
		t.Fatal("Plugin instance 'default' not registered")
	}
}

// TestRegisterPluginIns_DuplicateInstance tests duplicate plugin registration
func TestRegisterPluginIns_DuplicateInstance(t *testing.T) {
	// Reset plugin manager state
	_pluginLock.Lock()
	_pluginMgr.insMap = make(map[string]map[string]map[string]Plugin)
	_pluginLock.Unlock()

	plugin1 := &mockPlugin{factoryName: "test_factory", config: map[string]any{"name": "test1"}}
	plugin2 := &mockPlugin{factoryName: "test_factory", config: map[string]any{"name": "test2"}}

	// First registration should succeed
	err := registerPluginIns("cache", "redis", "instance1", plugin1)
	if err != nil {
		t.Fatalf("First registration failed: %v", err)
	}

	// Duplicate registration should fail
	err = registerPluginIns("cache", "redis", "instance1", plugin2)
	if err == nil {
		t.Fatal("Expected error for duplicate registration, got nil")
	}
	expectedErr := "plugin instance1 already exists"
	if err.Error() != expectedErr {
		t.Errorf("Expected error '%s', got '%s'", expectedErr, err.Error())
	}
}

// TestRegisterPluginIns_MultipleFactoryTypes tests multiple factory types
func TestRegisterPluginIns_MultipleFactoryTypes(t *testing.T) {
	// Reset plugin manager state
	_pluginLock.Lock()
	_pluginMgr.insMap = make(map[string]map[string]map[string]Plugin)
	_pluginLock.Unlock()

	testCases := []struct {
		factoryType  string
		factoryName  string
		instanceName string
	}{
		{"database", "mysql", "master"},
		{"database", "mysql", "slave"},
		{"database", "postgres", "default"},
		{"cache", "redis", "session"},
		{"cache", "memcached", "default"},
		{"mq", "kafka", "producer"},
	}

	for _, tc := range testCases {
		plugin := &mockPlugin{
			factoryName: tc.factoryName,
			config:      map[string]any{"name": tc.instanceName},
		}
		err := registerPluginIns(tc.factoryType, tc.factoryName, tc.instanceName, plugin)
		if err != nil {
			t.Fatalf("Failed to register %s/%s/%s: %v", tc.factoryType, tc.factoryName, tc.instanceName, err)
		}
	}

	// Verify all registrations
	_pluginLock.RLock()
	defer _pluginLock.RUnlock()

	if len(_pluginMgr.insMap) != 3 {
		t.Errorf("Expected 3 factory types, got %d", len(_pluginMgr.insMap))
	}
	if len(_pluginMgr.insMap["database"]) != 2 {
		t.Errorf("Expected 2 database factories, got %d", len(_pluginMgr.insMap["database"]))
	}
	if len(_pluginMgr.insMap["database"]["mysql"]) != 2 {
		t.Errorf("Expected 2 mysql instances, got %d", len(_pluginMgr.insMap["database"]["mysql"]))
	}
}

// TestRegisterPluginIns_ConcurrentRegistration tests concurrent plugin registration
// This is critical for Linux multi-core environments where multiple goroutines
// may register plugins simultaneously during server startup
func TestRegisterPluginIns_ConcurrentRegistration(t *testing.T) {
	// Reset plugin manager state
	_pluginLock.Lock()
	_pluginMgr.insMap = make(map[string]map[string]map[string]Plugin)
	_pluginLock.Unlock()

	const numGoroutines = 100
	const numPluginsPerGoroutine = 10

	var wg sync.WaitGroup
	var successCount int32
	var errorCount int32

	// Launch concurrent registrations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < numPluginsPerGoroutine; j++ {
				plugin := &mockPlugin{
					factoryName: fmt.Sprintf("factory_%d", goroutineID),
					config:      map[string]any{"id": goroutineID*numPluginsPerGoroutine + j},
				}
				instanceName := fmt.Sprintf("instance_%d_%d", goroutineID, j)
				err := registerPluginIns("concurrent_test", "test_factory", instanceName, plugin)
				if err != nil {
					atomic.AddInt32(&errorCount, 1)
					t.Logf("Registration failed: %v", err)
				} else {
					atomic.AddInt32(&successCount, 1)
				}
			}
		}(i)
	}

	wg.Wait()

	expectedSuccess := int32(numGoroutines * numPluginsPerGoroutine)
	if successCount != expectedSuccess {
		t.Errorf("Expected %d successful registrations, got %d (errors: %d)",
			expectedSuccess, successCount, errorCount)
	}

	// Verify all plugins are registered
	_pluginLock.RLock()
	defer _pluginLock.RUnlock()

	totalRegistered := len(_pluginMgr.insMap["concurrent_test"]["test_factory"])
	if totalRegistered != int(expectedSuccess) {
		t.Errorf("Expected %d registered plugins, got %d", expectedSuccess, totalRegistered)
	}
}

// TestRegisterPluginIns_ConcurrentDuplicateRegistration tests concurrent duplicate registration
// This simulates race conditions in Linux multi-core environments
func TestRegisterPluginIns_ConcurrentDuplicateRegistration(t *testing.T) {
	// Reset plugin manager state
	_pluginLock.Lock()
	_pluginMgr.insMap = make(map[string]map[string]map[string]Plugin)
	_pluginLock.Unlock()

	const numGoroutines = 50
	const instanceName = "duplicate_instance"

	var wg sync.WaitGroup
	var successCount int32
	var errorCount int32

	// All goroutines try to register the same instance
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			plugin := &mockPlugin{
				factoryName: "test_factory",
				config:      map[string]any{"id": id},
			}
			err := registerPluginIns("race_test", "test_factory", instanceName, plugin)
			if err != nil {
				atomic.AddInt32(&errorCount, 1)
			} else {
				atomic.AddInt32(&successCount, 1)
			}
		}(i)
	}

	wg.Wait()

	// Only one should succeed, others should fail
	if successCount != 1 {
		t.Errorf("Expected exactly 1 successful registration, got %d", successCount)
	}
	if errorCount != int32(numGoroutines-1) {
		t.Errorf("Expected %d errors, got %d", numGoroutines-1, errorCount)
	}

	// Verify only one instance is registered
	_pluginLock.RLock()
	defer _pluginLock.RUnlock()

	totalRegistered := len(_pluginMgr.insMap["race_test"]["test_factory"])
	if totalRegistered != 1 {
		t.Errorf("Expected 1 registered plugin, got %d", totalRegistered)
	}
}

// TestRegisterPluginIns_NilPlugin tests registering nil plugin
func TestRegisterPluginIns_NilPlugin(t *testing.T) {
	// Reset plugin manager state
	_pluginLock.Lock()
	_pluginMgr.insMap = make(map[string]map[string]map[string]Plugin)
	_pluginLock.Unlock()

	// Registering nil plugin should succeed (no validation in registerPluginIns)
	err := registerPluginIns("test", "test", "nil_instance", nil)
	if err != nil {
		t.Fatalf("Expected no error for nil plugin, got: %v", err)
	}

	// Verify nil plugin is registered
	_pluginLock.RLock()
	defer _pluginLock.RUnlock()

	plugin := _pluginMgr.insMap["test"]["test"]["nil_instance"]
	if plugin != nil {
		t.Error("Expected nil plugin, got non-nil")
	}
}

// BenchmarkRegisterPluginIns benchmarks plugin registration performance
// Critical for Linux game server startup time optimization
func BenchmarkRegisterPluginIns(b *testing.B) {
	// Reset plugin manager state
	_pluginLock.Lock()
	_pluginMgr.insMap = make(map[string]map[string]map[string]Plugin)
	_pluginLock.Unlock()

	plugin := &mockPlugin{
		factoryName: "bench_factory",
		config:      map[string]any{"name": "bench"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		instanceName := fmt.Sprintf("instance_%d", i)
		_ = registerPluginIns("benchmark", "test", instanceName, plugin)
	}
}

// BenchmarkRegisterPluginIns_Parallel benchmarks parallel plugin registration
// Simulates Linux multi-core server startup scenario
func BenchmarkRegisterPluginIns_Parallel(b *testing.B) {
	// Reset plugin manager state
	_pluginLock.Lock()
	_pluginMgr.insMap = make(map[string]map[string]map[string]Plugin)
	_pluginLock.Unlock()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			plugin := &mockPlugin{
				factoryName: "parallel_factory",
				config:      map[string]any{"id": i},
			}
			instanceName := fmt.Sprintf("parallel_instance_%d", i)
			_ = registerPluginIns("parallel_bench", "test", instanceName, plugin)
			i++
		}
	})
}
