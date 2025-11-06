// Package mysql registers and manages the MySQL-backed database implementation.
package mysql

import (
	"github.com/lcx/asura/config"
	"github.com/lcx/asura/plugin"
)

var (
	// openDatabaseFn wraps DBOpen to allow tests to substitute a fake database implementation.
	openDatabaseFn = DBOpen
)

func init() {
	plugin.RegisterPlugin(&factory{})
}

// factory implements the MySQL database plugin factory.
type factory struct{}

// Type identifies the plugin as a database implementation.
func (f *factory) Type() plugin.Type {
	return plugin.DB
}

// Name returns the canonical plugin name.
func (f *factory) Name() string {
	return "mysql"
}

// Setup builds a new MySQL Database using the provided configuration payload.
func (f *factory) Setup(v map[string]any) (plugin.Plugin, error) {
	cfg := &Config{}
	if err := config.Decode(v, cfg); err != nil {
		return nil, err
	}

	return openDatabaseFn(cfg)
}

// Destroy releases resources associated with the plugin instance.
func (f *factory) Destroy(plugin.Plugin, any) error {
	return nil
}

// Reload applies configuration updates for hot-reload scenarios.
func (f *factory) Reload(plugin.Plugin, map[string]any) error {
	return nil
}

// CanDelete reports whether the plugin instance can be safely deleted.
func (f *factory) CanDelete(plugin.Plugin) bool {
	return false
}
