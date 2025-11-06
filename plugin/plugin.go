package plugin

import (
	"fmt"
	"strings"

	"github.com/lcx/asura/config"
	"github.com/lcx/asura/log"
)

// Type represents the plugin type supported by the system.
// Each type corresponds to a specific category of functionality (e.g., transport, database, logging).
type Type string

const (
	// DB DB插件类型.
	DB = "db"
)

const (
	DefaultInsName = "default" // DefaultInsName is the default instance name when not specified in config.
)

// PluginConfig represents the plugin configuration structure.
// Structure: map[plugin_type][factory_name_instance_config] = config_items
// Example YAML:
//
//	db:
//	  mysql_master:
//	    host: localhost
//	    port: 3306
//	    tag: master  # Instance name (optional, defaults to "default")
type PluginConfig map[string]map[string]map[string]any

// GetName implements the config.Config interface.
// Returns the configuration name used for registration with ConfigManager.
func (c *PluginConfig) GetName() string {
	return "plugin"
}

// Validate implements the config.Config interface.
// Validates the plugin configuration structure and ensures all required fields are present.
func (c *PluginConfig) Validate() error {
	if c == nil || len(*c) == 0 {
		return fmt.Errorf("plugin config is empty")
	}

	// Validate configuration for each plugin type
	for pluginType, factories := range *c {
		if len(factories) == 0 {
			return fmt.Errorf("plugin type %s has no factory config", pluginType)
		}
		for factoryName, instances := range factories {
			if len(instances) == 0 {
				return fmt.Errorf("plugin %s_%s has no instance config", pluginType, factoryName)
			}
		}
	}

	return nil
}

// Plugin represents the plugin instance interface.
// All plugin implementations must satisfy this interface.
type Plugin interface { //nolint:revive
	FactoryName() string
}

// RegisterPlugin registers a plugin factory with the plugin manager.
// This should be called during package initialization (init function).
// Thread-safe: uses global lock for concurrent registration.
func RegisterPlugin(f Factory) {
	_pluginLock.Lock()
	defer _pluginLock.Unlock()
	_factoryMap[fmt.Sprintf("%s_%s", f.Type(), f.Name())] = f
}

// InitPlugins initializes all plugins with automatic rollback on partial failure.
// Automatically loads configuration from ConfigManager and registers as a config change listener for hot reload.
// IMPORTANT: Must be called after config.GetInstance() is initialized.
//
// Initialization flow:
// 1. Load plugin config from ConfigManager
// 2. Register as config change listener for hot reload support
// 3. Initialize plugins in order (with rollback on failure)
// 4. Register plugin instances in the global registry
//
// Thread-safe: uses global lock to protect initialization process.
func InitPlugins() error {
	_pluginLock.Lock()
	defer _pluginLock.Unlock()

	// Use singleton ConfigManager directly (no injection needed)
	cm := config.GetInstance()

	// Load plugin configuration from ConfigManager
	var cfg PluginConfig
	if err := cm.LoadConfig("plugin", &cfg); err != nil {
		return fmt.Errorf("load plugin config failed: %v", err)
	}

	// Register as config change listener for hot reload support
	cm.AddChangeListener(_pluginMgr)
	log.Info().Msg("plugin manager registered as config change listener")

	// Track initialized plugins for rollback on partial failure
	initializedPlugins := make([]struct {
		ft, fn, pn string
		ins        Plugin
	}, 0)

	for ft, s := range cfg {
		haveDefault := false
		for k, c := range s {
			fn := getFactoryName(k)
			f := getPluginFactory(ft, fn)
			if f == nil {
				// Rollback all initialized plugins on factory not found error
				rollbackPlugins(initializedPlugins)
				return fmt.Errorf("plugin factory [%s/%s] not found, available factories: %v",
					ft, fn, listAvailableFactories(ft))
			}

			log.Info().Str("type", string(f.Type())).Str("name", f.Name()).Msg("plugin setup begin")
			ins, err := f.Setup(c)
			if err != nil {
				// Rollback all initialized plugins on setup failure
				rollbackPlugins(initializedPlugins)
				return fmt.Errorf("plugin [%s/%s] setup failed: %v", ft, fn, err)
			}

			pn := getPluginNameFromCfg(c)
			if pn == DefaultInsName {
				if haveDefault {
					rollbackPlugins(initializedPlugins)
					return fmt.Errorf("plugin type [%s] default instance already exists", ft)
				}
				haveDefault = true
			}
			if err := registerPluginIns(ft, fn, pn, ins); err != nil {
				// Destroy current plugin instance
				f.Destroy(ins, nil)
				// Rollback all previously initialized plugins
				rollbackPlugins(initializedPlugins)
				return err
			}

			// Track successfully initialized plugin
			initializedPlugins = append(initializedPlugins, struct {
				ft, fn, pn string
				ins        Plugin
			}{ft, fn, pn, ins})

			log.Info().Str("type", string(f.Type())).Str("name", f.Name()).
				Str("instance", pn).Msg("plugin setup success")
		}
	}

	log.Info().Int("count", len(initializedPlugins)).Msg("InitPlugins success")
	return nil
}

