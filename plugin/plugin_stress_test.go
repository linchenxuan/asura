package plugin

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Stress Test Configuration (Linux Game Server Scenarios)
// ============================================================================

const (
	// High concurrency scenarios (simulating game server load)
	StressGoroutineCount     = 10000  // 10K concurrent goroutines
	StressPluginCount        = 1000   // 1K plugin instances
	StressOperationCount     = 100000 // 100K operations
	StressHotReloadCount     = 10000  // 10K hot reload operations
	StressDuration           = 10 * time.Second
	StressMemoryAllocCount   = 50000 // 50K memory allocations
	StressConnectionPoolSize = 5000  // 5K connection pool size
)

// ============================================================================
// Stress Test Helper Functions
// ============================================================================

// stressTestMetrics collects stress test metrics
type stressTestMetrics struct {
	totalOps       int64
	successOps     int64
	failedOps      int64
	totalLatency   int64 // nanoseconds
	maxLatency     int64 // nanoseconds
	minLatency     int64 // nanoseconds
	startTime      time.Time
	endTime        time.Time
	goroutineCount int
	memAllocBefore uint64
	memAllocAfter  uint64
	mu             sync.Mutex
}

func newStressTestMetrics() *stressTestMetrics {
	return &stressTestMetrics{
		minLatency: int64(^uint64(0) >> 1), // Max int64
		startTime:  time.Now(),
	}
}

func (m *stressTestMetrics) recordOp(latency time.Duration, success bool) {
	atomic.AddInt64(&m.totalOps, 1)
	if success {
		atomic.AddInt64(&m.successOps, 1)
	} else {
		atomic.AddInt64(&m.failedOps, 1)
	}

	latencyNs := latency.Nanoseconds()
	atomic.AddInt64(&m.totalLatency, latencyNs)

	// Update max latency
	for {
		oldMax := atomic.LoadInt64(&m.maxLatency)
		if latencyNs <= oldMax {
			break
		}
		if atomic.CompareAndSwapInt64(&m.maxLatency, oldMax, latencyNs) {
			break
		}
	}

	// Update min latency
	for {
		oldMin := atomic.LoadInt64(&m.minLatency)
		if latencyNs >= oldMin {
			break
		}
		if atomic.CompareAndSwapInt64(&m.minLatency, oldMin, latencyNs) {
			break
		}
	}
}

func (m *stressTestMetrics) finish() {
	m.endTime = time.Now()
	m.goroutineCount = runtime.NumGoroutine()

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	m.memAllocAfter = memStats.Alloc
}

func (m *stressTestMetrics) report(t *testing.T) {
	duration := m.endTime.Sub(m.startTime)
	avgLatency := time.Duration(m.totalLatency / m.totalOps)
	qps := float64(m.totalOps) / duration.Seconds()
	memUsed := float64(m.memAllocAfter-m.memAllocBefore) / 1024 / 1024 // MB

	t.Logf("\n"+
		"========== Stress Test Report (Linux Game Server) ==========\n"+
		"Duration:          %v\n"+
		"Total Operations:  %d\n"+
		"Success:           %d (%.2f%%)\n"+
		"Failed:            %d (%.2f%%)\n"+
		"QPS:               %.2f ops/sec\n"+
		"Avg Latency:       %v\n"+
		"Min Latency:       %v\n"+
		"Max Latency:       %v\n"+
		"Goroutines:        %d\n"+
		"Memory Used:       %.2f MB\n"+
		"============================================================\n",
		duration,
		m.totalOps,
		m.successOps, float64(m.successOps)/float64(m.totalOps)*100,
		m.failedOps, float64(m.failedOps)/float64(m.totalOps)*100,
		qps,
		avgLatency,
		time.Duration(m.minLatency),
		time.Duration(m.maxLatency),
		m.goroutineCount,
		memUsed,
	)
}

// ============================================================================
// Stress Test 1: High Concurrency Plugin Access (Read-Heavy)
// ============================================================================

