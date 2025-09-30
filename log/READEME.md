# AsuraLog - 高性能零分配 Go 日志库

`AsuraLog` 是一个专为高性能场景设计的结构化、异步 Go 日志库，特别适用于游戏服务器等对性能和延迟要求极高的应用场景。通过零分配 API、异步写入、对象池化和高性能编码器，`AsuraLog` 最大限度地减少了日志记录对主逻辑的影响。

---

## 核心特性

- **零分配 API**: 提供链式调用方法（如 `.Str()`、`.Int()`），避免 `interface{}` 装箱导致的内存分配。
- **异步写入**: 日志创建与写入分离，通过 `channel` 解耦，避免磁盘或网络 I/O 阻塞业务逻辑。
- **对象池化**: 使用 `sync.Pool` 复用 `LogEvent` 对象，显著降低 GC 压力。
- **高性能编码**: 内置无反射的 JSON 编码器，直接操作 `[]byte`，性能远超标准库。
- **结构化日志**: 以 JSON 格式输出日志，便于与 ELK、Loki 等日志分析系统集成。
- **自动轮转**: 支持按文件大小或时间切割日志文件。
- **灵活配置**: 支持日志级别、输出目标（Appender）、调用者信息等多种配置项。

---

## 快速开始

### 1. 安装

```bash
go get github.com/lcx/asura/log
```

### 2. 使用示例

```go
package main

import (
    "errors"
    "github.com/lcx/asura/log"
)

func main() {
    // 初始化 Logger
    defer log.Sync() // 确保程序退出前日志刷盘

    // 记录服务器启动日志
    log.Info().
        Str("server_name", "login_server").
        Int("port", 8080).
        Msg("服务器启动成功")

    playerID := "player-12345"
    // 记录玩家登录事件
    log.Debug().
        Str("player_id", playerID).
        Str("ip_addr", "192.168.1.10").
        Msg("玩家尝试登录")

    err := errors.New("invalid password")
    if err != nil {
        // 记录错误日志
        log.Error().
            Err(err).
            Str("player_id", playerID).
            Msg("玩家登录失败")
    }
}
```

### 输出示例

```json
{"level":"info","time":"2025-09-07T12:00:00.123Z","caller":"main.go:16","message":"服务器启动成功","server_name":"login_server","port":8080}
{"level":"debug","time":"2025-09-07T12:00:00.124Z","caller":"main.go:22","message":"玩家尝试登录","player_id":"player-12345","ip_addr":"192.168.1.10"}
{"level":"error","time":"2025-09-07T12:00:00.125Z","caller":"main.go:29","message":"玩家登录失败","error":"invalid password","player_id":"player-12345"}
```

---

## 配置说明

`AsuraLog` 提供了灵活的配置项，支持多种场景下的日志记录需求。

### 配置示例

```go
cfg := &log.LogCfg{
    LogLevel:        log.InfoLevel,       // 日志级别
    EnableCaller:    true,                // 是否记录调用者信息
    ConsoleAppender: true,                // 是否输出到控制台
    FileAppender:    true,                // 是否输出到文件
    LogPath:         "./logs/server.log", // 日志文件路径
    IsAsync:         true,                // 是否启用异步写入
}
logger := log.NewLogger(cfg)
```