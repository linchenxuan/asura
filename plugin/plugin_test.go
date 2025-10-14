package plugin

import (
	"errors"
	"testing"
	"time"
)

// mockPlugin is a test implementation of Plugin interface
type mockPlugin struct {
	name         string
	version      string
	dependencies []string
	initError    error
	startError   error
	stopError    error
	initialized  bool
	started      bool
	stopped      bool
}

func newMockPlugin(name, version string, dependencies []string) *mockPlugin {
	return &mockPlugin{
		name:         name,
		version:      version,
		dependencies: dependencies,
	}
}

func (p *mockPlugin) Name() string {
	return p.name
}

func (p *mockPlugin) Version() string {
	return p.version
}

func (p *mockPlugin) Dependencies() []string {
	return p.dependencies
}

func (p *mockPlugin) Init() error {
	if p.initError != nil {
		return p.initError
	}
	p.initialized = true
	return nil
}

func (p *mockPlugin) Start() error {
	if p.startError != nil {
		return p.startError
	}
	p.started = true
	return nil
}

func (p *mockPlugin) Stop() error {
	if p.stopError != nil {
		return p.stopError
	}
	p.stopped = true
	return nil
}

func (p *mockPlugin) IsInitialized() bool {
	return p.initialized
}

func (p *mockPlugin) IsStarted() bool {
	return p.started
}

func (p *mockPlugin) IsStopped() bool {
	return p.stopped
}

func TestPluginStatus_String(t *testing.T) {
	tests := []struct {
		status PluginStatus
		want   string
	}{
		{PluginStatusUnknown, "unknown"},
		{PluginStatusRegistered, "registered"},
		{PluginStatusInitialized, "initialized"},
		{PluginStatusStarted, "started"},
		{PluginStatusStopped, "stopped"},
		{PluginStatusError, "error"},
		{PluginStatus(999), "unknown"}, // invalid status
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.status.String(); got != tt.want {
				t.Errorf("PluginStatus.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPluginInfo(t *testing.T) {
	now := time.Now()
	info := PluginInfo{
		Name:         "test-plugin",
		Version:      "1.0.0",
		Status:       PluginStatusStarted,
		Dependencies: []string{"dep1", "dep2"},
		StartTime:    now,
		StopTime:     now.Add(time.Hour),
		Error:        nil,
	}

	if info.Name != "test-plugin" {
		t.Errorf("Expected name 'test-plugin', got %s", info.Name)
	}
	if info.Version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got %s", info.Version)
	}
	if info.Status != PluginStatusStarted {
		t.Errorf("Expected status PluginStatusStarted, got %v", info.Status)
	}
	if len(info.Dependencies) != 2 {
		t.Errorf("Expected 2 dependencies, got %d", len(info.Dependencies))
	}
}

func TestPluginError(t *testing.T) {
	originalErr := errors.New("original error")
	pluginErr := NewPluginError("test-plugin", "start", originalErr)

	// Test Error() method
	expectedMsg := "plugin test-plugin start failed: original error"
	if pluginErr.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, pluginErr.Error())
	}

	// Test Unwrap() method
	if pluginErr.Unwrap() != originalErr {
		t.Errorf("Expected unwrapped error to be original error")
	}

	// Test fields
	if pluginErr.PluginName != "test-plugin" {
		t.Errorf("Expected PluginName 'test-plugin', got '%s'", pluginErr.PluginName)
	}
	if pluginErr.Operation != "start" {
		t.Errorf("Expected Operation 'start', got '%s'", pluginErr.Operation)
	}
}

func TestPluginContext(t *testing.T) {
	ctx := PluginContext{
		Context: nil, // can be nil for testing
	}

	if ctx.Context != nil {
		t.Errorf("Expected context to be nil for testing")
	}
}

func TestMockPlugin(t *testing.T) {
	plugin := newMockPlugin("test", "1.0.0", []string{"dep1"})

	// Test basic properties
	if plugin.Name() != "test" {
		t.Errorf("Expected name 'test', got '%s'", plugin.Name())
	}
	if plugin.Version() != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got '%s'", plugin.Version())
	}
	if len(plugin.Dependencies()) != 1 || plugin.Dependencies()[0] != "dep1" {
		t.Errorf("Expected dependencies ['dep1'], got %v", plugin.Dependencies())
	}

	// Test lifecycle
	// Test Init
	if err := plugin.Init(); err != nil {
		t.Errorf("Expected no error during Init, got %v", err)
	}
	if !plugin.IsInitialized() {
		t.Errorf("Expected plugin to be initialized")
	}

	// Test Start
	if err := plugin.Start(); err != nil {
		t.Errorf("Expected no error during Start, got %v", err)
	}
	if !plugin.IsStarted() {
		t.Errorf("Expected plugin to be started")
	}

	// Test Stop
	if err := plugin.Stop(); err != nil {
		t.Errorf("Expected no error during Stop, got %v", err)
	}
	if !plugin.IsStopped() {
		t.Errorf("Expected plugin to be stopped")
	}
}

func TestMockPlugin_WithErrors(t *testing.T) {
	initErr := errors.New("init error")
	startErr := errors.New("start error")
	stopErr := errors.New("stop error")

	plugin := newMockPlugin("test", "1.0.0", nil)
	plugin.initError = initErr
	plugin.startError = startErr
	plugin.stopError = stopErr

	// Test Init with error
	if err := plugin.Init(); err != initErr {
		t.Errorf("Expected init error, got %v", err)
	}

	// Test Start with error
	if err := plugin.Start(); err != startErr {
		t.Errorf("Expected start error, got %v", err)
	}

	// Test Stop with error
	if err := plugin.Stop(); err != stopErr {
		t.Errorf("Expected stop error, got %v", err)
	}
}