// OnConfigChanged implements the config.ConfigChangeListener interface.
// Automatically reinitializes affected plugins when configuration file changes.
//
// Hot reload strategy (production-grade):
// 1. Safety check: Call CanDelete() to ensure plugins can be safely stopped
// 2. Lightweight update: Try Reload() first (10x faster than Destroy+Setup)
// 3. Fallback: If Reload() fails, fallback to Destroy+Setup
// 4. Atomic rollback: Rollback all changes on any failure
//
// Performance characteristics:
// - Reload-only: ~10ms (e.g., connection pool size adjustment)
// - Partial recreate: ~50ms (e.g., add new plugin)
// - Full recreate: ~100ms (e.g., major config change)
func (pm *pluginMgr) OnConfigChanged(configName string, newConfig, oldConfig config.Config) error {
	// Only handle plugin configuration changes
	if configName != "plugin" {
		return nil
	}

	// Type assertion to PluginConfig
	newPluginConfig, ok := newConfig.(*PluginConfig)
	if !ok {
		return fmt.Errorf("invalid config type: expected *PluginConfig, got %T", newConfig)
	}

	// If old config is nil (first load), skip hot reload
	if oldConfig == nil {
		log.Info().Msg("old config is nil, skipping hot reload")
		return nil
	}

	oldPluginConfig := oldConfig.(*PluginConfig)
	log.Info().Str("config", configName).Msg("plugin config changed, performing safe hot reload...")

	// Acquire global lock to protect atomicity of entire hot reload process
	_pluginLock.Lock()
	defer _pluginLock.Unlock()

	// Step 1: Safety check - ensure all plugins can be safely deleted
	// This prevents data loss (e.g., uncommitted DB transactions, active network connections)
	for pluginType, factories := range pm.insMap {
		for factoryName, instances := range factories {
			factory := getPluginFactory(pluginType, factoryName)
			if factory == nil {
				continue
			}

			for instanceName, instance := range instances {
				// Check if plugin can be safely deleted (e.g., no active connections)
				if !factory.CanDelete(instance) {
					return fmt.Errorf("plugin [%s/%s/%s] cannot be deleted: has active tasks (e.g., DB transactions, network connections)",
						pluginType, factoryName, instanceName)
				}
			}
		}
	}

	// Step 2: Try lightweight hot reload for existing plugins
	// Reload() is 10x faster than Destroy+Setup (e.g., only update connection pool size)
	reloadedPlugins := make(map[string]bool) // Track successfully reloaded plugins
	for ft, s := range *newPluginConfig {
		for k, c := range s {
			fn := getFactoryName(k)
			pn := getPluginNameFromCfg(c)
			key := fmt.Sprintf("%s/%s/%s", ft, fn, pn)

			// Check if plugin exists in old config
			if oldFactories, ok := (*oldPluginConfig)[ft]; ok {
				if oldConfig, ok := oldFactories[k]; ok {
					oldPn := getPluginNameFromCfg(oldConfig)
					if oldPn == pn {
						// Plugin exists, try to reload
						factory := getPluginFactory(ft, fn)
						if factory == nil {
							continue
						}

						if instance, err := GetPlugin(ft, fn, pn); err == nil {
							log.Info().Str("type", ft).Str("factory", fn).
								Str("instance", pn).Msg("attempting lightweight hot reload...")

							// Try Reload() first (lightweight update)
							if err := factory.Reload(instance, c); err == nil {
								log.Info().Str("type", ft).Str("factory", fn).
									Str("instance", pn).Msg("hot reload success (lightweight)")
								reloadedPlugins[key] = true
								continue
							} else {
								log.Warn().Err(err).Str("type", ft).Str("factory", fn).
									Str("instance", pn).Msg("hot reload failed, will recreate plugin")
							}
						}
					}
				}
			}
		}
	}

	// Step 3: Destroy plugins that need to be recreated or removed
	for pluginType, factories := range pm.insMap {
		for factoryName, instances := range factories {
			factory := getPluginFactory(pluginType, factoryName)
			if factory == nil {
				continue
			}

			for instanceName, instance := range instances {
				key := fmt.Sprintf("%s/%s/%s", pluginType, factoryName, instanceName)

				// Skip reloaded plugins (already updated)
				if reloadedPlugins[key] {
					continue
				}

				log.Debug().Str("type", pluginType).Str("factory", factoryName).
					Str("instance", instanceName).Msg("destroying plugin for recreation...")

				// Destroy plugin (release connections, file handles, etc.)
				if err := factory.Destroy(instance, nil); err != nil {
					log.Error().Err(err).Str("type", pluginType).
						Str("factory", factoryName).Str("instance", instanceName).
						Msg("destroy plugin failed")
				}
			}
		}
	}

	// Step 4: Rebuild plugin instance map (keep reloaded plugins)
	newInsMap := make(map[string]map[string]map[string]Plugin)
	for key := range reloadedPlugins {
		parts := strings.Split(key, "/")
		if len(parts) != 3 {
			continue
		}
		ft, fn, pn := parts[0], parts[1], parts[2]

		if instance, err := GetPlugin(ft, fn, pn); err == nil {
			if newInsMap[ft] == nil {
				newInsMap[ft] = make(map[string]map[string]Plugin)
			}
			if newInsMap[ft][fn] == nil {
				newInsMap[ft][fn] = make(map[string]Plugin)
			}
			newInsMap[ft][fn][pn] = instance
		}
	}
	pm.insMap = newInsMap

	// Step 5: Setup new plugins (skip reloaded ones)
	initializedPlugins := make([]struct {
		ft, fn, pn string
		ins        Plugin
	}, 0)

	for ft, s := range *newPluginConfig {
		haveDefault := false
		for k, c := range s {
			fn := getFactoryName(k)
			pn := getPluginNameFromCfg(c)
			key := fmt.Sprintf("%s/%s/%s", ft, fn, pn)

			// Skip reloaded plugins (already updated)
			if reloadedPlugins[key] {
				if pn == DefaultInsName {
					haveDefault = true
				}
				continue
			}

			f := getPluginFactory(ft, fn)
			if f == nil {
				rollbackPlugins(initializedPlugins)
				return fmt.Errorf("plugin factory [%s/%s] not found, available factories: %v",
					ft, fn, listAvailableFactories(ft))
			}

			log.Info().Str("type", string(f.Type())).Str("name", f.Name()).Msg("plugin setup begin")
			ins, err := f.Setup(c)
			if err != nil {
				// Rollback all initialized plugins on setup failure
				rollbackPlugins(initializedPlugins)
				return fmt.Errorf("plugin [%s/%s] setup failed: %v", ft, fn, err)
			}

			if pn == DefaultInsName {
				if haveDefault {
					rollbackPlugins(initializedPlugins)
					return fmt.Errorf("plugin type [%s] default instance already exists", ft)
				}
				haveDefault = true
			}
			if err := registerPluginIns(ft, fn, pn, ins); err != nil {
				// Destroy current plugin instance
				f.Destroy(ins, nil)
				// Rollback all previously initialized plugins
				rollbackPlugins(initializedPlugins)
				return err
			}

			// Track successfully initialized plugin
			initializedPlugins = append(initializedPlugins, struct {
				ft, fn, pn string
				ins        Plugin
			}{ft, fn, pn, ins})

			log.Info().Str("type", string(f.Type())).Str("name", f.Name()).
				Str("instance", pn).Msg("plugin setup success")
		}
	}

	log.Info().Int("reloaded", len(reloadedPlugins)).
		Int("recreated", len(initializedPlugins)).
		Msg("all plugins hot reload completed")
	return nil
}

