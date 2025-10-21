package plugin

// Factory defines the plugin factory interface.
// Each plugin type must implement this interface to support lifecycle management.
//
// Lifecycle methods:
//   - Setup: Initialize plugin instance with configuration
//   - Destroy: Clean up plugin resources (connections, file handles, etc.)
//   - Reload: Hot reload plugin with new configuration (optional, can return error if not supported)
//   - CanDelete: Check if plugin can be safely deleted (e.g., no active connections)
//
// Thread-safety: Factory implementations must be thread-safe for concurrent Setup/Destroy calls.
type Factory interface {
	// Type returns the plugin type (e.g., "db", "cstransport")
	Type() Type

	// Name returns the factory name (e.g., "mysql", "redis")
	Name() string

	// Setup initializes a new plugin instance with the given configuration.
	// Returns the plugin instance or error if initialization fails.
	// Thread-safe: can be called concurrently for different instances.
	Setup(v map[string]any) (Plugin, error)

	// Destroy cleans up plugin resources (connections, file handles, goroutines, etc.).
	// The second parameter is reserved for future use (e.g., graceful shutdown timeout).
	// Thread-safe: can be called concurrently for different instances.
	Destroy(Plugin, any) error

	// Reload hot reloads the plugin with new configuration.
	// Returns error if hot reload is not supported or fails.
	// Thread-safe: must handle concurrent access to plugin state.
	Reload(Plugin, map[string]any) error

	// CanDelete checks if the plugin can be safely deleted.
	// Returns false if plugin is processing critical tasks (e.g., active connections, pending writes).
	// Thread-safe: must be safe to call during plugin operation.
	CanDelete(Plugin) bool
}

var (
	// _factoryMap stores all registered plugin factories.
	// Key format: "<plugin_type>_<factory_name>" (e.g., "db_mysql", "cstransport_tcp")
	// Protected by _pluginLock in plugin.go for thread-safe access.
	_factoryMap = make(map[string]Factory)
)
