package plugin

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

// integrationPlugin is a more complex plugin for integration testing
type integrationPlugin struct {
	name         string
	version      string
	dependencies []string
	initialized  bool
	started      bool
	stopped      bool
	startTime    time.Time
	stopTime     time.Time
	initError    error
	startError   error
	stopError    error
}

func newIntegrationPlugin(name, version string, dependencies []string) *integrationPlugin {
	return &integrationPlugin{
		name:         name,
		version:      version,
		dependencies: dependencies,
	}
}

func (p *integrationPlugin) Name() string {
	return p.name
}

func (p *integrationPlugin) Version() string {
	return p.version
}

func (p *integrationPlugin) Dependencies() []string {
	return p.dependencies
}

func (p *integrationPlugin) Init() error {
	if p.initError != nil {
		return p.initError
	}
	p.initialized = true
	return nil
}

func (p *integrationPlugin) Start() error {
	if p.startError != nil {
		return p.startError
	}
	p.started = true
	p.startTime = time.Now()
	return nil
}

func (p *integrationPlugin) Stop() error {
	if p.stopError != nil {
		return p.stopError
	}
	p.stopped = true
	p.stopTime = time.Now()
	return nil
}

func (p *integrationPlugin) IsInitialized() bool {
	return p.initialized
}

func (p *integrationPlugin) IsStarted() bool {
	return p.started
}

func (p *integrationPlugin) IsStopped() bool {
	return p.stopped
}

func (p *integrationPlugin) GetStartTime() time.Time {
	return p.startTime
}

func (p *integrationPlugin) GetStopTime() time.Time {
	return p.stopTime
}

func TestPluginSystem_CompleteLifecycle(t *testing.T) {
	pm := NewPluginManager()

	// Create plugins with dependencies
	configPlugin := newIntegrationPlugin("config", "1.0.0", []string{})
	databasePlugin := newIntegrationPlugin("database", "1.0.0", []string{"config"})
	servicePlugin := newIntegrationPlugin("service", "1.0.0", []string{"config", "database"})

	// Register plugins
	plugins := []*integrationPlugin{configPlugin, databasePlugin, servicePlugin}
	for _, plugin := range plugins {
		err := pm.RegisterPlugin(plugin)
		if err != nil {
			t.Fatalf("Failed to register plugin %s: %v", plugin.Name(), err)
		}
	}

	// Verify all plugins are registered
	registeredPlugins := pm.ListPlugins()
	if len(registeredPlugins) != 3 {
		t.Errorf("Expected 3 registered plugins, got %d", len(registeredPlugins))
	}

	// Start all plugins
	err := pm.StartAll()
	if err != nil {
		t.Fatalf("Failed to start all plugins: %v", err)
	}

	// Verify all plugins are started
	for _, plugin := range plugins {
		if !plugin.IsStarted() {
			t.Errorf("Expected plugin %s to be started", plugin.Name())
		}
		if !plugin.IsInitialized() {
			t.Errorf("Expected plugin %s to be initialized", plugin.Name())
		}
	}

	// Verify dependency order (config should start before database, database before service)
	if configPlugin.GetStartTime().After(databasePlugin.GetStartTime()) {
		t.Error("Expected config plugin to start before database plugin")
	}
	if databasePlugin.GetStartTime().After(servicePlugin.GetStartTime()) {
		t.Error("Expected database plugin to start before service plugin")
	}

	// Test individual plugin operations
	err = pm.StopPlugin("service")
	if err != nil {
		t.Errorf("Failed to stop service plugin: %v", err)
	}
	if !servicePlugin.IsStopped() {
		t.Error("Expected service plugin to be stopped")
	}

	// Restart service plugin
	err = pm.StartPlugin("service")
	if err != nil {
		t.Errorf("Failed to restart service plugin: %v", err)
	}
	if !servicePlugin.IsStarted() {
		t.Error("Expected service plugin to be restarted")
	}

	// Stop all plugins
	err = pm.StopAll()
	if err != nil {
		t.Fatalf("Failed to stop all plugins: %v", err)
	}

	// Verify all plugins are stopped
	for _, plugin := range plugins {
		if !plugin.IsStopped() {
			t.Errorf("Expected plugin %s to be stopped", plugin.Name())
		}
	}

	// Verify stop order (service should stop before database, database before config)
	if servicePlugin.GetStopTime().After(databasePlugin.GetStopTime()) {
		t.Error("Expected service plugin to stop before database plugin")
	}
	if databasePlugin.GetStopTime().After(configPlugin.GetStopTime()) {
		t.Error("Expected database plugin to stop before config plugin")
	}
}

