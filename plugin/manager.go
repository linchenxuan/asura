package plugin

import (
	"fmt"
	"sync"
	"time"

	"github.com/lcx/asura/log"
)

// PluginManager plugin manager interface
// responsible for plugin registration, startup, shutdown and lifecycle management
type PluginManager interface {
	// RegisterPlugin register plugin
	RegisterPlugin(plugin Plugin) error

	// UnregisterPlugin unregister plugin
	UnregisterPlugin(name string) error

	// StartAll start all plugins
	StartAll() error

	// StopAll stop all plugins
	StopAll() error

	// StartPlugin start specified plugin
	StartPlugin(name string) error

	// StopPlugin stop specified plugin
	StopPlugin(name string) error

	// GetPlugin get plugin instance
	GetPlugin(name string) Plugin

	// GetPluginInfo get plugin information
	GetPluginInfo(name string) (*PluginInfo, error)

	// ListPlugins list all plugins
	ListPlugins() []PluginInfo
}

// pluginManager plugin manager implementation
type pluginManager struct {
	plugins map[string]Plugin
	infos   map[string]*PluginInfo
	started map[string]bool
	mu      sync.RWMutex
}

// NewPluginManager create new plugin manager
func NewPluginManager() PluginManager {
	return &pluginManager{
		plugins: make(map[string]Plugin),
		infos:   make(map[string]*PluginInfo),
		started: make(map[string]bool),
	}
}

// RegisterPlugin register plugin
func (pm *pluginManager) RegisterPlugin(plugin Plugin) error {
	if plugin == nil {
		return fmt.Errorf("plugin cannot be nil")
	}

	name := plugin.Name()
	if name == "" {
		return fmt.Errorf("plugin name cannot be empty")
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()

	if _, exists := pm.plugins[name]; exists {
		return fmt.Errorf("plugin %s already registered", name)
	}

	// register plugin
	pm.plugins[name] = plugin

	// create plugin info
	pm.infos[name] = &PluginInfo{
		Name:         name,
		Version:      plugin.Version(),
		Status:       PluginStatusRegistered,
		Dependencies: plugin.Dependencies(),
	}

	log.Info().Str("name", name).Str("version", plugin.Version()).Msg("plugin registered")
	return nil
}

// UnregisterPlugin unregister plugin
func (pm *pluginManager) UnregisterPlugin(name string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	plugin, exists := pm.plugins[name]
	if !exists {
		return fmt.Errorf("plugin %s not found", name)
	}

	// if plugin is started, stop it first
	if pm.started[name] {
		if err := plugin.Stop(); err != nil {
			log.Error().Str("name", name).Err(err).Msg("failed to stop plugin during unregister")
		}
		delete(pm.started, name)
	}

	// delete plugin
	delete(pm.plugins, name)
	delete(pm.infos, name)

	log.Info().Str("name", name).Msg("plugin unregistered")
	return nil
}

// StartAll start all plugins
func (pm *pluginManager) StartAll() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// resolve dependencies
	order, err := pm.resolveDependencies()
	if err != nil {
		return fmt.Errorf("failed to resolve dependencies: %w", err)
	}

	log.Info().Strs("order", order).Msg("starting plugins in order")

	// initialize plugins in order
	for _, pluginName := range order {
		plugin := pm.plugins[pluginName]
		info := pm.infos[pluginName]

		log.Info().Str("name", pluginName).Msg("initializing plugin")

		if err := plugin.Init(); err != nil {
			info.Status = PluginStatusError
			info.Error = err
			return NewPluginError(pluginName, "init", err)
		}

		info.Status = PluginStatusInitialized
		log.Info().Str("name", pluginName).Msg("plugin initialized")
	}

	// start plugins in order
	for _, pluginName := range order {
		plugin := pm.plugins[pluginName]
		info := pm.infos[pluginName]

		log.Info().Str("name", pluginName).Msg("starting plugin")

		if err := plugin.Start(); err != nil {
			info.Status = PluginStatusError
			info.Error = err
			return NewPluginError(pluginName, "start", err)
		}

		info.Status = PluginStatusStarted
		info.StartTime = time.Now()
		pm.started[pluginName] = true

		log.Info().Str("name", pluginName).Msg("plugin started")
	}

	return nil
}

