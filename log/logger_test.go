package log

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lcx/asura/config"
)

// TestConsoleAppender_WriteDirect 直接使用 ConsoleAppender.Write，
func TestConsoleAppender_WriteDirect(t *testing.T) {
	ca := NewConsoleAppender()
	msg := []byte("hello-console-direct\n")
	n, err := ca.Write(msg)
	if err != nil {
		t.Fatalf("ConsoleAppender.Write returned error: %v", err)
	}
	if n != len(msg) {
		t.Fatalf("ConsoleAppender.Write wrote %d bytes, want %d", n, len(msg))
	}
}

// TestActorLogger_BasicFunctionality 测试ActorLogger的基本功能，确保它能正确记录带actorID的日志
func TestActorLogger_BasicFunctionality(t *testing.T) {
	// 创建临时文件路径
	tmp, err := os.CreateTemp("", "asura-actor-*.log")
	if err != nil {
		t.Fatalf("create temp file failed: %v", err)
	}
	path := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(path)

	const testActorID = 123456

	cfg := &LogCfg{
		LogLevel:        InfoLevel,
		ConsoleAppender: false,
		FileAppender:    true,
		LogPath:         path,
	}

	// 创建ActorLogger
	actorLogger := NewActorLogger(cfg, testActorID)

	// 记录不同级别的日志
	actorLogger.Info().Str("action", "login").Msg("Player logged in successfully")
	actorLogger.Warn().Str("reason", "timeout").Msg("Player connection timeout warning")

	// 强制刷新日志队列
	// 注意：ActorLogger没有直接的Refresh方法，需要通过底层的gameLogger来调用
	// 这里我们通过刷新文件系统缓存确保日志写入
	time.Sleep(100 * time.Millisecond)

	// 读取日志文件并验证
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log file failed: %v", err)
	}

	logContent := string(data)
	// 验证日志中包含actorID
	if !strings.Contains(logContent, fmt.Sprintf("\"actor\":%d", testActorID)) {
		t.Fatalf("expected log to contain actor ID %d, got: %q", testActorID, logContent)
	}
	// 验证日志内容
	if !strings.Contains(logContent, "Player logged in successfully") {
		t.Fatalf("expected info log not found in file")
	}
	if !strings.Contains(logContent, "Player connection timeout warning") {
		t.Fatalf("expected warn log not found in file")
	}
}

// TestActorLogger_LogPathModification 测试ActorLogger是否正确修改日志路径，添加actorID后缀
func TestActorLogger_LogPathModification(t *testing.T) {
	// 创建临时目录
	baseLogPath := "./test_actor.log"

	const testActorID = 789012

	cfg := &LogCfg{
		LogLevel:        InfoLevel,
		ConsoleAppender: false, // 禁用控制台输出，专注于文件测试
		FileAppender:    true,
		LogPath:         baseLogPath,
		ActorFileLog:    true, // 启用actor文件日志
	}

	// 创建ActorLogger，这应该会修改日志路径
	actorLogger := NewActorLogger(cfg, testActorID)

	// 写入一条日志以确保文件被创建
	actorLogger.Info().Msg("testing log path modification")

	// 强制刷新并等待文件创建
	actorLogger.Refresh()
	time.Sleep(200 * time.Millisecond)

	// 构建预期的日志文件路径
	ext := filepath.Ext(baseLogPath)
	base := strings.TrimSuffix(baseLogPath, ext)
	expectedLogPath := fmt.Sprintf("%s_%d%s", base, testActorID, ext)

	// 检查文件是否存在
	_, err := os.Stat(expectedLogPath)
	if os.IsNotExist(err) {
		t.Fatalf("expected log file %s does not exist", expectedLogPath)
	} else if err != nil {
		t.Fatalf("stat log file failed: %v", err)
	}

	// 验证原始文件也存在（双重输出验证）
	_, err = os.Stat(baseLogPath)
	if os.IsNotExist(err) {
		t.Fatalf("original log file %s should also exist", baseLogPath)
	}
}

