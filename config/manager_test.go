package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestConfig test configuration structure
type TestConfig struct {
	Name     string `mapstructure:"name"`
	Port     int    `mapstructure:"port"`
	Host     string `mapstructure:"host"`
	MaxConns int    `mapstructure:"maxConns"`
}

// TestChangeListener test configuration change listener
// This is used to track configuration changes in tests
type TestChangeListener struct {
	mu             sync.Mutex
	ChangeCount    int32
	LastConfig     Config
	LastOldConfig  Config
	LastConfigName string
}

func (c *TestConfig) GetName() string {
	return c.Name
}

func (c *TestConfig) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("name cannot be empty")
	}
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	if c.MaxConns <= 0 {
		return fmt.Errorf("maxConns must be positive")
	}
	// Additional validation for specific test scenarios
	if c.Port > 9000 && c.Name == "validation-server" {
		return fmt.Errorf("port %d exceeds maximum allowed value", c.Port)
	}
	return nil
}

// OnConfigChanged implements ConfigChangeListener interface
func (l *TestChangeListener) OnConfigChanged(configName string, newConfig, oldConfig Config) error {
	atomic.AddInt32(&l.ChangeCount, 1)

	l.mu.Lock()
	defer l.mu.Unlock()

	l.LastConfig = newConfig
	l.LastOldConfig = oldConfig
	l.LastConfigName = configName

	// Simple validation to ensure oldConfig is valid
	if oldConfig != nil {
		oldTestConfig, ok := oldConfig.(*TestConfig)
		if !ok {
			return fmt.Errorf("invalid old config type")
		}

		// Ensure name and port are consistent (used in TestAtomicConfigUpdate)
		var expectedPort int
		if n, err := fmt.Sscanf(oldTestConfig.Name, "atomic-server-%d", &expectedPort); n == 1 && err == nil {
			expectedPort += 8080
			if oldTestConfig.Port != expectedPort {
				return fmt.Errorf("config inconsistency in old value")
			}
		}
	}

	return nil
}

// TestNewConfigManager tests creating configuration manager
func TestNewConfigManager(t *testing.T) {
	cm := NewConfigManager()
	if cm == nil {
		t.Fatal("NewConfigManager() returned nil")
	}
}

// TestLoadConfig tests loading configuration
func TestLoadConfig(t *testing.T) {
	// Create temporary configuration file
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test.yaml")

	err := os.WriteFile(configFile, []byte(`
name: "test-server"
port: 8080
host: "localhost"
maxConns: 1000
`), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	cm := NewConfigManager()
	cm.SetBasePath(tmpDir)

	config := &TestConfig{}
	err = cm.LoadConfig("test", config)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if config.Name != "test-server" {
		t.Errorf("Expected name 'test-server', got '%s'", config.Name)
	}
	if config.Port != 8080 {
		t.Errorf("Expected port 8080, got %d", config.Port)
	}
	if config.MaxConns != 1000 {
		t.Errorf("Expected maxConns 1000, got %d", config.MaxConns)
	}
}

// TestGetConfig tests retrieving configuration
func TestGetConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "app.yaml")

	err := os.WriteFile(configFile, []byte(`
name: "app-server"
port: 9090
host: "127.0.0.1"
maxConns: 500
`), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	cm := NewConfigManager()
	cm.SetBasePath(tmpDir)

	config := &TestConfig{}
	err = cm.LoadConfig("app", config)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	retrievedConfig, err := cm.GetConfig("app")
	if err != nil {
		t.Fatalf("GetConfig failed: %v", err)
	}

	testConfig, ok := retrievedConfig.(*TestConfig)
	if !ok {
		t.Fatal("GetConfig returned wrong type")
	}

	if testConfig.Name != "app-server" {
		t.Errorf("Expected name 'app-server', got '%s'", testConfig.Name)
	}
}

// TestGetConfigNotFound tests retrieving non-existent configuration
func TestGetConfigNotFound(t *testing.T) {
	cm := NewConfigManager()

	_, err := cm.GetConfig("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent config, got nil")
	}
}

