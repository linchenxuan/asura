package log

import (
	"path/filepath"
	"testing"
)

// BenchmarkLogger_SyncFile 测试同步文件写入性能
func BenchmarkLogger_SyncFile(b *testing.B) {
	// 创建临时目录
	tmpDir := b.TempDir()
	logPath := filepath.Join(tmpDir, "sync.log")

	logger := NewLogger(&LogCfg{
		LogLevel:        InfoLevel,
		ConsoleAppender: false,
		FileAppender:    true,
		LogPath:         logPath,
		IsAsync:         false, // 同步写入
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info().Str("key", "value").Int("number", 42).Msg("benchmark test message")
	}

	// 确保所有日志都写入磁盘
	logger.Refresh()
}

// BenchmarkLogger_AsyncFile 测试异步文件写入性能
func BenchmarkLogger_AsyncFile(b *testing.B) {
	// 创建临时目录
	tmpDir := b.TempDir()
	logPath := filepath.Join(tmpDir, "async.log")

	logger := NewLogger(&LogCfg{
		LogLevel:        InfoLevel,
		ConsoleAppender: false,
		FileAppender:    true,
		LogPath:         logPath,
		IsAsync:         true, // 异步写入
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info().Str("key", "value").Int("number", 42).Msg("benchmark test message")
	}

	// 确保所有日志都写入磁盘
	logger.Refresh()
}

// BenchmarkLogger_Parallel 测试并发写入性能
func BenchmarkLogger_Parallel(b *testing.B) {
	logger := NewLogger(&LogCfg{
		LogLevel:        InfoLevel,
		ConsoleAppender: false,
		FileAppender:    false,
	})

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.Info().Str("key", "value").Int("number", 42).Msg("benchmark test message")
		}
	})
}

// BenchmarkLogger_ParallelFile 测试并发文件写入性能
func BenchmarkLogger_ParallelFile(b *testing.B) {
	// 创建临时目录
	tmpDir := b.TempDir()
	logPath := filepath.Join(tmpDir, "parallel.log")

	logger := NewLogger(&LogCfg{
		LogLevel:          InfoLevel,
		ConsoleAppender:   false,
		FileAppender:      true,
		LogPath:           logPath,
		IsAsync:           true, // 异步写入以提高并发性能
		EnabledCallerInfo: true,
	})

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.Info().Str("key", "value").Int("number", 42).Msg("benchmark test message")
		}
	})

	// 确保所有日志都写入磁盘
	logger.Refresh()
}

// BenchmarkLogger_ParallelFile 测试并发文件写入性能
func BenchmarkLogger_ParallelFileNoCaller(b *testing.B) {
	// 创建临时目录
	tmpDir := b.TempDir()
	logPath := filepath.Join(tmpDir, "parallel.log")

	logger := NewLogger(&LogCfg{
		LogLevel:        InfoLevel,
		ConsoleAppender: false,
		FileAppender:    true,
		LogPath:         logPath,
		IsAsync:         true, // 异步写入以提高并发性能
	})

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.Info().Str("key", "value").Int("number", 42).Msg("benchmark test message")
		}
	})

	// 确保所有日志都写入磁盘
	logger.Refresh()
}

// BenchmarkGetCallerInfo measures the performance of the getCallerInfo method
func BenchmarkLogger_GetCallerInfo(b *testing.B) {
	logger := NewLogger(&LogCfg{
		LogLevel:        InfoLevel,
		ConsoleAppender: false,
		FileAppender:    false,
	})
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		logger.getCallerInfo()
	}
}
