package config

import (
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

// ConfigManager interface for configuration management
type ConfigManager interface {
	ConfigChangeNotifier
	LoadConfig(configName string, config Config) error
	GetConfig(configName string) (Config, error)
	SetBasePath(path string)
	SetEnvironment(env string)
	Close() error
}

// configManager implementation of ConfigManager interface
type configManager struct {
	mu        sync.RWMutex
	configs   map[string]Config
	watchers  map[string]*fsnotify.Watcher
	basePath  string
	env       string
	listeners []ConfigChangeListener
}

// NewConfigManager creates a new configuration manager
func NewConfigManager() ConfigManager {
	return &configManager{
		configs:  make(map[string]Config),
		watchers: make(map[string]*fsnotify.Watcher),
		basePath: "./configs",
		env:      "development",
	}
}

// LoadConfig loads configuration from file
func (cm *configManager) LoadConfig(configName string, config Config) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	v := viper.New()

	// Set configuration file path
	v.SetConfigName(configName)
	v.SetConfigType("yaml")
	v.AddConfigPath(cm.basePath)
	v.AddConfigPath(fmt.Sprintf("%s/%s", cm.basePath, cm.env))

	// Read environment variables for override
	v.AutomaticEnv()
	v.SetEnvPrefix(strings.ToUpper(configName))
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Read configuration
	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("read config failed: %w", err)
	}

	// Unmarshal to struct
	if err := v.Unmarshal(config); err != nil {
		return fmt.Errorf("unmarshal config failed: %w", err)
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return fmt.Errorf("validate config failed: %w", err)
	}

	// Store configuration
	cm.configs[configName] = config

	// Set up file watching
	if err := cm.watchConfigFile(configName, v); err != nil {
		return fmt.Errorf("watch config file failed: %w", err)
	}

	return nil
}

// GetConfigSafe safely retrieves configuration with type assertion
func (cm *configManager) GetConfig(configName string) (Config, error) {
	config, exists := cm.configs[configName]
	if !exists {
		return nil, fmt.Errorf("config %s not found", configName)
	}

	return config, nil
}

// SetBasePath sets base path for configuration files
func (cm *configManager) SetBasePath(path string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.basePath = path
}

// SetEnvironment sets environment for configuration
func (cm *configManager) SetEnvironment(env string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.env = env
}

// watchConfigFile watches configuration file for changes
func (cm *configManager) watchConfigFile(configName string, v *viper.Viper) error {
	configFile := v.ConfigFileUsed()
	if configFile == "" {
		return nil
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	cm.watchers[configName] = watcher

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					cm.reloadConfig(configName)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				fmt.Printf("config watcher error: %v\n", err)
			}
		}
	}()

	return watcher.Add(configFile)
}

// reloadConfig reloads configuration when file changes
func (cm *configManager) reloadConfig(configName string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	oldConfig, exists := cm.configs[configName]
	if !exists {
		return
	}

	// Create new config instance (preserve original type via reflection)
	newConfig := reflect.New(reflect.TypeOf(oldConfig).Elem()).Interface().(Config)

	// Reload configuration (using viper)
	v := viper.New()
	v.SetConfigName(configName)
	v.SetConfigType("yaml")
	v.AddConfigPath(cm.basePath)
	v.AddConfigPath(fmt.Sprintf("%s/%s", cm.basePath, cm.env))

	if err := v.ReadInConfig(); err != nil {
		// Log error but don't panic - keep using old config
		fmt.Printf("reloadConfig: failed to read config %s: %v\n", configName, err)
		return
	}

	if err := v.Unmarshal(newConfig); err != nil {
		// Log error but don't panic - keep using old config
		fmt.Printf("reloadConfig: failed to unmarshal config %s: %v\n", configName, err)
		return
	}

	// Validate new configuration
	if err := newConfig.Validate(); err != nil {
		// Log validation error but don't panic - keep using old config
		fmt.Printf("reloadConfig: validation failed for config %s: %v\n", configName, err)
		return
	}

	// Replace map value (already protected by lock)
	cm.configs[configName] = newConfig

	// Notify all listeners about configuration change
	cm.NotifyConfigChanged(configName, newConfig, oldConfig)
}

// AddChangeListener adds a configuration change listener
func (cm *configManager) AddChangeListener(listener ConfigChangeListener) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Check if listener already exists
	for _, l := range cm.listeners {
		if l == listener {
			return
		}
	}

	cm.listeners = append(cm.listeners, listener)
}

// RemoveChangeListener removes a configuration change listener
func (cm *configManager) RemoveChangeListener(listener ConfigChangeListener) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for i, l := range cm.listeners {
		if l == listener {
			cm.listeners = append(cm.listeners[:i], cm.listeners[i+1:]...)
			return
		}
	}
}

// NotifyConfigChanged notifies all listeners about configuration change
func (cm *configManager) NotifyConfigChanged(configName string, newConfig, oldConfig Config) {
	// Make a copy of listeners to avoid holding the lock during notification
	listenersCopy := make([]ConfigChangeListener, len(cm.listeners))
	copy(listenersCopy, cm.listeners)

	// Unlock before notifying to prevent deadlocks
	cm.mu.Unlock()

	// Notify each listener
	for _, listener := range listenersCopy {
		if err := listener.OnConfigChanged(configName, newConfig, oldConfig); err != nil {
			fmt.Printf("NotifyConfigChanged: listener failed for config %s: %v\n", configName, err)
		}
	}

	// Relock to restore original state
	cm.mu.Lock()
}

// Close closes the configuration manager
func (cm *configManager) Close() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for _, watcher := range cm.watchers {
		if err := watcher.Close(); err != nil {
			return err
		}
	}

	return nil
}

// ConfigManagerProvider provides configuration manager
type ConfigManagerProvider struct {
	configManager ConfigManager
}

// NewConfigManagerProvider creates a new configuration manager provider
func NewConfigManagerProvider(cm ConfigManager) *ConfigManagerProvider {
	return &ConfigManagerProvider{
		configManager: cm,
	}
}

// GetConfigManager gets the configuration manager
func (p *ConfigManagerProvider) GetConfigManager() ConfigManager {
	return p.configManager
}

// SetConfigManager sets the configuration manager
func (p *ConfigManagerProvider) SetConfigManager(cm ConfigManager) {
	p.configManager = cm
}