// TestConfigValidator tests configuration validation
func TestConfigValidator(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "invalid.yaml")

	// Create invalid configuration
	err := os.WriteFile(configFile, []byte(`
name: ""
port: 70000
host: "localhost"
maxConns: -100
`), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	cm := NewConfigManager()
	cm.SetBasePath(tmpDir)

	config := &TestConfig{}
	err = cm.LoadConfig("invalid", config)
	if err == nil {
		t.Error("Expected validation error, got nil")
	}
}

// TestConfigChangeListener tests configuration change notification mechanism
func TestConfigChangeListener(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "hook.yaml")

	err := os.WriteFile(configFile, []byte(`
name: "hook-server"
port: 8080
host: "localhost"
maxConns: 100
`), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	cm := NewConfigManager()
	cm.SetBasePath(tmpDir)

	// Create and register a change listener
	listener := &TestChangeListener{}
	cm.AddChangeListener(listener)

	config := &TestConfig{}
	err = cm.LoadConfig("hook", config)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Update configuration file to trigger config change
	err = os.WriteFile(configFile, []byte(`
name: "hook-server-updated"
port: 9090
host: "localhost"
maxConns: 200
`), 0644)
	if err != nil {
		t.Fatalf("Failed to update config file: %v", err)
	}

	// Wait for file change detection and config reload
	// Increase wait time for Windows file system
	time.Sleep(2 * time.Second)

	// Check if the listener was notified
	if atomic.LoadInt32(&listener.ChangeCount) != 1 {
		t.Errorf("Expected ChangeCount 1, got %d", atomic.LoadInt32(&listener.ChangeCount))
	}

	// Check listener received the correct configuration data
	listener.mu.Lock()
	defer listener.mu.Unlock()
	if listener.LastConfigName != "hook" {
		t.Errorf("Expected LastConfigName 'hook', got '%s'", listener.LastConfigName)
	}

	// Verify the last old config and new config
	if listener.LastOldConfig == nil || listener.LastConfig == nil {
		t.Error("Listener did not receive config objects")
	}

	// Test removing the listener
	cm.RemoveChangeListener(listener)

	// Update configuration file again
	err = os.WriteFile(configFile, []byte(`
name: "hook-server-final"
port: 9191
host: "localhost"
maxConns: 300
`), 0644)
	if err != nil {
		t.Fatalf("Failed to update config file: %v", err)
	}

	// Wait for file change detection
	time.Sleep(2 * time.Second)

	// Verify listener was not notified after removal
	if atomic.LoadInt32(&listener.ChangeCount) != 1 {
		t.Errorf("Expected ChangeCount to remain 1 after listener removal, got %d", atomic.LoadInt32(&listener.ChangeCount))
	}
}