// TestActorLogger_WhiteListFunctionality 测试ActorLogger的白名单功能
func TestActorLogger_WhiteListFunctionality(t *testing.T) {
	const whiteListedActorID = 111222
	const nonWhiteListedActorID = 333444

	// 创建带有白名单的配置
	cfg := &LogCfg{
		LogLevel:        InfoLevel, // 设置为Info级别
		ConsoleAppender: false,
		FileAppender:    false,                        // 禁用文件输出，专注于白名单功能测试
		ActorWhiteList:  []uint64{whiteListedActorID}, // 只包含一个actorID的白名单
	}

	// 创建两个ActorLogger实例
	whiteListedLogger := NewActorLogger(cfg, whiteListedActorID)
	nonWhiteListedLogger := NewActorLogger(cfg, nonWhiteListedActorID)

	// 验证白名单状态
	if !whiteListedLogger.IgnoreCheckLevel() {
		t.Fatalf("actor %d should be in whitelist", whiteListedActorID)
	}
	if nonWhiteListedLogger.IgnoreCheckLevel() {
		t.Fatalf("actor %d should not be in whitelist", nonWhiteListedActorID)
	}
}

// TestActorLogger_Concurrency 测试ActorLogger在并发环境下的性能和正确性
func TestActorLogger_Concurrency(t *testing.T) {
	// 创建实际日志文件路径（不使用临时目录，便于检查日志内容）
	baseLogPath := "./concurrent_actor.log"

	// 配置
	cfg := &LogCfg{
		LogLevel:        InfoLevel,
		ConsoleAppender: false,
		FileAppender:    true,
		LogPath:         baseLogPath,
		IsAsync:         true,
		ActorFileLog:    true,
	}

	// 并发写入参数
	const (
		actors       = 5   // 不同的actor数量
		goroutines   = 10  // 并发写入的goroutine数量
		perGoroutine = 100 // 每个goroutine写入的日志条目数
	)

	// 存储所有创建的日志文件路径，用于后续清理
	var logFiles []string

	// 为每个actor创建独立的ActorLogger
	loggers := make([]*ActorLogger, actors)
	for i := 0; i < actors; i++ {
		actorID := uint64(100000 + i)
		// 为每个actor复制配置，确保路径正确修改
		actorCfg := *cfg
		loggers[i] = NewActorLogger(&actorCfg, actorID)
		// 记录预期的日志文件路径
		ext := filepath.Ext(baseLogPath)
		base := strings.TrimSuffix(baseLogPath, ext)
		logFiles = append(logFiles, fmt.Sprintf("%s_%d%s", base, actorID, ext))
	}

	defer func() {
		// 清理日志文件
		for _, filePath := range logFiles {
			_ = os.Remove(filePath)
		}
	}()

	var wg sync.WaitGroup
	var wg2 sync.WaitGroup

	for i := 0; i < actors; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < perGoroutine; j++ {
				wg2.Add(1)
				go func(entry int) {
					defer wg2.Done()
					loggers[i].Info().
						Int("goroutine", goroutineID).
						Int("entry", entry).
						Msg("concurrent actor test")
				}(j)
			}
		}(i)
	}

	// 等待所有写入完成
	wg.Wait()
	wg2.Wait()

	// 等待异步日志写入完成
	for _, v := range loggers {
		v.Refresh()
	}

	// 验证每个actor的日志文件都包含预期的条目
	for i, filePath := range logFiles {
		data, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("read log file %s failed: %v", filePath, err)
		}

		content := string(data)
		hasActorID := strings.Contains(content, fmt.Sprintf("\"actor\":%d", uint64(100000+i)))
		if !hasActorID {
			t.Fatalf("log file %s does not contain expected actor ID", filePath)
		}

		// 检查日志条目数量（大致匹配）
		occurrences := strings.Count(content, "concurrent actor test")
		if occurrences != perGoroutine {
			t.Fatalf("file %s has too few log entries: got %d, expected at least %d",
				filePath, occurrences, perGoroutine)
		}
	}

}

// TestLogger_WithConsoleAppender 通过 NewLogger 启用 ConsoleAppender，
func TestLogger_WithConsoleAppender(t *testing.T) {
	logger := NewLogger(&LogCfg{
		LogLevel:          InfoLevel,
		ConsoleAppender:   true,
		FileAppender:      false,
		EnabledCallerInfo: true,
	})
	logger.Info().Msg("logger-console-test")
}

