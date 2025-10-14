package plugin

import (
	"errors"
	"fmt"
	"testing"
)

func TestNewPluginManager(t *testing.T) {
	pm := NewPluginManager()
	if pm == nil {
		t.Fatal("Expected plugin manager to be created, got nil")
	}

	// Test that no plugins are initially registered
	plugins := pm.ListPlugins()
	if len(plugins) != 0 {
		t.Errorf("Expected no plugins initially, got %d", len(plugins))
	}
}

func TestPluginManager_RegisterPlugin(t *testing.T) {
	pm := NewPluginManager()
	plugin := newMockPlugin("test-plugin", "1.0.0", []string{})

	// Test successful registration
	err := pm.RegisterPlugin(plugin)
	if err != nil {
		t.Errorf("Expected no error during registration, got %v", err)
	}

	// Verify plugin is registered
	registeredPlugin := pm.GetPlugin("test-plugin")
	if registeredPlugin != plugin {
		t.Error("Expected registered plugin to be returned")
	}

	// Test duplicate registration
	err = pm.RegisterPlugin(plugin)
	if err == nil {
		t.Error("Expected error for duplicate registration, got nil")
	}

	// Test nil plugin
	err = pm.RegisterPlugin(nil)
	if err == nil {
		t.Error("Expected error for nil plugin, got nil")
	}

	// Test plugin with empty name
	emptyNamePlugin := newMockPlugin("", "1.0.0", []string{})
	err = pm.RegisterPlugin(emptyNamePlugin)
	if err == nil {
		t.Error("Expected error for empty name, got nil")
	}
}

func TestPluginManager_UnregisterPlugin(t *testing.T) {
	pm := NewPluginManager()
	plugin := newMockPlugin("test-plugin", "1.0.0", []string{})

	// Register plugin
	err := pm.RegisterPlugin(plugin)
	if err != nil {
		t.Fatalf("Failed to register plugin: %v", err)
	}

	// Test successful unregistration
	err = pm.UnregisterPlugin("test-plugin")
	if err != nil {
		t.Errorf("Expected no error during unregistration, got %v", err)
	}

	// Verify plugin is unregistered
	registeredPlugin := pm.GetPlugin("test-plugin")
	if registeredPlugin != nil {
		t.Error("Expected plugin to be unregistered")
	}

	// Test unregistering non-existent plugin
	err = pm.UnregisterPlugin("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent plugin, got nil")
	}
}

func TestPluginManager_StartAll(t *testing.T) {
	pm := NewPluginManager()

	// Test starting with no plugins
	err := pm.StartAll()
	if err != nil {
		t.Errorf("Expected no error when starting no plugins, got %v", err)
	}

	// Register plugins with dependencies
	plugin1 := newMockPlugin("plugin1", "1.0.0", []string{})
	plugin2 := newMockPlugin("plugin2", "1.0.0", []string{"plugin1"})
	plugin3 := newMockPlugin("plugin3", "1.0.0", []string{"plugin1", "plugin2"})

	err = pm.RegisterPlugin(plugin1)
	if err != nil {
		t.Fatalf("Failed to register plugin1: %v", err)
	}

	err = pm.RegisterPlugin(plugin2)
	if err != nil {
		t.Fatalf("Failed to register plugin2: %v", err)
	}

	err = pm.RegisterPlugin(plugin3)
	if err != nil {
		t.Fatalf("Failed to register plugin3: %v", err)
	}

	// Test successful start all
	err = pm.StartAll()
	if err != nil {
		t.Errorf("Expected no error during start all, got %v", err)
	}

	// Verify all plugins are started
	if !plugin1.IsStarted() {
		t.Error("Expected plugin1 to be started")
	}
	if !plugin2.IsStarted() {
		t.Error("Expected plugin2 to be started")
	}
	if !plugin3.IsStarted() {
		t.Error("Expected plugin3 to be started")
	}

	// Verify plugin info status
	info1, err := pm.GetPluginInfo("plugin1")
	if err != nil {
		t.Fatalf("Failed to get plugin1 info: %v", err)
	}
	if info1.Status != PluginStatusStarted {
		t.Errorf("Expected plugin1 status to be Started, got %v", info1.Status)
	}
}

func TestPluginManager_StartAll_WithInitError(t *testing.T) {
	pm := NewPluginManager()
	plugin := newMockPlugin("test-plugin", "1.0.0", []string{})
	plugin.initError = errors.New("init error")

	err := pm.RegisterPlugin(plugin)
	if err != nil {
		t.Fatalf("Failed to register plugin: %v", err)
	}

	// Test start all with init error
	err = pm.StartAll()
	if err == nil {
		t.Error("Expected error during start all with init error, got nil")
	}

	// Verify plugin error status
	info, err := pm.GetPluginInfo("test-plugin")
	if err != nil {
		t.Fatalf("Failed to get plugin info: %v", err)
	}
	if info.Status != PluginStatusError {
		t.Errorf("Expected plugin status to be Error, got %v", info.Status)
	}
	if info.Error == nil {
		t.Error("Expected plugin error to be set")
	}
}