// TestEnvironmentConfig tests environment-specific configuration
func TestEnvironmentConfig(t *testing.T) {
	tmpDir := t.TempDir()
	envDir := filepath.Join(tmpDir, "production")
	os.MkdirAll(envDir, 0755)

	// Create environment-specific configuration
	configFile := filepath.Join(envDir, "env.yaml")
	err := os.WriteFile(configFile, []byte(`
name: "production-server"
port: 80
host: "prod.example.com"
maxConns: 10000
`), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	cm := NewConfigManager()
	cm.SetBasePath(tmpDir)
	cm.SetEnvironment("production")

	config := &TestConfig{}
	err = cm.LoadConfig("env", config)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if config.Name != "production-server" {
		t.Errorf("Expected name 'production-server', got '%s'", config.Name)
	}
	if config.Port != 80 {
		t.Errorf("Expected port 80, got %d", config.Port)
	}
}

// TestConfigManagerProvider tests configuration manager provider
func TestConfigManagerProvider(t *testing.T) {
	cm := NewConfigManager()
	provider := NewConfigManagerProvider(cm)

	retrievedCM := provider.GetConfigManager()
	if retrievedCM != cm {
		t.Error("ConfigManagerProvider returned different manager")
	}

	// Test setting new manager
	newCM := NewConfigManager()
	provider.SetConfigManager(newCM)

	if provider.GetConfigManager() != newCM {
		t.Error("SetConfigManager did not update the manager")
	}
}

// TestClose tests closing configuration manager
func TestClose(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "close.yaml")

	err := os.WriteFile(configFile, []byte(`
name: "close-server"
port: 8080
host: "localhost"
maxConns: 100
`), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	cm := NewConfigManager()
	cm.SetBasePath(tmpDir)

	config := &TestConfig{}
	err = cm.LoadConfig("close", config)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	err = cm.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

// TestConcurrentLoadConfig tests concurrent configuration loading
func TestConcurrentLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "concurrent.yaml")

	err := os.WriteFile(configFile, []byte(`
name: "concurrent-server"
port: 8080
host: "localhost"
maxConns: 100
`), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	cm := NewConfigManager()
	cm.SetBasePath(tmpDir)

	// Test concurrent loading of same config
	var wg sync.WaitGroup
	errors := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			config := &TestConfig{}
			err := cm.LoadConfig("concurrent", config)
			if err != nil {
				errors <- fmt.Errorf("goroutine %d: %v", id, err)
			}

			// Verify loaded config
			if config.Name != "concurrent-server" {
				errors <- fmt.Errorf("goroutine %d: expected name 'concurrent-server', got '%s'", id, config.Name)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for any errors
	for err := range errors {
		t.Error(err)
	}
}

// TestConcurrentGetConfig tests concurrent configuration retrieval
func TestConcurrentGetConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "concurrent-get.yaml")

	err := os.WriteFile(configFile, []byte(`
name: "concurrent-get-server"
port: 8080
host: "localhost"
maxConns: 100
`), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	cm := NewConfigManager()
	cm.SetBasePath(tmpDir)

	config := &TestConfig{}
	err = cm.LoadConfig("concurrent-get", config)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Test concurrent retrieval
	var wg sync.WaitGroup
	errors := make(chan error, 20)

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			retrievedConfig, err := cm.GetConfig("concurrent-get")
			if err != nil {
				errors <- fmt.Errorf("goroutine %d: %v", id, err)
				return
			}

			testConfig, ok := retrievedConfig.(*TestConfig)
			if !ok {
				errors <- fmt.Errorf("goroutine %d: wrong config type", id)
				return
			}

			if testConfig.Name != "concurrent-get-server" {
				errors <- fmt.Errorf("goroutine %d: expected name 'concurrent-get-server', got '%s'", id, testConfig.Name)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}
}

// TestConcurrentReload tests concurrent configuration reloading
func TestConcurrentReload(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "reload.yaml")

	// Initial config
	err := os.WriteFile(configFile, []byte(`
name: "reload-server"
port: 8080
host: "localhost"
maxConns: 100
`), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	cm := NewConfigManager()
	cm.SetBasePath(tmpDir)

	config := &TestConfig{}
	err = cm.LoadConfig("reload", config)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Test concurrent reload and access
	var wg sync.WaitGroup
	errors := make(chan error, 30)
	reloadCount := 0
	var reloadMutex sync.Mutex

	// Start goroutines that continuously access config
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < 10; j++ {
				retrievedConfig, err := cm.GetConfig("reload")
				if err != nil {
					errors <- fmt.Errorf("access goroutine %d-%d: %v", id, j, err)
					continue
				}

				testConfig, ok := retrievedConfig.(*TestConfig)
				if !ok {
					errors <- fmt.Errorf("access goroutine %d-%d: wrong config type", id, j)
					continue
				}

				// Config should be valid even during reload
				if testConfig.Name == "" {
					errors <- fmt.Errorf("access goroutine %d-%d: empty config name", id, j)
				}

				time.Sleep(time.Millisecond * 10) // Small delay to increase concurrency
			}
		}(i)
	}

	// Start reload goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()

		for i := 0; i < 5; i++ {
			time.Sleep(time.Millisecond * 50)

			reloadMutex.Lock()
			reloadCount++
			newPort := 8080 + reloadCount
			reloadMutex.Unlock()

			// Update config file
			err := os.WriteFile(configFile, []byte(fmt.Sprintf(`
name: "reload-server-%d"
port: %d
host: "localhost"
maxConns: %d
`, reloadCount, newPort, 100+reloadCount*10)), 0644)
			if err != nil {
				errors <- fmt.Errorf("reload %d: %v", i, err)
			}

			time.Sleep(time.Millisecond * 100) // Wait for file change detection
		}
	}()

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}

	// Verify final config state
	finalConfig, err := cm.GetConfig("reload")
	if err != nil {
		t.Fatalf("Failed to get final config: %v", err)
	}

	testConfig, ok := finalConfig.(*TestConfig)
	if !ok {
		t.Fatal("Final config has wrong type")
	}

	if testConfig.Port != 8085 {
		t.Errorf("Expected final port 8085, got %d", testConfig.Port)
	}
}

