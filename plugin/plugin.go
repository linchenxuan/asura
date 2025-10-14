package plugin

import (
	"context"
	"fmt"
	"time"
)

// Plugin basic plugin interface
// all plugins must implement this interface
type Plugin interface {
	// Name returns plugin name
	Name() string

	// Version returns plugin version
	Version() string

	// Dependencies returns list of other plugin names that this plugin depends on
	Dependencies() []string

	// Init initializes plugin, called before Start
	Init() error

	// Start starts plugin
	Start() error

	// Stop stops plugin
	Stop() error
}

// PluginStatus plugin status
type PluginStatus int

const (
	PluginStatusUnknown PluginStatus = iota
	PluginStatusRegistered
	PluginStatusInitialized
	PluginStatusStarted
	PluginStatusStopped
	PluginStatusError
)

// String returns string representation of plugin status
func (s PluginStatus) String() string {
	switch s {
	case PluginStatusRegistered:
		return "registered"
	case PluginStatusInitialized:
		return "initialized"
	case PluginStatusStarted:
		return "started"
	case PluginStatusStopped:
		return "stopped"
	case PluginStatusError:
		return "error"
	default:
		return "unknown"
	}
}

// PluginInfo plugin information
type PluginInfo struct {
	Name         string
	Version      string
	Status       PluginStatus
	Dependencies []string
	StartTime    time.Time
	StopTime     time.Time
	Error        error
}

// PluginContext plugin context
// provides runtime context information for plugins
type PluginContext struct {
	context.Context
}

// PluginError plugin error
type PluginError struct {
	PluginName string
	Operation  string
	Err        error
}

// Error implements error interface
func (e *PluginError) Error() string {
	return fmt.Sprintf("plugin %s %s failed: %v", e.PluginName, e.Operation, e.Err)
}

// Unwrap returns original error
func (e *PluginError) Unwrap() error {
	return e.Err
}

// NewPluginError creates plugin error
func NewPluginError(pluginName, operation string, err error) *PluginError {
	return &PluginError{
		PluginName: pluginName,
		Operation:  operation,
		Err:        err,
	}
}
