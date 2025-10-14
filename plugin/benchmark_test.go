package plugin

import (
	"errors"
	"fmt"
	"testing"
)

func BenchmarkPluginManager_RegisterPlugin(b *testing.B) {
	pm := NewPluginManager()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		plugin := newMockPlugin(fmt.Sprintf("plugin-%d", i), "1.0.0", []string{})
		pm.RegisterPlugin(plugin)
	}
}

func BenchmarkPluginManager_GetPlugin(b *testing.B) {
	pm := NewPluginManager()

	// Register plugins first
	for i := 0; i < 1000; i++ {
		plugin := newMockPlugin(fmt.Sprintf("plugin-%d", i), "1.0.0", []string{})
		pm.RegisterPlugin(plugin)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pm.GetPlugin(fmt.Sprintf("plugin-%d", i%1000))
	}
}

func BenchmarkPluginManager_ListPlugins(b *testing.B) {
	pm := NewPluginManager()

	// Register plugins first
	for i := 0; i < 1000; i++ {
		plugin := newMockPlugin(fmt.Sprintf("plugin-%d", i), "1.0.0", []string{})
		pm.RegisterPlugin(plugin)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pm.ListPlugins()
	}
}

func BenchmarkPluginManager_StartAll(b *testing.B) {
	pm := NewPluginManager()

	// Register plugins first
	for i := 0; i < 100; i++ {
		plugin := newMockPlugin(fmt.Sprintf("plugin-%d", i), "1.0.0", []string{})
		pm.RegisterPlugin(plugin)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pm.StartAll()
		pm.StopAll()
	}
}

func BenchmarkPluginStatus_String(b *testing.B) {
	status := PluginStatusStarted

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = status.String()
	}
}

func BenchmarkNewPluginError(b *testing.B) {
	err := errors.New("test error")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewPluginError("test-plugin", "start", err)
	}
}

func BenchmarkPluginError_Error(b *testing.B) {
	err := errors.New("test error")
	pluginErr := NewPluginError("test-plugin", "start", err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pluginErr.Error()
	}
}

// Benchmark concurrent operations
func BenchmarkPluginManager_ConcurrentRegister(b *testing.B) {
	pm := NewPluginManager()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			plugin := newMockPlugin(fmt.Sprintf("plugin-%d", i), "1.0.0", []string{})
			pm.RegisterPlugin(plugin)
			i++
		}
	})
}

func BenchmarkPluginManager_ConcurrentGet(b *testing.B) {
	pm := NewPluginManager()

	// Register plugins first
	for i := 0; i < 1000; i++ {
		plugin := newMockPlugin(fmt.Sprintf("plugin-%d", i), "1.0.0", []string{})
		pm.RegisterPlugin(plugin)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			pm.GetPlugin(fmt.Sprintf("plugin-%d", i%1000))
			i++
		}
	})
}