// TestHighFrequencyConcurrentReload tests high-frequency concurrent reload scenarios
func TestHighFrequencyConcurrentReload(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "highfreq.yaml")

	// Initial config
	err := os.WriteFile(configFile, []byte(`
name: "highfreq-server"
port: 8080
host: "localhost"
maxConns: 100
`), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	cm := NewConfigManager()
	cm.SetBasePath(tmpDir)

	config := &TestConfig{}
	err = cm.LoadConfig("highfreq", config)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// High-frequency concurrent reload test
	var wg sync.WaitGroup
	errors := make(chan error, 100)
	reloadCount := int32(0)
	accessCount := int32(0)

	// Start 20 goroutines that continuously access config
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < 50; j++ {
				retrievedConfig, err := cm.GetConfig("highfreq")
				if err != nil {
					errors <- fmt.Errorf("access goroutine %d-%d: %v", id, j, err)
					continue
				}

				testConfig, ok := retrievedConfig.(*TestConfig)
				if !ok {
					errors <- fmt.Errorf("access goroutine %d-%d: wrong config type", id, j)
					continue
				}

				// Verify config consistency during high-frequency reload
				if testConfig.Name == "" || testConfig.Port <= 0 {
					errors <- fmt.Errorf("access goroutine %d-%d: invalid config values", id, j)
				}

				atomic.AddInt32(&accessCount, 1)
				time.Sleep(time.Millisecond * 5) // Very short delay for high frequency
			}
		}(i)
	}

	// Start 5 goroutines that trigger rapid reloads
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < 20; j++ {
				count := atomic.AddInt32(&reloadCount, 1)
				newPort := 8080 + int(count)

				// Update config file rapidly
				err := os.WriteFile(configFile, []byte(fmt.Sprintf(`
name: "highfreq-server-%d-%d"
port: %d
host: "localhost"
maxConns: %d
`, id, j, newPort, 100+j)), 0644)
				if err != nil {
					errors <- fmt.Errorf("reload goroutine %d-%d: %v", id, j, err)
				}

				time.Sleep(time.Millisecond * 10) // Very short delay for high frequency
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for any errors
	errorCount := 0
	for err := range errors {
		t.Error(err)
		errorCount++
	}

	// Verify that we had significant concurrent activity
	if atomic.LoadInt32(&accessCount) < 500 {
		t.Errorf("Expected at least 500 access operations, got %d", atomic.LoadInt32(&accessCount))
	}

	if atomic.LoadInt32(&reloadCount) < 50 {
		t.Errorf("Expected at least 50 reload operations, got %d", atomic.LoadInt32(&reloadCount))
	}

	t.Logf("High-frequency test completed: %d accesses, %d reloads, %d errors",
		atomic.LoadInt32(&accessCount), atomic.LoadInt32(&reloadCount), errorCount)
}

// TestConcurrentReloadWithValidation tests concurrent reload with validation
func TestConcurrentReloadWithValidation(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "validation.yaml")

	// Initial valid config
	err := os.WriteFile(configFile, []byte(`
name: "validation-server"
port: 8080
host: "localhost"
maxConns: 100
`), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	cm := NewConfigManager()
	cm.SetBasePath(tmpDir)

	config := &TestConfig{}
	err = cm.LoadConfig("validation", config)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Test concurrent reload with validation
	var wg sync.WaitGroup
	errors := make(chan error, 20)

	// Start goroutines that trigger reloads with different port values
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < 5; j++ {
				port := 8080 + id*100 + j*10

				// Some ports will trigger validation errors
				err := os.WriteFile(configFile, []byte(fmt.Sprintf(`
name: "validation-server-%d-%d"
port: %d
host: "localhost"
maxConns: %d
`, id, j, port, 100+j)), 0644)
				if err != nil {
					errors <- fmt.Errorf("reload goroutine %d-%d: %v", id, j, err)
				}

				time.Sleep(time.Millisecond * 50)
			}
		}(i)
	}

	// Start goroutines that continuously access config
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < 10; j++ {
				retrievedConfig, err := cm.GetConfig("validation")
				if err != nil {
					errors <- fmt.Errorf("access goroutine %d-%d: %v", id, j, err)
					continue
				}

				testConfig, ok := retrievedConfig.(*TestConfig)
				if !ok {
					errors <- fmt.Errorf("access goroutine %d-%d: wrong config type", id, j)
					continue
				}

				// Config should remain valid even when some reloads fail validation
				if testConfig.Name == "" || testConfig.Port <= 0 {
					errors <- fmt.Errorf("access goroutine %d-%d: invalid config during validation failures", id, j)
				}

				time.Sleep(time.Millisecond * 30)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Error(err)
	}

	// Final config should be valid
	finalConfig, err := cm.GetConfig("validation")
	if err != nil {
		t.Fatalf("Failed to get final config: %v", err)
	}

	testConfig, ok := finalConfig.(*TestConfig)
	if !ok {
		t.Fatal("Final config has wrong type")
	}

	if testConfig.Port > 9000 {
		t.Errorf("Final config should not have invalid port %d", testConfig.Port)
	}
}