func TestPluginSystem_ErrorHandling(t *testing.T) {
	// Test init error
	t.Run("InitError", func(t *testing.T) {
		pm := NewPluginManager()

		// Create plugin that will fail during init
		failingPlugin := newIntegrationPlugin("failing", "1.0.0", []string{})
		failingPlugin.initError = errors.New("init failed")

		// Register plugin
		err := pm.RegisterPlugin(failingPlugin)
		if err != nil {
			t.Fatalf("Failed to register failing plugin: %v", err)
		}

		// Try to start all plugins
		err = pm.StartAll()
		if err == nil {
			t.Error("Expected error when starting plugin with init failure, got nil")
		}

		// Verify plugin status is error
		info, err := pm.GetPluginInfo("failing")
		if err != nil {
			t.Fatalf("Failed to get plugin info: %v", err)
		}
		if info.Status != PluginStatusError {
			t.Errorf("Expected plugin status to be Error, got %v", info.Status)
		}
		if info.Error == nil {
			t.Error("Expected plugin error to be set")
		}
	})

	// Test start error
	t.Run("StartError", func(t *testing.T) {
		pm := NewPluginManager()

		// Create plugin that will fail during start
		startFailingPlugin := newIntegrationPlugin("start-failing", "1.0.0", []string{})
		startFailingPlugin.startError = errors.New("start failed")

		// Register plugin
		err := pm.RegisterPlugin(startFailingPlugin)
		if err != nil {
			t.Fatalf("Failed to register start-failing plugin: %v", err)
		}

		// Try to start all plugins
		err = pm.StartAll()
		if err == nil {
			t.Error("Expected error when starting plugin with start failure, got nil")
		}

		// Verify plugin status is error
		info, err := pm.GetPluginInfo("start-failing")
		if err != nil {
			t.Fatalf("Failed to get plugin info: %v", err)
		}
		if info.Status != PluginStatusError {
			t.Errorf("Expected plugin status to be Error, got %v", info.Status)
		}
	})
}

func TestPluginSystem_ComplexDependencies(t *testing.T) {
	pm := NewPluginManager()

	// Create complex dependency graph
	// A -> B, C
	// B -> D
	// C -> D
	// D -> E
	// E -> (no dependencies)
	pluginA := newIntegrationPlugin("A", "1.0.0", []string{"B", "C"})
	pluginB := newIntegrationPlugin("B", "1.0.0", []string{"D"})
	pluginC := newIntegrationPlugin("C", "1.0.0", []string{"D"})
	pluginD := newIntegrationPlugin("D", "1.0.0", []string{"E"})
	pluginE := newIntegrationPlugin("E", "1.0.0", []string{})

	plugins := []*integrationPlugin{pluginA, pluginB, pluginC, pluginD, pluginE}

	// Register all plugins
	for _, plugin := range plugins {
		err := pm.RegisterPlugin(plugin)
		if err != nil {
			t.Fatalf("Failed to register plugin %s: %v", plugin.Name(), err)
		}
	}

	// Start all plugins
	err := pm.StartAll()
	if err != nil {
		t.Fatalf("Failed to start all plugins: %v", err)
	}

	// Verify all plugins are started
	for _, plugin := range plugins {
		if !plugin.IsStarted() {
			t.Errorf("Expected plugin %s to be started", plugin.Name())
		}
	}

	// Verify dependency order
	// E should start first (no dependencies)
	// D should start second (depends on E)
	// B and C should start third (both depend on D)
	// A should start last (depends on B and C)
	if pluginE.GetStartTime().After(pluginD.GetStartTime()) {
		t.Error("Expected E to start before D")
	}
	if pluginD.GetStartTime().After(pluginB.GetStartTime()) {
		t.Error("Expected D to start before B")
	}
	if pluginD.GetStartTime().After(pluginC.GetStartTime()) {
		t.Error("Expected D to start before C")
	}
	if pluginB.GetStartTime().After(pluginA.GetStartTime()) {
		t.Error("Expected B to start before A")
	}
	if pluginC.GetStartTime().After(pluginA.GetStartTime()) {
		t.Error("Expected C to start before A")
	}
}

