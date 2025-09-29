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
	LoadConfig(configName string, config Config) error
	GetConfig(configName string) (Config, error)
	RegisterValidator(configName string, validator ValidatorFunc)
	RegisterHook(configName string, hook HookFunc)
	SetBasePath(path string)
	SetEnvironment(env string)
	Close() error
}

// ValidatorFunc configuration validation function
type ValidatorFunc func(Config) error

// HookFunc configuration change hook function
type HookFunc func(oldVal, newVal Config) error

// configManager implementation of ConfigManager interface
type configManager struct {
	mu         sync.RWMutex
	configs    map[string]Config
	watchers   map[string]*fsnotify.Watcher
	validators map[string]ValidatorFunc
	hooks      map[string][]HookFunc
	basePath   string
	env        string
}

// NewConfigManager creates a new configuration manager
func NewConfigManager() ConfigManager {
	return &configManager{
		configs:    make(map[string]Config),
		watchers:   make(map[string]*fsnotify.Watcher),
		validators: make(map[string]ValidatorFunc),
		hooks:      make(map[string][]HookFunc),
		basePath:   "./configs",
		env:        "development",
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
	if validator, exists := cm.validators[configName]; exists {
		if err := validator(config); err != nil {
			return fmt.Errorf("validate config failed: %w", err)
		}
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

// RegisterValidator registers configuration validator
func (cm *configManager) RegisterValidator(configName string, validator ValidatorFunc) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.validators[configName] = validator
}

// RegisterHook registers configuration change hook
func (cm *configManager) RegisterHook(configName string, hook HookFunc) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.hooks[configName] = append(cm.hooks[configName], hook)
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
	if validator, exists := cm.validators[configName]; exists {
		if err := validator(newConfig); err != nil {
			// Log validation error but don't panic - keep using old config
			fmt.Printf("reloadConfig: validation failed for config %s: %v\n", configName, err)
			return
		}
	}

	// Execute hook functions
	if hooks, exists := cm.hooks[configName]; exists {
		for _, hook := range hooks {
			if err := hook(oldConfig, newConfig); err != nil {
				// Log hook error but don't panic - keep using old config
				fmt.Printf("reloadConfig: hook failed for config %s: %v\n", configName, err)
				return
			}
		}
	}

	// Directly replace map value (already protected by lock)
	cm.configs[configName] = newConfig
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