// StopAll stop all plugins
func (pm *pluginManager) StopAll() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// get reverse order of startup
	order, err := pm.resolveDependencies()
	if err != nil {
		return fmt.Errorf("failed to resolve dependencies: %w", err)
	}

	// stop plugins in reverse order
	for i := len(order) - 1; i >= 0; i-- {
		pluginName := order[i]
		plugin := pm.plugins[pluginName]
		info := pm.infos[pluginName]

		if !pm.started[pluginName] {
			continue
		}

		log.Info().Str("name", pluginName).Msg("stopping plugin")

		if err := plugin.Stop(); err != nil {
			info.Status = PluginStatusError
			info.Error = err
			log.Error().Str("name", pluginName).Err(err).Msg("failed to stop plugin")
			continue
		}

		info.Status = PluginStatusStopped
		info.StopTime = time.Now()
		delete(pm.started, pluginName)

		log.Info().Str("name", pluginName).Msg("plugin stopped")
	}

	return nil
}

// StartPlugin start specified plugin
func (pm *pluginManager) StartPlugin(name string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	plugin, exists := pm.plugins[name]
	if !exists {
		return fmt.Errorf("plugin %s not found", name)
	}

	if pm.started[name] {
		return fmt.Errorf("plugin %s already started", name)
	}

	info := pm.infos[name]

	// initialize plugin
	if info.Status == PluginStatusRegistered {
		log.Info().Str("name", name).Msg("initializing plugin")

		if err := plugin.Init(); err != nil {
			info.Status = PluginStatusError
			info.Error = err
			return NewPluginError(name, "init", err)
		}

		info.Status = PluginStatusInitialized
		log.Info().Str("name", name).Msg("plugin initialized")
	}

	// start plugin
	log.Info().Str("name", name).Msg("starting plugin")

	if err := plugin.Start(); err != nil {
		info.Status = PluginStatusError
		info.Error = err
		return NewPluginError(name, "start", err)
	}

	info.Status = PluginStatusStarted
	info.StartTime = time.Now()
	pm.started[name] = true

	log.Info().Str("name", name).Msg("plugin started")
	return nil
}

// StopPlugin stop specified plugin
func (pm *pluginManager) StopPlugin(name string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	plugin, exists := pm.plugins[name]
	if !exists {
		return fmt.Errorf("plugin %s not found", name)
	}

	if !pm.started[name] {
		return fmt.Errorf("plugin %s not started", name)
	}

	info := pm.infos[name]

	log.Info().Str("name", name).Msg("stopping plugin")

	if err := plugin.Stop(); err != nil {
		info.Status = PluginStatusError
		info.Error = err
		return NewPluginError(name, "stop", err)
	}

	info.Status = PluginStatusStopped
	info.StopTime = time.Now()
	delete(pm.started, name)

	log.Info().Str("name", name).Msg("plugin stopped")
	return nil
}

// GetPlugin get plugin instance
func (pm *pluginManager) GetPlugin(name string) Plugin {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	return pm.plugins[name]
}

// GetPluginInfo get plugin information
func (pm *pluginManager) GetPluginInfo(name string) (*PluginInfo, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	info, exists := pm.infos[name]
	if !exists {
		return nil, fmt.Errorf("plugin %s not found", name)
	}

	// return copy to avoid external modification
	infoCopy := *info
	return &infoCopy, nil
}

// ListPlugins list all plugins
func (pm *pluginManager) ListPlugins() []PluginInfo {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	infos := make([]PluginInfo, 0, len(pm.infos))
	for _, info := range pm.infos {
		// return copy to avoid external modification
		infos = append(infos, *info)
	}

	return infos
}

// resolveDependencies resolve plugin dependencies and return startup order
func (pm *pluginManager) resolveDependencies() ([]string, error) {
	// use topological sort to resolve dependencies
	visited := make(map[string]bool)
	tempVisited := make(map[string]bool)
	result := make([]string, 0, len(pm.plugins))

	var visit func(string) error
	visit = func(name string) error {
		if tempVisited[name] {
			return fmt.Errorf("circular dependency detected involving plugin %s", name)
		}

		if visited[name] {
			return nil
		}

		tempVisited[name] = true

		// check if plugin exists
		plugin, exists := pm.plugins[name]
		if !exists {
			return fmt.Errorf("plugin %s not found", name)
		}

		// recursively process dependencies
		for _, dep := range plugin.Dependencies() {
			if err := visit(dep); err != nil {
				return err
			}
		}

		tempVisited[name] = false
		visited[name] = true
		result = append(result, name)

		return nil
	}

	// perform topological sort for all plugins
	for name := range pm.plugins {
		if !visited[name] {
			if err := visit(name); err != nil {
				return nil, err
			}
		}
	}

	return result, nil
}