// TestAtomicConfigUpdate tests atomicity of configuration updates during concurrent reload
func TestAtomicConfigUpdate(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "atomic.yaml")

	// Initial config
	err := os.WriteFile(configFile, []byte(`
name: "atomic-server"
port: 8080
host: "localhost"
maxConns: 100
`), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	cm := NewConfigManager()
	cm.SetBasePath(tmpDir)

	config := &TestConfig{}
	err = cm.LoadConfig("atomic", config)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Test atomicity: config should never be in inconsistent state
	var wg sync.WaitGroup
	errors := make(chan error, 50)
	consistentReads := int32(0)
	totalReads := int32(0)

	// Start goroutine that rapidly updates config
	wg.Add(1)
	go func() {
		defer wg.Done()

		for i := 0; i < 20; i++ {
			// Update name and port together
			err := os.WriteFile(configFile, []byte(fmt.Sprintf(`
name: "atomic-server-%d"
port: %d
host: "localhost"
maxConns: %d
`, i, 8080+i, 100+i)), 0644)
			if err != nil {
				errors <- fmt.Errorf("update %d: %v", i, err)
			}
			time.Sleep(time.Millisecond * 25)
		}
	}()

	// Start multiple goroutines that verify config consistency
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < 30; j++ {
				atomic.AddInt32(&totalReads, 1)
				retrievedConfig, err := cm.GetConfig("atomic")
				if err != nil {
					errors <- fmt.Errorf("consistency goroutine %d-%d: %v", id, j, err)
					continue
				}

				testConfig, ok := retrievedConfig.(*TestConfig)
				if !ok {
					errors <- fmt.Errorf("consistency goroutine %d-%d: wrong config type", id, j)
					continue
				}

				// Verify that name and port are consistent
				// If name contains a number, port should match that number + 8080
				var expectedPort int
				if n, err := fmt.Sscanf(testConfig.Name, "atomic-server-%d", &expectedPort); n == 1 && err == nil {
					expectedPort += 8080
					if testConfig.Port != expectedPort {
						errors <- fmt.Errorf("consistency goroutine %d-%d: config inconsistency - name %s suggests port %d but got %d",
							id, j, testConfig.Name, expectedPort, testConfig.Port)
					} else {
						atomic.AddInt32(&consistentReads, 1)
					}
				} else if testConfig.Name == "atomic-server" && testConfig.Port != 8080 {
					// If name doesn't contain number, it's the initial config (port 8080)
					errors <- fmt.Errorf("consistency goroutine %d-%d: initial config inconsistency", id, j)
				} else {
					atomic.AddInt32(&consistentReads, 1)
				}

				time.Sleep(time.Millisecond * 10)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	errorCount := 0
	for err := range errors {
		t.Error(err)
		errorCount++
	}

	// Verify high consistency rate (should be 100% with proper locking)
	consistencyRate := float32(atomic.LoadInt32(&consistentReads)) / float32(atomic.LoadInt32(&totalReads))
	if consistencyRate < 0.95 {
		t.Errorf("Config consistency rate too low: %.2f%% (expected >95%%)", consistencyRate*100)
	}

	t.Logf("Atomicity test: %d/%d consistent reads (%.2f%%)",
		atomic.LoadInt32(&consistentReads), atomic.LoadInt32(&totalReads), consistencyRate*100)
}

// TestRaceConditionDetection tests for potential race conditions
func TestRaceConditionDetection(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "race.yaml")

	err := os.WriteFile(configFile, []byte(`
name: "race-server"
port: 8080
host: "localhost"
maxConns: 100
`), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	cm := NewConfigManager()
	cm.SetBasePath(tmpDir)

	// This test is designed to be run with -race flag
	// It creates a scenario where multiple goroutines access and modify config
	var wg sync.WaitGroup
	config := &TestConfig{}

	// Load config first
	err = cm.LoadConfig("race", config)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Start multiple goroutines that access config
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < 100; j++ {
				_, err := cm.GetConfig("race")
				if err != nil {
					t.Errorf("Goroutine %d access %d: %v", id, j, err)
				}
			}
		}(i)
	}

	// Start goroutine that triggers reloads
	wg.Add(1)
	go func() {
		defer wg.Done()

		for i := 0; i < 3; i++ {
			time.Sleep(time.Millisecond * 10)

			// Update config file to trigger reload
			err := os.WriteFile(configFile, []byte(fmt.Sprintf(`
name: "race-server-%d"
port: %d
host: "localhost"
maxConns: %d
`, i, 8080+i, 100+i*10)), 0644)
			if err != nil {
				t.Errorf("Reload %d: %v", i, err)
			}
		}
	}()

	wg.Wait()
}

