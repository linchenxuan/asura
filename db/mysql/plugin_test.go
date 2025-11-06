package mysql

import (
	"errors"
	"testing"

	"github.com/lcx/asura/db"
	"github.com/lcx/asura/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFactoryTypeAndName(t *testing.T) {
	f := &factory{}
	assert.Equal(t, plugin.Type(plugin.DB), f.Type())
	assert.Equal(t, "mysql", f.Name())
	assert.False(t, f.CanDelete(&stubDatabase{}))
	assert.NoError(t, f.Destroy(&stubDatabase{}, nil))
	assert.NoError(t, f.Reload(&stubDatabase{}, nil))
}

func TestFactorySetupDecodeError(t *testing.T) {
	f := &factory{}
	input := map[string]any{
		"tag":        "unit",
		"datasource": "dsn",
		"idleconns":  1,
		// missing maxlifetime
	}

	_, err := f.Setup(input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "maxlifetime")
}

func TestFactorySetupSuccess(t *testing.T) {
	f := &factory{}
	original := openDatabaseFn
	var captured *Config
	openDatabaseFn = func(cfg *Config) (db.Database, error) {
		captured = cfg
		return &stubDatabase{}, nil
	}
	t.Cleanup(func() { openDatabaseFn = original })

	input := map[string]any{
		"tag":         "unit",
		"datasource":  "dsn",
		"idleconns":   3,
		"maxlifetime": 10,
	}

	p, err := f.Setup(input)
	require.NoError(t, err)
	require.NotNil(t, p)
	require.NotNil(t, captured)
	assert.Equal(t, "unit", captured.Tag)
	assert.Equal(t, "dsn", captured.DataSource)
	assert.Equal(t, 3, captured.IdleConns)
	assert.Equal(t, uint32(10), captured.MaxLifeTime)
}

func TestFactorySetupPropagatesDBErrors(t *testing.T) {
	f := &factory{}
	original := openDatabaseFn
	openDatabaseFn = func(*Config) (db.Database, error) {
		return nil, errors.New("boom")
	}
	t.Cleanup(func() { openDatabaseFn = original })

	input := map[string]any{
		"tag":         "unit",
		"datasource":  "dsn",
		"idleconns":   3,
		"maxlifetime": 10,
	}

	_, err := f.Setup(input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}