func TestStress_HighConcurrencyPluginAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	resetPluginManager()
	metrics := newStressTestMetrics()

	// Setup plugins
	factory := newMockFactory(DB, "mysql")
	RegisterPlugin(factory)
	plugin := &mockPlugin{factoryName: "mysql"}
	err := registerPluginIns("db", "mysql", DefaultInsName, plugin)
	require.NoError(t, err)

	// Record initial memory
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	metrics.memAllocBefore = memStats.Alloc

	// Start stress test
	var wg sync.WaitGroup
	startSignal := make(chan struct{})

	// Simulate 10K concurrent players accessing plugin
	for i := 0; i < StressGoroutineCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startSignal // Wait for start signal

			for j := 0; j < 10; j++ {
				start := time.Now()
				_, err := GetDefaultPlugin("db", "mysql")
				latency := time.Since(start)
				metrics.recordOp(latency, err == nil)
			}
		}()
	}

	// Start all goroutines simultaneously
	close(startSignal)
	wg.Wait()

	metrics.finish()
	metrics.report(t)

	// Assertions (Linux game server requirements)
	assert.Greater(t, metrics.successOps, int64(0))
	assert.Equal(t, int64(0), metrics.failedOps, "Should have zero failures")
	assert.Less(t, time.Duration(metrics.maxLatency), 10*time.Millisecond, "Max latency should < 10ms")
	assert.Less(t, time.Duration(metrics.totalLatency/metrics.totalOps), 1*time.Millisecond, "Avg latency should < 1ms")
}

// ============================================================================
// Stress Test 2: High Concurrency Plugin Registration (Write-Heavy)
// ============================================================================

func TestStress_HighConcurrencyPluginRegistration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	resetPluginManager()
	metrics := newStressTestMetrics()

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	metrics.memAllocBefore = memStats.Alloc

	var wg sync.WaitGroup
	startSignal := make(chan struct{})

	// Simulate 1K plugin registrations concurrently
	for i := 0; i < StressPluginCount; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-startSignal

			factoryName := fmt.Sprintf("plugin%d", idx)
			factory := newMockFactory(DB, factoryName)

			start := time.Now()
			RegisterPlugin(factory)
			plugin := &mockPlugin{factoryName: factoryName}
			err := registerPluginIns("db", factoryName, DefaultInsName, plugin)
			latency := time.Since(start)

			metrics.recordOp(latency, err == nil)
		}(i)
	}

	close(startSignal)
	wg.Wait()

	metrics.finish()
	metrics.report(t)

	// Verify all plugins registered
	result := ListPlugins()
	assert.Equal(t, StressPluginCount, len(result))
	assert.Equal(t, int64(0), metrics.failedOps, "Should have zero registration failures")
}

// ============================================================================
// Stress Test 3: Mixed Read/Write Operations (Realistic Game Server Load)
// ============================================================================

func TestStress_MixedReadWriteOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	resetPluginManager()
	metrics := newStressTestMetrics()

	// Setup initial plugins
	for i := 0; i < 100; i++ {
		factoryName := fmt.Sprintf("plugin%d", i)
		factory := newMockFactory(DB, factoryName)
		RegisterPlugin(factory)
		plugin := &mockPlugin{factoryName: factoryName}
		_ = registerPluginIns("db", factoryName, DefaultInsName, plugin)
	}

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	metrics.memAllocBefore = memStats.Alloc

	var wg sync.WaitGroup
	startSignal := make(chan struct{})

	// 80% read operations (simulating player queries)
	readGoroutines := int(float64(StressGoroutineCount) * 0.8)
	for i := 0; i < readGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-startSignal

			pluginIdx := idx % 100
			factoryName := fmt.Sprintf("plugin%d", pluginIdx)

			for j := 0; j < 100; j++ {
				start := time.Now()
				_, err := GetDefaultPlugin("db", factoryName)
				latency := time.Since(start)
				metrics.recordOp(latency, err == nil)
			}
		}(i)
	}

	// 20% write operations (simulating plugin updates)
	writeGoroutines := StressGoroutineCount - readGoroutines
	for i := 0; i < writeGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-startSignal

			for j := 0; j < 10; j++ {
				factoryName := fmt.Sprintf("new_plugin_%d_%d", idx, j)
				factory := newMockFactory(DB, factoryName)

				start := time.Now()
				RegisterPlugin(factory)
				plugin := &mockPlugin{factoryName: factoryName}
				err := registerPluginIns("db", factoryName, DefaultInsName, plugin)
				latency := time.Since(start)

				metrics.recordOp(latency, err == nil)
			}
		}(i)
	}

	close(startSignal)
	wg.Wait()

	metrics.finish()
	metrics.report(t)

	assert.Equal(t, int64(0), metrics.failedOps, "Should have zero failures in mixed operations")
}

// ============================================================================
// Stress Test 4: Hot Reload Under Load (Game Server Hot Update)
// ============================================================================