// TestConfigManagerThreadSafety tests thread safety of config manager operations
func TestConfigManagerThreadSafety(t *testing.T) {
	cm := NewConfigManager()

	// Test concurrent manager operations
	var wg sync.WaitGroup
	errors := make(chan error, 50)

	// Multiple goroutines performing different operations
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Set base path
			cm.SetBasePath(fmt.Sprintf("/tmp/test-%d", id))

			// Set environment
			cm.SetEnvironment(fmt.Sprintf("env-%d", id))

			// These operations should be thread-safe
			time.Sleep(time.Millisecond * 5)
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}
}

// TestFileWatcherStability tests the stability of file watcher under various conditions
func TestFileWatcherStability(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "stability.yaml")

	// Initial config
	err := os.WriteFile(configFile, []byte(`
name: "stability-server"
port: 8080
host: "localhost"
maxConns: 100
`), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	cm := NewConfigManager()
	cm.SetBasePath(tmpDir)

	config := &TestConfig{}
	err = cm.LoadConfig("stability", config)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Test rapid file modifications to stress test the watcher
	var wg sync.WaitGroup
	errors := make(chan error, 100)
	reloadCount := int32(0)

	// Start goroutine that rapidly modifies the file
	wg.Add(1)
	go func() {
		defer wg.Done()

		for i := 0; i < 50; i++ {
			// Rapidly write to file to stress test watcher
			err := os.WriteFile(configFile, []byte(fmt.Sprintf(`
name: "stability-server-%d"
port: %d
host: "localhost"
maxConns: %d
`, i, 8080+i, 100+i)), 0644)
			if err != nil {
				errors <- fmt.Errorf("rapid write %d: %v", i, err)
			}
			atomic.AddInt32(&reloadCount, 1)
			time.Sleep(time.Millisecond * 5) // Very short delay for stress test
		}
	}()

	// Start goroutine that continuously accesses config
	wg.Add(1)
	go func() {
		defer wg.Done()

		for i := 0; i < 100; i++ {
			_, err := cm.GetConfig("stability")
			if err != nil {
				errors <- fmt.Errorf("access %d: %v", i, err)
			}
			time.Sleep(time.Millisecond * 10)
		}
	}()

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Error(err)
	}

	t.Logf("File watcher stability test completed: %d reloads attempted", atomic.LoadInt32(&reloadCount))
}