func TestPluginManager_StartAll_WithStartError(t *testing.T) {
	pm := NewPluginManager()
	plugin := newMockPlugin("test-plugin", "1.0.0", []string{})
	plugin.startError = errors.New("start error")

	err := pm.RegisterPlugin(plugin)
	if err != nil {
		t.Fatalf("Failed to register plugin: %v", err)
	}

	// Test start all with start error
	err = pm.StartAll()
	if err == nil {
		t.Error("Expected error during start all with start error, got nil")
	}

	// Verify plugin error status
	info, err := pm.GetPluginInfo("test-plugin")
	if err != nil {
		t.Fatalf("Failed to get plugin info: %v", err)
	}
	if info.Status != PluginStatusError {
		t.Errorf("Expected plugin status to be Error, got %v", info.Status)
	}
}

func TestPluginManager_StopAll(t *testing.T) {
	pm := NewPluginManager()

	// Test stopping with no plugins
	err := pm.StopAll()
	if err != nil {
		t.Errorf("Expected no error when stopping no plugins, got %v", err)
	}

	// Register and start plugins
	plugin1 := newMockPlugin("plugin1", "1.0.0", []string{})
	plugin2 := newMockPlugin("plugin2", "1.0.0", []string{"plugin1"})

	err = pm.RegisterPlugin(plugin1)
	if err != nil {
		t.Fatalf("Failed to register plugin1: %v", err)
	}

	err = pm.RegisterPlugin(plugin2)
	if err != nil {
		t.Fatalf("Failed to register plugin2: %v", err)
	}

	// Start all plugins
	err = pm.StartAll()
	if err != nil {
		t.Fatalf("Failed to start all plugins: %v", err)
	}

	// Test successful stop all
	err = pm.StopAll()
	if err != nil {
		t.Errorf("Expected no error during stop all, got %v", err)
	}

	// Verify all plugins are stopped
	if !plugin1.IsStopped() {
		t.Error("Expected plugin1 to be stopped")
	}
	if !plugin2.IsStopped() {
		t.Error("Expected plugin2 to be stopped")
	}

	// Verify plugin info status
	info1, err := pm.GetPluginInfo("plugin1")
	if err != nil {
		t.Fatalf("Failed to get plugin1 info: %v", err)
	}
	if info1.Status != PluginStatusStopped {
		t.Errorf("Expected plugin1 status to be Stopped, got %v", info1.Status)
	}
}

func TestPluginManager_StartPlugin(t *testing.T) {
	pm := NewPluginManager()
	plugin := newMockPlugin("test-plugin", "1.0.0", []string{})

	err := pm.RegisterPlugin(plugin)
	if err != nil {
		t.Fatalf("Failed to register plugin: %v", err)
	}

	// Test successful start
	err = pm.StartPlugin("test-plugin")
	if err != nil {
		t.Errorf("Expected no error during start, got %v", err)
	}

	// Verify plugin is started
	if !plugin.IsStarted() {
		t.Error("Expected plugin to be started")
	}

	// Test starting already started plugin
	err = pm.StartPlugin("test-plugin")
	if err == nil {
		t.Error("Expected error for already started plugin, got nil")
	}

	// Test starting non-existent plugin
	err = pm.StartPlugin("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent plugin, got nil")
	}
}

func TestPluginManager_StopPlugin(t *testing.T) {
	pm := NewPluginManager()
	plugin := newMockPlugin("test-plugin", "1.0.0", []string{})

	err := pm.RegisterPlugin(plugin)
	if err != nil {
		t.Fatalf("Failed to register plugin: %v", err)
	}

	// Start plugin first
	err = pm.StartPlugin("test-plugin")
	if err != nil {
		t.Fatalf("Failed to start plugin: %v", err)
	}

	// Test successful stop
	err = pm.StopPlugin("test-plugin")
	if err != nil {
		t.Errorf("Expected no error during stop, got %v", err)
	}

	// Verify plugin is stopped
	if !plugin.IsStopped() {
		t.Error("Expected plugin to be stopped")
	}

	// Test stopping already stopped plugin
	err = pm.StopPlugin("test-plugin")
	if err == nil {
		t.Error("Expected error for already stopped plugin, got nil")
	}

	// Test stopping non-existent plugin
	err = pm.StopPlugin("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent plugin, got nil")
	}
}

func TestPluginManager_GetPluginInfo(t *testing.T) {
	pm := NewPluginManager()
	plugin := newMockPlugin("test-plugin", "1.0.0", []string{"dep1", "dep2"})

	err := pm.RegisterPlugin(plugin)
	if err != nil {
		t.Fatalf("Failed to register plugin: %v", err)
	}

	// Test getting plugin info
	info, err := pm.GetPluginInfo("test-plugin")
	if err != nil {
		t.Errorf("Expected no error getting plugin info, got %v", err)
	}

	if info.Name != "test-plugin" {
		t.Errorf("Expected name 'test-plugin', got '%s'", info.Name)
	}
	if info.Version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got '%s'", info.Version)
	}
	if info.Status != PluginStatusRegistered {
		t.Errorf("Expected status Registered, got %v", info.Status)
	}
	if len(info.Dependencies) != 2 {
		t.Errorf("Expected 2 dependencies, got %d", len(info.Dependencies))
	}

	// Test getting non-existent plugin info
	_, err = pm.GetPluginInfo("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent plugin, got nil")
	}
}