func TestStress_HotReloadUnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	resetPluginManager()
	metrics := newStressTestMetrics()

	// Setup plugin with factory
	factory := newMockFactory(DB, "mysql")
	RegisterPlugin(factory)
	plugin := &mockPlugin{factoryName: "mysql"}
	err := registerPluginIns("db", "mysql", DefaultInsName, plugin)
	require.NoError(t, err)

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	metrics.memAllocBefore = memStats.Alloc

	var wg sync.WaitGroup
	stopSignal := make(chan struct{})

	// Background read load (simulating active players)
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stopSignal:
					return
				default:
					start := time.Now()
					_, err := GetDefaultPlugin("db", "mysql")
					latency := time.Since(start)
					metrics.recordOp(latency, err == nil)
					time.Sleep(1 * time.Millisecond)
				}
			}
		}()
	}

	// Hot reload operations (simulating config updates)
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				select {
				case <-stopSignal:
					return
				default:
					start := time.Now()
					newConfig := map[string]any{
						"host": "localhost",
						"port": 3306 + j,
					}
					err := factory.Reload(plugin, newConfig)
					latency := time.Since(start)
					metrics.recordOp(latency, err == nil)
					time.Sleep(10 * time.Millisecond)
				}
			}
		}()
	}

	// Run for duration
	time.Sleep(5 * time.Second)
	close(stopSignal)
	wg.Wait()

	metrics.finish()
	metrics.report(t)

	// Verify hot reload worked
	assert.Greater(t, plugin.GetReloadCount(), 0, "Should have reloaded at least once")
	assert.Less(t, time.Duration(metrics.maxLatency), 50*time.Millisecond, "Max latency should < 50ms during hot reload")
}

// ============================================================================
// Stress Test 5: Memory Pressure Test (GC Impact)
// ============================================================================

func TestStress_MemoryPressure(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	resetPluginManager()
	metrics := newStressTestMetrics()

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	metrics.memAllocBefore = memStats.Alloc
	initialGoroutines := runtime.NumGoroutine()

	// Create many plugins with large configs
	for i := 0; i < StressMemoryAllocCount; i++ {
		factoryName := fmt.Sprintf("plugin%d", i)
		factory := newMockFactory(DB, factoryName)
		RegisterPlugin(factory)

		// Large config to simulate memory pressure
		config := make(map[string]any)
		for j := 0; j < 100; j++ {
			config[fmt.Sprintf("key%d", j)] = fmt.Sprintf("value%d_%d", i, j)
		}

		plugin := &mockPlugin{
			factoryName: factoryName,
			config:      config,
		}

		start := time.Now()
		err := registerPluginIns("db", factoryName, DefaultInsName, plugin)
		latency := time.Since(start)
		metrics.recordOp(latency, err == nil)

		// Trigger GC periodically
		if i%1000 == 0 {
			runtime.GC()
		}
	}

	// Force GC and wait for completion
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	metrics.finish()
	metrics.report(t)

	// Check for goroutine leaks
	finalGoroutines := runtime.NumGoroutine()
	goroutineLeak := finalGoroutines - initialGoroutines
	t.Logf("Goroutine leak: %d (initial: %d, final: %d)", goroutineLeak, initialGoroutines, finalGoroutines)
	assert.Less(t, goroutineLeak, 10, "Should not leak significant goroutines")

	// Memory usage should be reasonable
	memUsedMB := float64(metrics.memAllocAfter-metrics.memAllocBefore) / 1024 / 1024
	t.Logf("Memory used: %.2f MB", memUsedMB)
	assert.Less(t, memUsedMB, 500.0, "Memory usage should < 500MB")
}

// ============================================================================
// Stress Test 6: Connection Pool Simulation (DB Plugin Stress)
// ============================================================================

func TestStress_ConnectionPoolSimulation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	resetPluginManager()
	metrics := newStressTestMetrics()

	// Setup DB plugin with connection pool
	factory := newMockFactory(DB, "mysql")
	RegisterPlugin(factory)
	plugin := &mockPlugin{factoryName: "mysql"}
	err := registerPluginIns("db", "mysql", DefaultInsName, plugin)
	require.NoError(t, err)

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	metrics.memAllocBefore = memStats.Alloc

	var wg sync.WaitGroup
	startSignal := make(chan struct{})

	// Simulate 5K concurrent connections
	for i := 0; i < StressConnectionPoolSize; i++ {
		wg.Add(1)
		go func(connID int) {
			defer wg.Done()
			<-startSignal

			// Simulate connection lifecycle
			for j := 0; j < 20; j++ {
				// Acquire connection
				start := time.Now()
				p, err := GetDefaultPlugin("db", "mysql")
				if err == nil {
					// Simulate active task
					if mp, ok := p.(*mockPlugin); ok {
						atomic.AddInt32(&mp.activeTaskCnt, 1)
					}

					// Simulate query execution
					time.Sleep(time.Microsecond * time.Duration(100+connID%100))

					// Release connection
					if mp, ok := p.(*mockPlugin); ok {
						atomic.AddInt32(&mp.activeTaskCnt, -1)
					}
				}
				latency := time.Since(start)
				metrics.recordOp(latency, err == nil)
			}
		}(i)
	}

	close(startSignal)
	wg.Wait()

	metrics.finish()
	metrics.report(t)

	// Verify no connection leaks
	assert.Equal(t, 0, plugin.GetActiveTaskCount(), "Should have zero active connections after test")
	assert.Equal(t, int64(0), metrics.failedOps, "Should have zero connection failures")
}

