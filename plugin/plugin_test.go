package plugin

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Mock Plugin Implementation (for testing)
// ============================================================================

// mockPlugin is a test plugin implementation
type mockPlugin struct {
	factoryName   string
	config        map[string]any
	setupCount    int32
	destroyCount  int32
	reloadCount   int32
	canDelete     bool
	reloadError   error
	destroyError  error
	activeTaskCnt int32 // Simulate active tasks (connections, transactions, etc.)
}

func (m *mockPlugin) FactoryName() string {
	return m.factoryName
}

func (m *mockPlugin) GetSetupCount() int {
	return int(atomic.LoadInt32(&m.setupCount))
}

func (m *mockPlugin) GetDestroyCount() int {
	return int(atomic.LoadInt32(&m.destroyCount))
}

func (m *mockPlugin) GetReloadCount() int {
	return int(atomic.LoadInt32(&m.reloadCount))
}

func (m *mockPlugin) GetActiveTaskCount() int {
	return int(atomic.LoadInt32(&m.activeTaskCnt))
}

func (m *mockPlugin) SetActiveTaskCount(count int) {
	atomic.StoreInt32(&m.activeTaskCnt, int32(count))
}

// mockFactory is a test factory implementation
type mockFactory struct {
	pluginType    Type
	factoryName   string
	setupError    error
	destroyError  error
	reloadError   error
	canDelete     bool
	setupDelay    time.Duration
	destroyDelay  time.Duration
	createdPlugin *mockPlugin
	mu            sync.Mutex
}

func newMockFactory(pluginType Type, factoryName string) *mockFactory {
	return &mockFactory{
		pluginType:  pluginType,
		factoryName: factoryName,
		canDelete:   true,
	}
}

func (f *mockFactory) Type() Type {
	return f.pluginType
}

func (f *mockFactory) Name() string {
	return f.factoryName
}