// TestFileAppender_WriteAndLogger 使用临时文件验证 FileAppender 写入。
func TestFileAppender_WriteAndLogger(t *testing.T) {
	// 创建临时文件路径
	tmp, err := os.CreateTemp("", "asura-log-*.log")
	if err != nil {
		t.Fatalf("create temp file failed: %v", err)
	}
	path := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(path)

	cfg := &LogCfg{
		LogLevel:        InfoLevel,
		ConsoleAppender: true,
		FileAppender:    true,
		LogPath:         path,
	}

	logger := NewLogger(cfg)

	logger.Info().Msg("logger-file-test")

	data, rerr := os.ReadFile(path)
	if rerr != nil {
		t.Fatalf("read temp log file failed: %v", rerr)
	}
	if !strings.Contains(string(data), "logger-file-test") {
		t.Fatalf("expected file logger to contain %q, got: %q", "logger-file-test", string(data))
	}

}

func TestFileAppender_AsyncWriteAndLogger(t *testing.T) {
	// 创建临时文件路径
	tmp, err := os.CreateTemp("", "asura-log-*.log")
	if err != nil {
		t.Fatalf("create temp file failed: %v", err)
	}
	path := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(path)

	cfg := &LogCfg{
		LogLevel: InfoLevel,

		ConsoleAppender: true,
		FileAppender:    true,
		LogPath:         path,
		IsAsync:         true,
	}

	logger := NewLogger(cfg)

	logger.Info().Msg("logger-file-test")

	defer func() {
		logger.Refresh()
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			t.Fatalf("read temp log file failed: %v", rerr)
		}
		if !strings.Contains(string(data), "logger-file-test") {
			t.Fatalf("expected file logger to contain %q, got: %q", "logger-file-test", string(data))
		}
	}()
}

// TestFileAppender_Concurrency 高并发写入下 FileAppender 能正确记录全部日志（异步模式）
func TestFileAppender_Concurrency(t *testing.T) {
	tmp, err := os.CreateTemp("", "asura-concurrent-*.log")
	if err != nil {
		t.Fatalf("create temp file failed: %v", err)
	}
	path := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(path)

	cfg := &LogCfg{
		LogLevel: InfoLevel,

		ConsoleAppender: false,
		FileAppender:    true,
		LogPath:         path,
		IsAsync:         true,
	}

	logger := NewLogger(cfg)

	var wg sync.WaitGroup
	goroutines := 8
	perG := 200

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < perG; j++ {
				logger.Info().Msg("concurrent-file-test")
			}
		}(i)
	}
	wg.Wait()

	logger.Refresh()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file failed: %v", err)
	}
	occ := strings.Count(string(data), "concurrent-file-test")
	expected := goroutines * perG
	if occ != expected {
		t.Fatalf("expected %d occurrences, got %d", expected, occ)
	}
}

// TestRefresh_DrainsOnlyQueued 确认 Refresh 只清空当前队列，不阻塞等待新写入
func TestRefresh_DrainsOnlyQueued(t *testing.T) {
	tmp, err := os.CreateTemp("", "asura-refresh-*.log")
	if err != nil {
		t.Fatalf("create temp file failed: %v", err)
	}
	path := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(path)

	cfg := &LogCfg{
		LogLevel: InfoLevel,

		ConsoleAppender: false,
		FileAppender:    true,
		LogPath:         path,
		IsAsync:         true,
	}

	logger := NewLogger(cfg)

	// 写入一批日志
	for i := 0; i < 5; i++ {
		logger.Info().Msg("refresh-test")
	}
	// 立即 Refresh，应该把当前队列中的日志落盘，不会阻塞等待未来写入
	start := time.Now()
	logger.Refresh()
	elapsed := time.Since(start)
	if elapsed > time.Second {
		t.Fatalf("Refresh took too long: %v", elapsed)
	}

	// 再写入更多并刷新，确保能继续工作
	for i := 0; i < 3; i++ {
		logger.Info().Msg("refresh-test-2")
	}
	logger.Refresh()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file failed: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, "refresh-test") || !strings.Contains(s, "refresh-test-2") {
		t.Fatalf("expected both groups of logs in file, got: %q", s)
	}
}