func TestPluginSystem_ConcurrentOperations(t *testing.T) {
	pm := NewPluginManager()
	done := make(chan bool, 4)

	// Goroutine 1: Register plugins
	go func() {
		for i := 0; i < 5; i++ {
			plugin := newIntegrationPlugin(fmt.Sprintf("plugin-%d", i), "1.0.0", []string{})
			pm.RegisterPlugin(plugin)
			time.Sleep(time.Millisecond * 10)
		}
		done <- true
	}()

	// Goroutine 2: Start plugins
	go func() {
		time.Sleep(time.Millisecond * 5)
		for i := 0; i < 5; i++ {
			pm.StartPlugin(fmt.Sprintf("plugin-%d", i))
			time.Sleep(time.Millisecond * 10)
		}
		done <- true
	}()

	// Goroutine 3: Get plugin info
	go func() {
		time.Sleep(time.Millisecond * 10)
		for i := 0; i < 5; i++ {
			pm.GetPluginInfo(fmt.Sprintf("plugin-%d", i))
			time.Sleep(time.Millisecond * 10)
		}
		done <- true
	}()

	// Goroutine 4: List plugins
	go func() {
		time.Sleep(time.Millisecond * 15)
		for i := 0; i < 5; i++ {
			pm.ListPlugins()
			time.Sleep(time.Millisecond * 10)
		}
		done <- true
	}()

	// Wait for all goroutines to complete
	for i := 0; i < 4; i++ {
		<-done
	}

	// Verify some plugins were registered
	plugins := pm.ListPlugins()
	if len(plugins) == 0 {
		t.Error("Expected some plugins to be registered")
	}
}

func TestPluginSystem_UnregisterAndReregister(t *testing.T) {
	pm := NewPluginManager()

	// Create and register plugin
	plugin1 := newIntegrationPlugin("test", "1.0.0", []string{})
	err := pm.RegisterPlugin(plugin1)
	if err != nil {
		t.Fatalf("Failed to register plugin: %v", err)
	}

	// Start plugin
	err = pm.StartPlugin("test")
	if err != nil {
		t.Fatalf("Failed to start plugin: %v", err)
	}

	// Unregister plugin
	err = pm.UnregisterPlugin("test")
	if err != nil {
		t.Errorf("Failed to unregister plugin: %v", err)
	}

	// Verify plugin is unregistered
	registeredPlugin := pm.GetPlugin("test")
	if registeredPlugin != nil {
		t.Error("Expected plugin to be unregistered")
	}

	// Create and register new plugin with same name
	plugin2 := newIntegrationPlugin("test", "2.0.0", []string{})
	err = pm.RegisterPlugin(plugin2)
	if err != nil {
		t.Fatalf("Failed to register new plugin: %v", err)
	}

	// Verify new plugin is registered
	registeredPlugin = pm.GetPlugin("test")
	if registeredPlugin != plugin2 {
		t.Error("Expected new plugin to be registered")
	}

	// Verify plugin info is updated
	info, err := pm.GetPluginInfo("test")
	if err != nil {
		t.Fatalf("Failed to get plugin info: %v", err)
	}
	if info.Version != "2.0.0" {
		t.Errorf("Expected version 2.0.0, got %s", info.Version)
	}
}
