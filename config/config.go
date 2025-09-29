package config

// Config interface defines the basic configuration contract
type Config interface {
	GetName() string
	Validate() error
}

// ConfigChangeListener interface for listening to configuration changes
// Business layers should implement this interface to handle config changes
type ConfigChangeListener interface {
	// OnConfigChanged is called when configuration changes
	OnConfigChanged(configName string, newConfig, oldConfig Config) error
}

// ConfigChangeNotifier interface for notifying configuration changes
type ConfigChangeNotifier interface {
	// AddChangeListener adds a configuration change listener
	AddChangeListener(listener ConfigChangeListener)
	// RemoveChangeListener removes a configuration change listener
	RemoveChangeListener(listener ConfigChangeListener)
	// NotifyConfigChanged notifies all listeners about configuration change
	NotifyConfigChanged(configName string, newConfig, oldConfig Config)
}