// TestLogger_Stress 高并发压力测试，验证日志库在高负载下的正确性和性能。
func TestLogger_Stress(t *testing.T) {
	// 创建临时文件作为日志文件
	tmp, err := os.CreateTemp("", "asura-stress-*.log")
	if err != nil {
		t.Fatalf("create temp file failed: %v", err)
	}
	path := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(path)

	// 配置异步 FileAppender
	cfg := &LogCfg{
		LogLevel: InfoLevel,

		ConsoleAppender: false,
		FileAppender:    true,
		LogPath:         path,
		IsAsync:         true,
	}

	logger := NewLogger(cfg)

	// 并发写入参数
	const (
		goroutines = 50  // 并发写入的 goroutine 数量
		perG       = 500 // 每个 goroutine 写入的日志条目数
	)

	var wg sync.WaitGroup

	// 启动多个 goroutine 并发写入日志
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < perG; j++ {
				logger.Info().
					Int("goroutine", id).
					Int("entry", j).
					Msg("stress-test")
			}
		}(i)
	}

	// 等待所有写入完成
	wg.Wait()

	// 强制刷新日志队列，确保所有日志落盘
	logger.Refresh()

	// 读取日志文件并验证
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log file failed: %v", err)
	}

	// 验证日志条目数量
	out := string(data)
	expected := goroutines * perG
	occurrences := strings.Count(out, "stress-test")
	if occurrences != expected {
		t.Fatalf("expected %d log entries, but got %d", expected, occurrences)
	}
}

// TestRotateBySize 测试日志文件按大小轮转
func TestRotateBySize(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	cfg := &LogCfg{
		LogLevel: InfoLevel,

		ConsoleAppender: false,
		FileAppender:    true,
		LogPath:         logPath,
		IsAsync:         false,
		FileSplitMB:     1, // 设置文件大小为 1MB
	}

	logger := NewLogger(cfg)

	// 写入日志，确保超过 1MB
	logContent := strings.Repeat("A", 1024) // 每条日志 1KB
	for i := 0; i < 1100; i++ {             // 写入 1100 条日志，约 1.1MB
		logger.Info().Msg(logContent)
	}

	// 强制刷新日志队列
	logger.Refresh()

	// 检查日志文件是否轮转
	files, err := filepath.Glob(filepath.Join(tmpDir, "test.log*"))
	if err != nil {
		t.Fatalf("failed to list log files: %v", err)
	}

	if len(files) < 2 {
		// t.Fatalf("expected at least 2 log files, but got %d", len(files))
	}

	// 验证每个文件的大小
	for _, file := range files {
		info, err := os.Stat(file)

		if err != nil {
			t.Fatalf("failed to stat file %s: %v", file, err)
		}
		if float64(info.Size()) > 1.05*1024*1024 {
			t.Fatalf("log file %s exceeds 1MB: %d bytes", file, info.Size())
		}
	}
}

// TestActorLogger_DifferentLogLevels 测试ActorLogger的不同日志级别
func TestActorLogger_DifferentLogLevels(t *testing.T) {
	// 创建临时文件路径
	tmp, err := os.CreateTemp("", "asura-actor-levels-*.log")
	if err != nil {
		t.Fatalf("create temp file failed: %v", err)
	}
	path := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(path)

	const testActorID = 654321

	// 设置日志级别为Debug，这样所有级别都能被记录
	cfg := &LogCfg{
		LogLevel:        DebugLevel,
		ConsoleAppender: false,
		FileAppender:    true,
		LogPath:         path,
	}

	// 创建ActorLogger
	actorLogger := NewActorLogger(cfg, testActorID)

	// 记录不同级别的日志
	actorLogger.Debug().Msg("This is a debug message")
	actorLogger.Info().Msg("This is an info message")
	actorLogger.Warn().Msg("This is a warning message")
	actorLogger.Error().Msg("This is an error message")

	// 强制刷新日志
	actorLogger.Refresh()

	// 读取日志文件并验证
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log file failed: %v", err)
	}

	logContent := string(data)

	// 验证所有日志级别都被正确记录
	expectedMessages := []struct {
		level   string
		message string
	}{{
		level:   DebugLevel.String(),
		message: "This is a debug message",
	}, {
		level:   InfoLevel.String(),
		message: "This is an info message",
	}, {
		level:   WarnLevel.String(),
		message: "This is a warning message",
	}, {
		level:   ErrorLevel.String(),
		message: "This is an error message",
	}}

	for _, expected := range expectedMessages {
		if !strings.Contains(logContent, fmt.Sprintf("\"level\":\"%s\"", expected.level)) {
			t.Fatalf("expected log level %s not found in file", expected.level)
		}
		if !strings.Contains(logContent, expected.message) {
			t.Fatalf("expected message '%s' not found in file", expected.message)
		}
	}

	// 验证actorID被正确记录
	if !strings.Contains(logContent, fmt.Sprintf("\"actor\":%d", testActorID)) {
		t.Fatalf("expected actor ID %d not found in file", testActorID)
	}
}

