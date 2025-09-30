package config

import (
	"sync"
	"testing"
)

// TestSingletonInstance tests the singleton pattern implementation
func TestSingletonInstance(t *testing.T) {
	// Reset instance before test
	ResetInstance()

	// Test that GetInstance returns the same instance
	instance1 := GetInstance()
	instance2 := GetInstance()

	if instance1 != instance2 {
		t.Error("GetInstance() should return the same instance")
	}

	// Test that it's a valid ConfigManager
	if instance1 == nil {
		t.Error("GetInstance() should not return nil")
	}

	// Test thread safety
	var wg sync.WaitGroup
	instances := make([]ConfigManager, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			instances[index] = GetInstance()
		}(i)
	}

	wg.Wait()

	// All instances should be the same
	firstInstance := instances[0]
	for i, instance := range instances {
		if instance != firstInstance {
			t.Errorf("Instance at index %d is different from first instance", i)
		}
	}
}

// TestSetInstanceForTesting tests the testing support functions
func TestSetInstanceForTesting(t *testing.T) {
	// Reset instance before test
	ResetInstance()

	// Create a mock config manager for testing
	mockManager := &mockConfigManager{}

	// Set the testing instance
	SetInstanceForTesting(mockManager)

	// Verify GetInstance returns the mock instance
	instance := GetInstance()
	if instance != mockManager {
		t.Error("GetInstance() should return the testing instance")
	}

	// Reset instance
	ResetInstance()

	// Verify GetInstance returns a new instance after reset
	newInstance := GetInstance()
	if newInstance == mockManager {
		t.Error("GetInstance() should return a new instance after reset")
	}
	if newInstance == nil {
		t.Error("GetInstance() should not return nil after reset")
	}
}

// mockConfigManager is a mock implementation for testing
type mockConfigManager struct{}

func (m *mockConfigManager) LoadConfig(configName string, config Config) error {
	return nil
}

func (m *mockConfigManager) GetConfig(configName string) (Config, error) {
	return nil, nil
}

func (m *mockConfigManager) SetBasePath(path string) {}

func (m *mockConfigManager) SetEnvironment(env string) {}

func (m *mockConfigManager) Close() error {
	return nil
}

func (m *mockConfigManager) AddChangeListener(listener ConfigChangeListener) {}

func (m *mockConfigManager) RemoveChangeListener(listener ConfigChangeListener) {}

func (m *mockConfigManager) NotifyConfigChanged(configName string, newConfig, oldConfig Config) {}