// TestFileDeleteAndRecreate tests handling of file deletion and recreation
func TestFileDeleteAndRecreate(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "delete-recreate.yaml")

	// Initial config
	err := os.WriteFile(configFile, []byte(`
name: "delete-recreate-server"
port: 8080
host: "localhost"
maxConns: 100
`), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	cm := NewConfigManager()
	cm.SetBasePath(tmpDir)

	config := &TestConfig{}
	err = cm.LoadConfig("delete-recreate", config)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Delete the file
	err = os.Remove(configFile)
	if err != nil {
		t.Fatalf("Failed to delete config file: %v", err)
	}

	// Wait a bit for watcher to detect deletion
	time.Sleep(200 * time.Millisecond)

	// Recreate the file with different content
	err = os.WriteFile(configFile, []byte(`
name: "recreated-server"
port: 9090
host: "localhost"
maxConns: 200
`), 0644)
	if err != nil {
		t.Fatalf("Failed to recreate config file: %v", err)
	}

	// Wait for potential reload
	time.Sleep(200 * time.Millisecond)

	// Config should still be accessible with original values (no reload due to deletion)
	retrievedConfig, err := cm.GetConfig("delete-recreate")
	if err != nil {
		t.Fatalf("GetConfig failed after file recreation: %v", err)
	}

	testConfig, ok := retrievedConfig.(*TestConfig)
	if !ok {
		t.Fatal("Retrieved config has wrong type")
	}

	// Should still have original values since file was deleted and recreated
	if testConfig.Name != "delete-recreate-server" {
		t.Errorf("Expected original name 'delete-recreate-server', got '%s'", testConfig.Name)
	}
}

// TestFileRename tests handling of file renaming
func TestFileRename(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "rename.yaml")
	newConfigFile := filepath.Join(tmpDir, "renamed.yaml")

	// Initial config
	err := os.WriteFile(configFile, []byte(`
name: "rename-server"
port: 8080
host: "localhost"
maxConns: 100
`), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	cm := NewConfigManager()
	cm.SetBasePath(tmpDir)

	config := &TestConfig{}
	err = cm.LoadConfig("rename", config)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Rename the file
	err = os.Rename(configFile, newConfigFile)
	if err != nil {
		t.Fatalf("Failed to rename config file: %v", err)
	}

	// Wait for watcher to detect rename
	time.Sleep(200 * time.Millisecond)

	// Config should still be accessible
	retrievedConfig, err := cm.GetConfig("rename")
	if err != nil {
		t.Fatalf("GetConfig failed after file rename: %v", err)
	}

	testConfig, ok := retrievedConfig.(*TestConfig)
	if !ok {
		t.Fatal("Retrieved config has wrong type")
	}

	if testConfig.Name != "rename-server" {
		t.Errorf("Expected name 'rename-server', got '%s'", testConfig.Name)
	}
}

// TestMultipleConfigFiles tests concurrent monitoring of multiple config files
func TestMultipleConfigFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create multiple config files
	configFiles := []string{"server1.yaml", "server2.yaml", "server3.yaml"}
	configNames := []string{"server1", "server2", "server3"}

	cm := NewConfigManager()
	cm.SetBasePath(tmpDir)

	// Load all configs
	for i, fileName := range configFiles {
		configFile := filepath.Join(tmpDir, fileName)

		err := os.WriteFile(configFile, []byte(fmt.Sprintf(`
name: "%s"
port: %d
host: "localhost"
maxConns: %d
`, configNames[i], 8080+i, 100+i*10)), 0644)
		if err != nil {
			t.Fatalf("Failed to create config file %s: %v", fileName, err)
		}

		config := &TestConfig{}
		err = cm.LoadConfig(configNames[i], config)
		if err != nil {
			t.Fatalf("LoadConfig failed for %s: %v", configNames[i], err)
		}
	}

	// Test concurrent modifications to multiple config files
	var wg sync.WaitGroup
	errors := make(chan error, 30)

	// Modify each config file concurrently
	for i, fileName := range configFiles {
		wg.Add(1)
		go func(id int, name string) {
			defer wg.Done()

			configFile := filepath.Join(tmpDir, fileName)
			for j := 0; j < 3; j++ {
				err := os.WriteFile(configFile, []byte(fmt.Sprintf(`
name: "%s-updated-%d"
port: %d
host: "localhost"
maxConns: %d
`, name, j, 8080+id+j, 100+id*10+j)), 0644)
				if err != nil {
					errors <- fmt.Errorf("modify %s-%d: %v", name, j, err)
				}
				time.Sleep(100 * time.Millisecond)
			}
		}(i, configNames[i])
	}

	// Concurrently access all configs
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < 10; j++ {
				for _, name := range configNames {
					_, err := cm.GetConfig(name)
					if err != nil {
						errors <- fmt.Errorf("access %s-%d-%d: %v", name, id, j, err)
					}
				}
				time.Sleep(20 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}

	// Verify final states
	for _, name := range configNames {
		retrievedConfig, err := cm.GetConfig(name)
		if err != nil {
			t.Errorf("GetConfig failed for %s: %v", name, err)
			continue
		}

		testConfig, ok := retrievedConfig.(*TestConfig)
		if !ok {
			t.Errorf("Retrieved config for %s has wrong type", name)
			continue
		}

		if testConfig.Name == "" {
			t.Errorf("Config for %s has empty name", name)
		}
	}
}