func TestPluginManager_ListPlugins(t *testing.T) {
	pm := NewPluginManager()

	// Test empty list
	plugins := pm.ListPlugins()
	if len(plugins) != 0 {
		t.Errorf("Expected empty list, got %d plugins", len(plugins))
	}

	// Register multiple plugins
	plugin1 := newMockPlugin("plugin1", "1.0.0", []string{})
	plugin2 := newMockPlugin("plugin2", "2.0.0", []string{"plugin1"})

	err := pm.RegisterPlugin(plugin1)
	if err != nil {
		t.Fatalf("Failed to register plugin1: %v", err)
	}

	err = pm.RegisterPlugin(plugin2)
	if err != nil {
		t.Fatalf("Failed to register plugin2: %v", err)
	}

	// Test list with plugins
	plugins = pm.ListPlugins()
	if len(plugins) != 2 {
		t.Errorf("Expected 2 plugins, got %d", len(plugins))
	}

	// Verify plugin names are in the list
	names := make(map[string]bool)
	for _, plugin := range plugins {
		names[plugin.Name] = true
	}

	if !names["plugin1"] || !names["plugin2"] {
		t.Errorf("Expected both plugins in list, got %v", names)
	}
}

func TestPluginManager_CircularDependency(t *testing.T) {
	pm := NewPluginManager()

	// Create plugins with circular dependency
	plugin1 := newMockPlugin("plugin1", "1.0.0", []string{"plugin2"})
	plugin2 := newMockPlugin("plugin2", "1.0.0", []string{"plugin1"})

	err := pm.RegisterPlugin(plugin1)
	if err != nil {
		t.Fatalf("Failed to register plugin1: %v", err)
	}

	err = pm.RegisterPlugin(plugin2)
	if err != nil {
		t.Fatalf("Failed to register plugin2: %v", err)
	}

	// Test start all with circular dependency
	err = pm.StartAll()
	if err == nil {
		t.Error("Expected error for circular dependency, got nil")
	}
}

func TestPluginManager_MissingDependency(t *testing.T) {
	pm := NewPluginManager()

	// Create plugin with missing dependency
	plugin := newMockPlugin("plugin", "1.0.0", []string{"missing-plugin"})

	err := pm.RegisterPlugin(plugin)
	if err != nil {
		t.Fatalf("Failed to register plugin: %v", err)
	}

	// Test start all with missing dependency
	err = pm.StartAll()
	if err == nil {
		t.Error("Expected error for missing dependency, got nil")
	}
}

func TestPluginManager_ConcurrentAccess(t *testing.T) {
	pm := NewPluginManager()
	done := make(chan bool, 3)

	// Goroutine 1: Register plugins
	go func() {
		for i := 0; i < 10; i++ {
			plugin := newMockPlugin(fmt.Sprintf("plugin-%d", i), "1.0.0", []string{})
			pm.RegisterPlugin(plugin)
		}
		done <- true
	}()

	// Goroutine 2: Get plugins
	go func() {
		for i := 0; i < 10; i++ {
			pm.GetPlugin(fmt.Sprintf("plugin-%d", i))
		}
		done <- true
	}()

	// Goroutine 3: List plugins
	go func() {
		for i := 0; i < 10; i++ {
			pm.ListPlugins()
		}
		done <- true
	}()

	// Wait for all goroutines to complete
	for i := 0; i < 3; i++ {
		<-done
	}

	// Verify some plugins were registered
	plugins := pm.ListPlugins()
	if len(plugins) == 0 {
		t.Error("Expected some plugins to be registered")
	}
}

func TestPluginManager_UnregisterStartedPlugin(t *testing.T) {
	pm := NewPluginManager()
	plugin := newMockPlugin("test-plugin", "1.0.0", []string{})

	// Register and start plugin
	err := pm.RegisterPlugin(plugin)
	if err != nil {
		t.Fatalf("Failed to register plugin: %v", err)
	}

	err = pm.StartPlugin("test-plugin")
	if err != nil {
		t.Fatalf("Failed to start plugin: %v", err)
	}

	// Test unregistering started plugin
	err = pm.UnregisterPlugin("test-plugin")
	if err != nil {
		t.Errorf("Expected no error unregistering started plugin, got %v", err)
	}

	// Verify plugin is stopped and unregistered
	if !plugin.IsStopped() {
		t.Error("Expected plugin to be stopped during unregister")
	}

	registeredPlugin := pm.GetPlugin("test-plugin")
	if registeredPlugin != nil {
		t.Error("Expected plugin to be unregistered")
	}
}