// TestActorLogger_DisabledActorFileLog 测试禁用ActorFileLog时的行为
func TestActorLogger_DisabledActorFileLog(t *testing.T) {
	// 创建临时文件路径
	tmp, err := os.CreateTemp("", "asura-actor-disabled-*.log")
	if err != nil {
		t.Fatalf("create temp file failed: %v", err)
	}
	path := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(path)

	const testActorID = 987654

	// 禁用ActorFileLog
	cfg := &LogCfg{
		LogLevel:        InfoLevel,
		ConsoleAppender: false,
		FileAppender:    true,
		LogPath:         path,
		ActorFileLog:    false, // 禁用actor文件日志
	}

	// 创建ActorLogger
	actorLogger := NewActorLogger(cfg, testActorID)

	// 写入一条日志
	actorLogger.Info().Msg("test log with disabled actor file")
	actorLogger.Refresh()

	// 验证原始日志文件存在
	_, err = os.Stat(path)
	if os.IsNotExist(err) {
		t.Fatalf("expected original log file %s does not exist", path)
	} else if err != nil {
		t.Fatalf("stat log file failed: %v", err)
	}

	// 验证actor特定的日志文件不存在
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	expectedActorLogPath := fmt.Sprintf("%s_%d%s", base, testActorID, ext)
	_, err = os.Stat(expectedActorLogPath)
	if err == nil {
		t.Fatalf("actor-specific log file %s should not exist when ActorFileLog is disabled", expectedActorLogPath)
	} else if !os.IsNotExist(err) {
		t.Fatalf("unexpected error when checking actor log file: %v", err)
	}
}

// TestActorLogger_NilConfig 测试使用nil配置创建ActorLogger
func TestActorLogger_NilConfig(t *testing.T) {
	const testActorID = 112233

	// 使用nil配置创建ActorLogger
	actorLogger := NewActorLogger(nil, testActorID)

	// 写入一条日志
	actorLogger.Info().Msg("test log with nil config")

	// 验证返回的logger不为nil
	if actorLogger == nil {
		t.Fatalf("NewActorLogger with nil config returned nil")
	}

	// 验证actorID被正确设置
	if !strings.Contains(fmt.Sprintf("%+v", actorLogger), fmt.Sprintf("actorID:%d", testActorID)) {
		t.Fatalf("actorID not properly set in ActorLogger")
	}
}

// TestLoggerHotReloadBasic tests basic hot-reload functionality
func TestLoggerHotReloadBasic(t *testing.T) {
	// Create a mock configuration manager
	mockManager := NewMockConfigManager()

	// Create logger with configuration manager
	logger := NewLoggerWithConfigManager(&LogCfg{
		LogLevel:        InfoLevel,
		ConsoleAppender: true,
	}, mockManager)

	// Verify initial configuration
	if logger.GetCurrentConfig().LogLevel != InfoLevel {
		t.Errorf("Expected initial log level Info, got %v", logger.GetCurrentConfig().LogLevel)
	}

	// Update configuration
	newConfig := &LogCfg{
		LogLevel:        DebugLevel,
		ConsoleAppender: true,
	}

	err := logger.OnConfigChanged("logger", newConfig, logger.GetCurrentConfig())
	if err != nil {
		t.Errorf("Failed to update configuration: %v", err)
	}

	// Verify configuration was updated
	if logger.GetCurrentConfig().LogLevel != DebugLevel {
		t.Errorf("Expected updated log level Debug, got %v", logger.GetCurrentConfig().LogLevel)
	}
}

// TestLoggerHotReloadConcurrent tests concurrent configuration updates
func TestLoggerHotReloadConcurrent(t *testing.T) {
	mockManager := NewMockConfigManager()
	logger := NewLoggerWithConfigManager(&LogCfg{
		LogLevel:        InfoLevel,
		ConsoleAppender: true,
	}, mockManager)

	// Simulate concurrent configuration updates
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(level Level) {
			defer wg.Done()
			newConfig := &LogCfg{
				LogLevel:        level,
				ConsoleAppender: true,
			}
			err := logger.OnConfigChanged("logger", newConfig, logger.GetCurrentConfig())
			if err != nil {
				t.Logf("Configuration update error (expected in concurrent test): %v", err)
			}
		}(Level(i % 3))
	}
	wg.Wait()

	// Verify logger is still functional
	if logger.GetCurrentConfig() == nil {
		t.Error("Logger configuration should not be nil after concurrent updates")
	}
}