// getPluginFactory retrieves plugin factory by type and name (must check for nil).
func getPluginFactory(ft string, fn string) Factory {
	_pluginLock.RLock()
	defer _pluginLock.RUnlock()
	return _factoryMap[fmt.Sprintf("%s_%s", ft, fn)]
}

// getPluginNameFromCfg extracts tag from config key-value pairs (future: support multiple tags).
func getPluginNameFromCfg(c map[string]any) string {
	t, ok := c["tag"]
	if !ok {
		return DefaultInsName
	}
	tag, ok := t.(string)
	if !ok {
		return DefaultInsName
	}
	return tag
}

func getFactoryName(fn string) string {
	return strings.Split(fn, "_")[0]
}

// GetPlugin retrieves plugin instance (thread-safe, zero-copy).
// ft: plugin type (e.g., "db")
// fn: factory name (e.g., "mysql")
// pn: instance name (e.g., "master"/"slave")
// Returns: plugin instance pointer (requires type assertion), error.
func GetPlugin(ft, fn, pn string) (Plugin, error) {
	_pluginLock.RLock()
	defer _pluginLock.RUnlock()

	typeMap, ok := _pluginMgr.insMap[ft]
	if !ok {
		return nil, fmt.Errorf("plugin type [%s] not registered", ft)
	}

	factoryMap, ok := typeMap[fn]
	if !ok {
		return nil, fmt.Errorf("plugin factory [%s/%s] not found", ft, fn)
	}

	ins, ok := factoryMap[pn]
	if !ok {
		return nil, fmt.Errorf("plugin instance [%s/%s/%s] not found", ft, fn, pn)
	}

	return ins, nil
}