func (f *mockFactory) Setup(v map[string]any) (Plugin, error) {
	if f.setupDelay > 0 {
		time.Sleep(f.setupDelay)
	}

	if f.setupError != nil {
		return nil, f.setupError
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	plugin := &mockPlugin{
		factoryName:  f.factoryName,
		config:       v,
		canDelete:    f.canDelete,
		reloadError:  f.reloadError,
		destroyError: f.destroyError,
	}
	atomic.AddInt32(&plugin.setupCount, 1)
	f.createdPlugin = plugin

	return plugin, nil
}

func (f *mockFactory) Destroy(p Plugin, _ any) error {
	if f.destroyDelay > 0 {
		time.Sleep(f.destroyDelay)
	}

	if f.destroyError != nil {
		return f.destroyError
	}

	if mp, ok := p.(*mockPlugin); ok {
		atomic.AddInt32(&mp.destroyCount, 1)
	}

	return nil
}

func (f *mockFactory) Reload(p Plugin, v map[string]any) error {
	if f.reloadError != nil {
		return f.reloadError
	}

	if mp, ok := p.(*mockPlugin); ok {
		atomic.AddInt32(&mp.reloadCount, 1)
		mp.config = v
	}

	return nil
}

func (f *mockFactory) CanDelete(p Plugin) bool {
	if mp, ok := p.(*mockPlugin); ok {
		// Check if plugin has active tasks
		return mp.GetActiveTaskCount() == 0
	}
	return f.canDelete
}

func (f *mockFactory) GetCreatedPlugin() *mockPlugin {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.createdPlugin
}

// ============================================================================
// Test Helper Functions
// ============================================================================

// resetPluginManager resets global plugin manager state (for test isolation)
func resetPluginManager() {
	_pluginLock.Lock()
	defer _pluginLock.Unlock()

	_pluginMgr.insMap = make(map[string]map[string]map[string]Plugin)
	_factoryMap = make(map[string]Factory)
}

// ============================================================================
// Unit Tests
// ============================================================================

func TestPluginConfig_GetName(t *testing.T) {
	cfg := PluginConfig{}
	assert.Equal(t, "plugin", cfg.GetName())
}

func TestPluginConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  PluginConfig
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty config",
			config:  PluginConfig{},
			wantErr: true,
			errMsg:  "plugin config is empty",
		},
		{
			name: "empty factory config",
			config: PluginConfig{
				"db": {},
			},
			wantErr: true,
			errMsg:  "plugin type db has no factory config",
		},
		{
			name: "empty instance config",
			config: PluginConfig{
				"db": {
					"mysql": {},
				},
			},
			wantErr: true,
			errMsg:  "plugin db_mysql has no instance config",
		},
		{
			name: "valid config",
			config: PluginConfig{
				"db": {
					"mysql": {
						"host": "localhost",
						"port": 3306,
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRegisterPlugin(t *testing.T) {
	resetPluginManager()

	factory := newMockFactory(DB, "mysql")
	RegisterPlugin(factory)

	_pluginLock.RLock()
	defer _pluginLock.RUnlock()

	key := "db_mysql"
	assert.Contains(t, _factoryMap, key)
	assert.Equal(t, factory, _factoryMap[key])
}

func TestRegisterPlugin_Concurrent(t *testing.T) {
	resetPluginManager()

	var wg sync.WaitGroup
	factoryCount := 100

	// Concurrent registration (stress test for Linux multi-core environment)
	for i := 0; i < factoryCount; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			factory := newMockFactory(DB, fmt.Sprintf("test%d", idx))
			RegisterPlugin(factory)
		}(i)
	}

	wg.Wait()

	_pluginLock.RLock()
	defer _pluginLock.RUnlock()

	assert.Equal(t, factoryCount, len(_factoryMap))
}

func TestGetPluginFactory(t *testing.T) {
	resetPluginManager()

	factory := newMockFactory(DB, "mysql")
	RegisterPlugin(factory)

	// Test successful retrieval
	result := getPluginFactory("db", "mysql")
	assert.NotNil(t, result)
	assert.Equal(t, factory, result)

	// Test non-existent factory
	result = getPluginFactory("db", "nonexistent")
	assert.Nil(t, result)
}

func TestGetPluginNameFromCfg(t *testing.T) {
	tests := []struct {
		name     string
		config   map[string]any
		expected string
	}{
		{
			name:     "no tag",
			config:   map[string]any{"host": "localhost"},
			expected: DefaultInsName,
		},
		{
			name:     "with tag",
			config:   map[string]any{"tag": "master", "host": "localhost"},
			expected: "master",
		},
		{
			name:     "invalid tag type",
			config:   map[string]any{"tag": 123},
			expected: DefaultInsName,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getPluginNameFromCfg(tt.config)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetFactoryName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"mysql", "mysql"},
		{"mysql_master", "mysql"},
		{"redis_slave_1", "redis"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := getFactoryName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRegisterPluginIns(t *testing.T) {
	resetPluginManager()

	plugin := &mockPlugin{factoryName: "mysql"}

	// Test first registration
	err := registerPluginIns("db", "mysql", "master", plugin)
	require.NoError(t, err)

	// Test duplicate registration
	err = registerPluginIns("db", "mysql", "master", plugin)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "plugin master already exists")

	// Test different instance name
	err = registerPluginIns("db", "mysql", "slave", plugin)
	require.NoError(t, err)
}

func TestGetPlugin(t *testing.T) {
	resetPluginManager()

	plugin := &mockPlugin{factoryName: "mysql"}
	err := registerPluginIns("db", "mysql", "master", plugin)
	require.NoError(t, err)

	// Test successful retrieval
	result, err := GetPlugin("db", "mysql", "master")
	require.NoError(t, err)
	assert.Equal(t, plugin, result)

	// Test non-existent type
	_, err = GetPlugin("nonexistent", "mysql", "master")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "plugin type [nonexistent] not registered")

	// Test non-existent factory
	_, err = GetPlugin("db", "nonexistent", "master")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "plugin factory [db/nonexistent] not found")

	// Test non-existent instance
	_, err = GetPlugin("db", "mysql", "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "plugin instance [db/mysql/nonexistent] not found")
}

func TestGetDefaultPlugin(t *testing.T) {
	resetPluginManager()

	plugin := &mockPlugin{factoryName: "mysql"}
	err := registerPluginIns("db", "mysql", DefaultInsName, plugin)
	require.NoError(t, err)

	result, err := GetDefaultPlugin("db", "mysql")
	require.NoError(t, err)
	assert.Equal(t, plugin, result)
}

func TestMustGetPlugin(t *testing.T) {
	resetPluginManager()

	plugin := &mockPlugin{factoryName: "mysql"}
	err := registerPluginIns("db", "mysql", "master", plugin)
	require.NoError(t, err)

	// Test successful retrieval (should not panic)
	result := MustGetPlugin("db", "mysql", "master")
	assert.Equal(t, plugin, result)
}

func TestListPlugins(t *testing.T) {
	resetPluginManager()

	plugin1 := &mockPlugin{factoryName: "mysql"}
	plugin2 := &mockPlugin{factoryName: "mysql"}
	plugin3 := &mockPlugin{factoryName: "redis"}

	err := registerPluginIns("db", "mysql", "master", plugin1)
	require.NoError(t, err)
	err = registerPluginIns("db", "mysql", "slave", plugin2)
	require.NoError(t, err)
	err = registerPluginIns("db", "redis", DefaultInsName, plugin3)
	require.NoError(t, err)

	result := ListPlugins()

	assert.Len(t, result, 2)
	assert.ElementsMatch(t, []string{"master", "slave"}, result["db/mysql"])
	assert.ElementsMatch(t, []string{DefaultInsName}, result["db/redis"])
}

func TestValidatePluginConfig(t *testing.T) {
	tests := []struct {
		name       string
		pluginType Type
		config     map[string]any
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "empty config",
			pluginType: DB,
			config:     map[string]any{},
			wantErr:    true,
			errMsg:     "config is empty",
		},
		{
			name:       "db missing host",
			pluginType: DB,
			config:     map[string]any{"port": 3306},
			wantErr:    true,
			errMsg:     "missing required field: host",
		},
		{
			name:       "db missing port",
			pluginType: DB,
			config:     map[string]any{"host": "localhost"},
			wantErr:    true,
			errMsg:     "missing required field: port",
		},
		{
			name:       "db valid config",
			pluginType: DB,
			config:     map[string]any{"host": "localhost", "port": 3306},
			wantErr:    false,
		},
		{
			name:       "transport missing addr",
			pluginType: CSTransport,
			config:     map[string]any{"timeout": 30},
			wantErr:    true,
			errMsg:     "missing required field: addr",
		},
		{
			name:       "transport valid config",
			pluginType: CSTransport,
			config:     map[string]any{"addr": ":8080"},
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := newMockFactory(tt.pluginType, "test")
			err := validatePluginConfig(factory, tt.config)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRollbackPlugins(t *testing.T) {
	resetPluginManager()

	factory := newMockFactory(DB, "mysql")
	RegisterPlugin(factory)

	plugin1 := &mockPlugin{factoryName: "mysql"}
	plugin2 := &mockPlugin{factoryName: "mysql"}

	err := registerPluginIns("db", "mysql", "master", plugin1)
	require.NoError(t, err)
	err = registerPluginIns("db", "mysql", "slave", plugin2)
	require.NoError(t, err)

	plugins := []struct {
		ft, fn, pn string
		ins        Plugin
	}{
		{"db", "mysql", "master", plugin1},
		{"db", "mysql", "slave", plugin2},
	}

	rollbackPlugins(plugins)

	// Verify plugins are destroyed
	assert.Equal(t, 1, plugin1.GetDestroyCount())
	assert.Equal(t, 1, plugin2.GetDestroyCount())

	// Verify plugin manager is cleared
	_pluginLock.RLock()
	defer _pluginLock.RUnlock()
	assert.Empty(t, _pluginMgr.insMap)
}

func TestListAvailableFactories(t *testing.T) {
	resetPluginManager()

	// Register multiple factories
	factory1 := newMockFactory(DB, "mysql")
	factory2 := newMockFactory(DB, "postgres")
	factory3 := newMockFactory(DB, "redis")

	RegisterPlugin(factory1)
	RegisterPlugin(factory2)
	RegisterPlugin(factory3)

	// Test list DB factories
	result := listAvailableFactories("db")
	assert.Len(t, result, 3)
	assert.Contains(t, result, "mysql")
	assert.Contains(t, result, "postgres")
	assert.Contains(t, result, "redis")

	// Test list non-existent factory type
	result = listAvailableFactories("nonexistent")
	assert.Empty(t, result)

	// Test empty result after reset
	resetPluginManager()
	result = listAvailableFactories("db")
	assert.Empty(t, result)
}

// ============================================================================
// Concurrency Tests (Linux Multi-Core Environment)
// ============================================================================

func TestConcurrentPluginAccess(t *testing.T) {
	resetPluginManager()

	factory := newMockFactory(DB, "mysql")
	RegisterPlugin(factory)

	plugin := &mockPlugin{factoryName: "mysql"}
	err := registerPluginIns("db", "mysql", DefaultInsName, plugin)
	require.NoError(t, err)

	var wg sync.WaitGroup
	goroutineCount := 1000

	// Concurrent read access (stress test for RWMutex)
	for i := 0; i < goroutineCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p, err := GetDefaultPlugin("db", "mysql")
			assert.NoError(t, err)
			assert.NotNil(t, p)
		}()
	}

	wg.Wait()
}

func TestConcurrentPluginRegistration(t *testing.T) {
	resetPluginManager()

	var wg sync.WaitGroup
	pluginCount := 100

	// Concurrent plugin registration and retrieval
	for i := 0; i < pluginCount; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			factoryName := fmt.Sprintf("test%d", idx)
			factory := newMockFactory(DB, factoryName)
			RegisterPlugin(factory)

			plugin := &mockPlugin{factoryName: factoryName}
			err := registerPluginIns("db", factoryName, DefaultInsName, plugin)
			assert.NoError(t, err)

			// Immediately try to retrieve
			p, err := GetDefaultPlugin("db", factoryName)
			assert.NoError(t, err)
			assert.Equal(t, plugin, p)
		}(i)
	}

	wg.Wait()

	// Verify all plugins are registered
	result := ListPlugins()
	assert.Len(t, result, pluginCount)
}

// ============================================================================
// Performance Benchmark Tests (Linux Environment)
// ============================================================================

func BenchmarkGetPlugin(b *testing.B) {
	resetPluginManager()

	factory := newMockFactory(DB, "mysql")
	RegisterPlugin(factory)

	plugin := &mockPlugin{factoryName: "mysql"}
	_ = registerPluginIns("db", "mysql", DefaultInsName, plugin)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = GetPlugin("db", "mysql", DefaultInsName)
		}
	})
}

func BenchmarkGetDefaultPlugin(b *testing.B) {
	resetPluginManager()

	factory := newMockFactory(DB, "mysql")
	RegisterPlugin(factory)

	plugin := &mockPlugin{factoryName: "mysql"}
	_ = registerPluginIns("db", "mysql", DefaultInsName, plugin)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = GetDefaultPlugin("db", "mysql")
		}
	})
}

func BenchmarkRegisterPlugin(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resetPluginManager()
		factory := newMockFactory(DB, fmt.Sprintf("test%d", i))
		RegisterPlugin(factory)
	}
}

func BenchmarkListPlugins(b *testing.B) {
	resetPluginManager()

	// Setup 100 plugins
	for i := 0; i < 100; i++ {
		factory := newMockFactory(DB, fmt.Sprintf("test%d", i))
		RegisterPlugin(factory)
		plugin := &mockPlugin{factoryName: fmt.Sprintf("test%d", i)}
		_ = registerPluginIns("db", fmt.Sprintf("test%d", i), DefaultInsName, plugin)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ListPlugins()
	}
}