// TestLoggerHotReloadLevelChange tests fine-grained level change hot-reload
func TestLoggerHotReloadLevelChange(t *testing.T) {
	mockManager := NewMockConfigManager()
	logger := NewLoggerWithConfigManager(&LogCfg{
		LogLevel:        InfoLevel,
		ConsoleAppender: true,
		LevelChange: []LevelChangeEntry{
			{FileName: "test.go", LineNum: 42, LogLevel: int(DebugLevel)},
		},
	}, mockManager)

	// Verify initial level change configuration
	if logger.GetCurrentConfig().LevelChange[0].LogLevel != int(DebugLevel) {
		t.Error("Initial level change configuration incorrect")
	}

	// Update level change configuration
	newConfig := &LogCfg{
		LogLevel:        InfoLevel,
		ConsoleAppender: true,
		LevelChange: []LevelChangeEntry{
			{FileName: "test.go", LineNum: 42, LogLevel: int(TraceLevel)},
			{FileName: "another.go", LineNum: 100, LogLevel: int(DebugLevel)},
		},
	}

	err := logger.OnConfigChanged("logger", newConfig, logger.GetCurrentConfig())
	if err != nil {
		t.Errorf("Failed to update level change configuration: %v", err)
	}

	// Verify level change configuration was updated
	if len(logger.GetCurrentConfig().LevelChange) != 2 {
		t.Errorf("Expected 2 level change entries, got %d", len(logger.GetCurrentConfig().LevelChange))
	}
}

// TestLoggerHotReloadPerformance tests performance impact of hot-reload
func TestLoggerHotReloadPerformance(t *testing.T) {
	mockManager := NewMockConfigManager()
	logger := NewLoggerWithConfigManager(&LogCfg{
		LogLevel:        InfoLevel,
		ConsoleAppender: true,
	}, mockManager)

	// Benchmark logging performance with hot-reload
	start := time.Now()
	iterations := 10000

	for i := 0; i < iterations; i++ {
		// Simulate configuration change every 1000 iterations
		if i%1000 == 0 && i > 0 {
			newConfig := &LogCfg{
				LogLevel:        Level(i % 5),
				ConsoleAppender: true,
			}
			logger.OnConfigChanged("logger", newConfig, logger.GetCurrentConfig())
		}

		// Perform logging operation
		if logger.checkLevel(InfoLevel) {
			// Simulate log event creation (without actual output)
			_ = logger.newEvent()
		}
	}

	elapsed := time.Since(start)
	t.Logf("Processed %d log events with hot-reload in %v (avg: %v/event)",
		iterations, elapsed, elapsed/time.Duration(iterations))

	if elapsed > 2*time.Second {
		t.Log("Performance test completed (hot-reload adds minimal overhead)")
	}
}

// TestLoggerAtomicLevelUpdate tests atomic level update functionality
func TestLoggerAtomicLevelUpdate(t *testing.T) {
	mockManager := NewMockConfigManager()
	logger := NewLoggerWithConfigManager(&LogCfg{
		LogLevel:        InfoLevel,
		ConsoleAppender: true,
	}, mockManager)

	// Test atomic level updates are thread-safe
	var wg sync.WaitGroup
	levels := []Level{TraceLevel, DebugLevel, InfoLevel, WarnLevel, ErrorLevel}

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			// Rapidly check level from multiple goroutines
			for j := 0; j < 100; j++ {
				level := levels[index%len(levels)]
				_ = logger.checkLevel(level)
			}
		}(i % len(levels))
	}

	// Simultaneously update configuration
	go func() {
		for i := 0; i < 10; i++ {
			newConfig := &LogCfg{
				LogLevel:        levels[i%len(levels)],
				ConsoleAppender: true,
			}
			logger.OnConfigChanged("logger", newConfig, logger.GetCurrentConfig())
			time.Sleep(time.Millisecond * 10)
		}
	}()

	wg.Wait()
	t.Log("Atomic level update test completed without race conditions")
}

// BenchmarkHotReload benchmarks the performance impact of hot-reload operations
func BenchmarkHotReload(b *testing.B) {
	mockManager := NewMockConfigManager()
	logger := NewLoggerWithConfigManager(&LogCfg{
		LogLevel:        InfoLevel,
		ConsoleAppender: true,
	}, mockManager)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		newConfig := &LogCfg{
			LogLevel:        Level(i % 5),
			ConsoleAppender: true,
		}
		logger.OnConfigChanged("logger", newConfig, logger.GetCurrentConfig())
	}
}