// ============================================================================
// Stress Test 7: Rapid Plugin Lifecycle (Setup/Destroy Stress)
// ============================================================================

func TestStress_RapidPluginLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	metrics := newStressTestMetrics()

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	metrics.memAllocBefore = memStats.Alloc

	var wg sync.WaitGroup
	startSignal := make(chan struct{})

	// Rapid setup/destroy cycles
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-startSignal

			for j := 0; j < 10; j++ {
				resetPluginManager()

				factoryName := fmt.Sprintf("plugin%d_%d", idx, j)
				factory := newMockFactory(DB, factoryName)

				start := time.Now()

				// Setup
				RegisterPlugin(factory)
				config := map[string]any{"host": "localhost", "port": 3306}
				plugin, err := factory.Setup(config)
				if err == nil {
					_ = registerPluginIns("db", factoryName, DefaultInsName, plugin)

					// Destroy
					_ = factory.Destroy(plugin, nil)
				}

				latency := time.Since(start)
				metrics.recordOp(latency, err == nil)
			}
		}(i)
	}

	close(startSignal)
	wg.Wait()

	metrics.finish()
	metrics.report(t)

	assert.Greater(t, metrics.successOps, int64(0))
}

// ============================================================================
// Stress Test 8: CanDelete Safety Check Under Load
// ============================================================================

func TestStress_CanDeleteSafetyCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	resetPluginManager()
	metrics := newStressTestMetrics()

	factory := newMockFactory(DB, "mysql")
	RegisterPlugin(factory)
	plugin := &mockPlugin{factoryName: "mysql"}
	err := registerPluginIns("db", "mysql", DefaultInsName, plugin)
	require.NoError(t, err)

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	metrics.memAllocBefore = memStats.Alloc

	var wg sync.WaitGroup
	stopSignal := make(chan struct{})

	// Simulate active tasks
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stopSignal:
					return
				default:
					// Simulate task start
					atomic.AddInt32(&plugin.activeTaskCnt, 1)
					time.Sleep(time.Millisecond * time.Duration(1+i%10))
					// Simulate task end
					atomic.AddInt32(&plugin.activeTaskCnt, -1)
				}
			}
		}()
	}

	// Continuously check CanDelete
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				select {
				case <-stopSignal:
					return
				default:
					start := time.Now()
					canDelete := factory.CanDelete(plugin)
					latency := time.Since(start)
					metrics.recordOp(latency, true)

					// Log when plugin is safe to delete
					if canDelete && plugin.GetActiveTaskCount() == 0 {
						// Safe to delete
					}
					time.Sleep(time.Millisecond)
				}
			}
		}()
	}

	// Run for duration
	time.Sleep(3 * time.Second)
	close(stopSignal)
	wg.Wait()

	metrics.finish()
	metrics.report(t)

	// Final check should be safe
	assert.Equal(t, 0, plugin.GetActiveTaskCount(), "Should have zero active tasks after test")
	assert.True(t, factory.CanDelete(plugin), "Should be safe to delete after all tasks complete")
}

// ============================================================================
// Benchmark: Stress Test Performance Baseline
// ============================================================================

func BenchmarkStress_GetPluginUnderLoad(b *testing.B) {
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

func BenchmarkStress_FactoryReloadUnderLoad(b *testing.B) {
	resetPluginManager()

	factory := newMockFactory(DB, "mysql")
	RegisterPlugin(factory)
	plugin := &mockPlugin{factoryName: "mysql"}
	_ = registerPluginIns("db", "mysql", DefaultInsName, plugin)

	config := map[string]any{"host": "localhost", "port": 3306}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = factory.Reload(plugin, config)
		}
	})
}

func BenchmarkStress_CanDeleteCheck(b *testing.B) {
	resetPluginManager()

	factory := newMockFactory(DB, "mysql")
	RegisterPlugin(factory)
	plugin := &mockPlugin{factoryName: "mysql"}
	_ = registerPluginIns("db", "mysql", DefaultInsName, plugin)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = factory.CanDelete(plugin)
		}
	})
}