// TestConfigReloadErrorHandling tests error handling during config reload
func TestConfigReloadErrorHandling(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "error-handling.yaml")

	// Initial valid config
	err := os.WriteFile(configFile, []byte(`
name: "error-server"
port: 8080
host: "localhost"
maxConns: 100
`), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	cm := NewConfigManager()
	cm.SetBasePath(tmpDir)

	config := &TestConfig{}
	err = cm.LoadConfig("error-handling", config)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Write invalid YAML to trigger reload error
	err = os.WriteFile(configFile, []byte(`
name: "error-server"
port: invalid-port  # Invalid YAML
host: "localhost"
maxConns: 100
`), 0644)
	if err != nil {
		t.Fatalf("Failed to write invalid config: %v", err)
	}

	// Wait for reload attempt
	time.Sleep(200 * time.Millisecond)

	// Config should still be accessible with original values
	retrievedConfig, err := cm.GetConfig("error-handling")
	if err != nil {
		t.Fatalf("GetConfig failed after invalid reload: %v", err)
	}

	testConfig, ok := retrievedConfig.(*TestConfig)
	if !ok {
		t.Fatal("Retrieved config has wrong type")
	}

	// Should still have original valid values
	if testConfig.Name != "error-server" {
		t.Errorf("Expected name 'error-server', got '%s'", testConfig.Name)
	}
	if testConfig.Port != 8080 {
		t.Errorf("Expected port 8080, got %d", testConfig.Port)
	}
}

// TestConfigReloadWithPartialUpdates tests reload behavior with partial file updates
func TestConfigReloadWithPartialUpdates(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "partial.yaml")

	// Initial config
	err := os.WriteFile(configFile, []byte(`
name: "partial-server"
port: 8080
host: "localhost"
maxConns: 100
`), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	cm := NewConfigManager()
	cm.SetBasePath(tmpDir)

	// Create and register a change listener
	listener := &TestChangeListener{}
	cm.AddChangeListener(listener)

	config := &TestConfig{}
	err = cm.LoadConfig("partial", config)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Perform partial file updates (simulating editor saves)
	for i := 0; i < 5; i++ {
		// Write partial content (simulating incomplete save)
		partialContent := fmt.Sprintf(`
name: "partial-server-%d"
port: %d
`, i, 8080+i)
		err = os.WriteFile(configFile, []byte(partialContent), 0644)
		if err != nil {
			t.Fatalf("Failed to write partial config: %v", err)
		}
		time.Sleep(50 * time.Millisecond)

		// Write complete content
		completeContent := fmt.Sprintf(`
name: "partial-server-%d"
port: %d
host: "localhost"
maxConns: %d
`, i, 8080+i, 100+i)
		err = os.WriteFile(configFile, []byte(completeContent), 0644)
		if err != nil {
			t.Fatalf("Failed to write complete config: %v", err)
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Wait for all reloads to complete
	time.Sleep(500 * time.Millisecond)

	// Check if the listener was notified multiple times
	t.Logf("Partial update test: %d reloads detected", atomic.LoadInt32(&listener.ChangeCount))

	// Final config should be valid
	finalConfig, err3 := cm.GetConfig("partial")
	if err3 != nil {
		t.Fatalf("GetConfig failed: %v", err3)
	}

	finalTestConfig, ok := finalConfig.(*TestConfig)
	if !ok {
		t.Fatal("Final config has wrong type")
	}

	if finalTestConfig.Port != 8084 {
		t.Errorf("Expected final port 8084, got %d", finalTestConfig.Port)
	}

	// Check last config name via listener
	listener.mu.Lock()
	defer listener.mu.Unlock()
	if listener.LastConfigName != "partial" {
		t.Errorf("Expected LastConfigName 'partial', got '%s'", listener.LastConfigName)
	}
}