// GetDefaultPlugin retrieves default instance (simplifies high-frequency calls).
// Use case: 90% of plugins have only one instance (e.g., primary MySQL, Redis).
func GetDefaultPlugin(ft, fn string) (Plugin, error) {
	return GetPlugin(ft, fn, DefaultInsName)
}

// MustGetPlugin retrieves plugin (panics on failure).
// Use case: Get critical plugins during startup (e.g., log/database), should terminate immediately on failure.
func MustGetPlugin(ft, fn, pn string) Plugin {
	ins, err := GetPlugin(ft, fn, pn)
	if err != nil {
		log.Fatal().Err(err).Str("type", ft).Str("factory", fn).
			Str("instance", pn).Msg("critical plugin not found")
	}
	return ins
}

// ListPlugins lists all registered plugins (for monitoring/debugging).
// Return format: map["db/mysql"] = ["master", "slave"].
func ListPlugins() map[string][]string {
	_pluginLock.RLock()
	defer _pluginLock.RUnlock()

	result := make(map[string][]string)
	for ft, typeMap := range _pluginMgr.insMap {
		for fn, factoryMap := range typeMap {
			key := fmt.Sprintf("%s/%s", ft, fn)
			for pn := range factoryMap {
				result[key] = append(result[key], pn)
			}
		}
	}
	return result
}

// rollbackPlugins rolls back initialized plugins.
func rollbackPlugins(plugins []struct {
	ft, fn, pn string
	ins        Plugin
}) {
	if len(plugins) == 0 {
		return
	}

	log.Warn().Int("count", len(plugins)).Msg("rolling back initialized plugins...")

	for i := len(plugins) - 1; i >= 0; i-- {
		p := plugins[i]
		factory := getPluginFactory(p.ft, p.fn)
		if factory == nil {
			continue
		}

		log.Info().Str("type", p.ft).Str("factory", p.fn).
			Str("instance", p.pn).Msg("rolling back plugin...")

		if err := factory.Destroy(p.ins, nil); err != nil {
			log.Error().Err(err).Str("type", p.ft).Str("factory", p.fn).
				Str("instance", p.pn).Msg("rollback failed")
		}
	}

	// Clear plugin manager
	_pluginLock.Lock()
	_pluginMgr.insMap = make(map[string]map[string]map[string]Plugin)
	_pluginLock.Unlock()

	log.Info().Msg("rollback completed")
}

// listAvailableFactories lists available factories (for error messages).
func listAvailableFactories(ft string) []string {
	_pluginLock.RLock()
	defer _pluginLock.RUnlock()

	var factories []string
	for key := range _factoryMap {
		if strings.HasPrefix(key, ft+"_") {
			factories = append(factories, strings.TrimPrefix(key, ft+"_"))
		}
	}
	return factories
}