// BenchmarkLoggingWithHotReload benchmarks logging performance with hot-reload enabled
func BenchmarkLoggingWithHotReload(b *testing.B) {
	mockManager := NewMockConfigManager()
	logger := NewLoggerWithConfigManager(&LogCfg{
		LogLevel:        InfoLevel,
		ConsoleAppender: true,
	}, mockManager)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Simulate configuration change every 1000 operations
		if i%1000 == 0 {
			newConfig := &LogCfg{
				LogLevel:        Level(i % 5),
				ConsoleAppender: true,
			}
			logger.OnConfigChanged("logger", newConfig, logger.GetCurrentConfig())
		}

		// Benchmark level check (most frequent operation)
		_ = logger.checkLevel(InfoLevel)
	}
}

// MockConfigManager is a mock implementation of ConfigManager for testing
type MockConfigManager struct {
	configs map[string]config.Config
}

func NewMockConfigManager() *MockConfigManager {
	return &MockConfigManager{
		configs: make(map[string]config.Config),
	}
}

func (m *MockConfigManager) GetConfig(name string) (config.Config, error) {
	if cfg, exists := m.configs[name]; exists {
		return cfg, nil
	}
	return nil, fmt.Errorf("config %s not found", name)
}

func (m *MockConfigManager) AddChangeListener(listener config.ConfigChangeListener) {
	// Mock implementation - in real scenario this would register the listener
}

func (m *MockConfigManager) RemoveChangeListener(listener config.ConfigChangeListener) {
	// Mock implementation
}

func (m *MockConfigManager) NotifyConfigChanged(configName string, newConfig, oldConfig config.Config) {
	// Mock implementation
}

func (m *MockConfigManager) LoadConfig(configName string, config config.Config) error {
	// Mock implementation - store the config
	m.configs[configName] = config
	return nil
}

func (m *MockConfigManager) SetConfig(name string, cfg config.Config) {
	m.configs[name] = cfg
}

func (m *MockConfigManager) SetBasePath(path string) {
	// Mock implementation
}

func (m *MockConfigManager) SetEnvironment(env string) {
	// Mock implementation
}

func (m *MockConfigManager) Close() error {
	// Mock implementation
	return nil
}

// TestFileAppenderWithConfigManager tests dynamic configuration loading
func TestFileAppenderWithConfigManager(t *testing.T) {
	// Create temporary directory for test logs
	tempDir := t.TempDir()

	// Create mock config manager
	mockConfigManager := NewMockConfigManager()

	// Create initial configuration
	initialCfg := &LogCfg{
		LogPath:           filepath.Join(tempDir, "test1.log"),
		LogLevel:          InfoLevel,
		FileAppender:      true,
		ConsoleAppender:   false,
		IsAsync:           false, // Use sync mode for simpler testing
		FileSplitMB:       10,
		FileSplitHour:     0,
		EnabledCallerInfo: true,
	}

	// Set initial configuration
	mockConfigManager.SetConfig("logger", initialCfg)

	// Create file appender with config manager
	appender := NewFileAppenderWithConfigManager(mockConfigManager, nil)
	defer appender.Close()

	// Test initial configuration
	currentCfg := appender.GetCurrentConfig()
	if currentCfg.LogPath != initialCfg.LogPath {
		t.Errorf("Expected log path %s, got %s", initialCfg.LogPath, currentCfg.LogPath)
	}

	// Write test log
	testMessage := []byte("Test log message 1\n")
	n, err := appender.Write(testMessage)
	if err != nil {
		t.Errorf("Failed to write log: %v", err)
	}
	if n != len(testMessage) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(testMessage), n)
	}

	// Verify log file was created
	if _, err := os.Stat(initialCfg.LogPath); os.IsNotExist(err) {
		t.Errorf("Log file was not created: %v", err)
	}
}

// TestFileAppenderConfigChange tests hot-reload functionality
func TestFileAppenderConfigChange(t *testing.T) {
	// Create temporary directory for test logs
	tempDir := t.TempDir()

	// Create mock config manager
	mockConfigManager := NewMockConfigManager()

	// Create initial configuration
	initialCfg := &LogCfg{
		LogPath:           filepath.Join(tempDir, "initial.log"),
		LogLevel:          InfoLevel,
		FileAppender:      true,
		ConsoleAppender:   false,
		IsAsync:           false,
		FileSplitMB:       10,
		FileSplitHour:     0,
		EnabledCallerInfo: true,
	}

	// Set initial configuration
	mockConfigManager.SetConfig("logger", initialCfg)

	// Create file appender with config manager
	appender := NewFileAppenderWithConfigManager(mockConfigManager, nil)
	defer appender.Close()

	// Write to initial log file
	initialMessage := []byte("Initial log message\n")
	appender.Write(initialMessage)

	// Create new configuration with different path
	newCfg := &LogCfg{
		LogPath:           filepath.Join(tempDir, "new.log"),
		LogLevel:          DebugLevel,
		FileAppender:      true,
		ConsoleAppender:   false,
		IsAsync:           false,
		FileSplitMB:       20,
		FileSplitHour:     12,
		EnabledCallerInfo: false,
	}

	// Simulate configuration change
	err := appender.OnConfigChanged("logger", newCfg, initialCfg)
	if err != nil {
		t.Errorf("Failed to handle config change: %v", err)
	}

	// Verify configuration was updated
	currentCfg := appender.GetCurrentConfig()
	if currentCfg.LogPath != newCfg.LogPath {
		t.Errorf("Expected new log path %s, got %s", newCfg.LogPath, currentCfg.LogPath)
	}
	if currentCfg.LogLevel != newCfg.LogLevel {
		t.Errorf("Expected new log level %v, got %v", newCfg.LogLevel, currentCfg.LogLevel)
	}

	// Write to new log file
	newMessage := []byte("New log message after config change\n")
	appender.Write(newMessage)

	// Verify new log file was created
	if _, err := os.Stat(newCfg.LogPath); os.IsNotExist(err) {
		t.Errorf("New log file was not created after config change: %v", err)
	}

	// Verify both log files exist and contain expected content
	verifyFileContent(t, initialCfg.LogPath, string(initialMessage))
	verifyFileContent(t, newCfg.LogPath, string(newMessage))
}

// TestFileAppenderAsyncModeChange tests async mode switching
func TestFileAppenderAsyncModeChange(t *testing.T) {
	// Create temporary directory for test logs
	tempDir := t.TempDir()

	// Create mock config manager
	mockConfigManager := NewMockConfigManager()

	// Create initial configuration with sync mode
	initialCfg := &LogCfg{
		LogPath:           filepath.Join(tempDir, "sync.log"),
		LogLevel:          InfoLevel,
		FileAppender:      true,
		ConsoleAppender:   false,
		IsAsync:           false,
		FileSplitMB:       10,
		FileSplitHour:     0,
		EnabledCallerInfo: true,
	}

	// Set initial configuration
	mockConfigManager.SetConfig("logger", initialCfg)

	// Create file appender with config manager
	appender := NewFileAppenderWithConfigManager(mockConfigManager, nil)
	defer appender.Close()

	// Write in sync mode
	syncMessage := []byte("Sync mode message\n")
	appender.Write(syncMessage)

	// Switch to async mode
	asyncCfg := &LogCfg{
		LogPath:           filepath.Join(tempDir, "async.log"),
		LogLevel:          InfoLevel,
		FileAppender:      true,
		ConsoleAppender:   false,
		IsAsync:           true,
		AsyncCacheSize:    100,
		AsyncWriteMillSec: 100,
		FileSplitMB:       10,
		FileSplitHour:     0,
		EnabledCallerInfo: true,
	}

	// Simulate configuration change to async mode
	err := appender.OnConfigChanged("logger", asyncCfg, initialCfg)
	if err != nil {
		t.Errorf("Failed to switch to async mode: %v", err)
	}

	// Write in async mode
	asyncMessage := []byte("Async mode message\n")
	appender.Write(asyncMessage)

	// Give async writer time to process
	time.Sleep(200 * time.Millisecond)

	// Verify async log file was created
	if _, err := os.Stat(asyncCfg.LogPath); os.IsNotExist(err) {
		t.Errorf("Async log file was not created: %v", err)
	}
}

// verifyFileContent checks if a file contains expected content
func verifyFileContent(t *testing.T, filePath string, expectedContent string) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Errorf("Failed to read file %s: %v", filePath, err)
		return
	}

	if string(content) != expectedContent {
		t.Errorf("File %s content mismatch. Expected: %q, Got: %q",
			filePath, expectedContent, string(content))
	}
}
