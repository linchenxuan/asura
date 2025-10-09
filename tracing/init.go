package tracing

import (
	"context"
	"sync"

	"github.com/lcx/asura/config"
)

// 全局Tracer实例
var (
	globalTracer     Tracer
	globalTracerMu   sync.RWMutex
	globalPropagator = make(map[string]Propagator)
	configListenerID = "tracing_config_listener"
)

// 初始化链路追踪系统
func InitTracing() (Tracer, error) {
	// 获取配置管理器
	configMgr := config.GetInstance()

	// 创建并加载配置
	cfg := DefaultTracerConfig()
	if err := configMgr.LoadConfig("tracing", &cfg); err != nil {
		// 配置加载失败时使用默认配置
		cfg = DefaultTracerConfig()
	}

	// 创建tracer
	tracer, err := buildTracer(&cfg)
	if err != nil {
		return nil, err
	}

	// 设置全局tracer
	setGlobalTracer(tracer)

	// 注册默认传播器
	registerDefaultPropagators()

	// 注册配置变更监听器
	configMgr.AddChangeListener(&tracingConfigListener{})

	return tracer, nil
}

// 配置变更监听器
type tracingConfigListener struct{}

// OnConfigChanged 处理配置变更
func (l *tracingConfigListener) OnConfigChanged(configName string, newConfig, oldConfig config.Config) error {
	if configName != "tracing" {
		return nil
	}

	// 类型断言
	newCfg, ok := newConfig.(*TracerConfig)
	if !ok {
		return nil
	}

	// 构建新的tracer
	tracer, err := buildTracer(newCfg)
	if err != nil {
		return err
	}

	// 更新全局tracer
	setGlobalTracer(tracer)

	// 重新注册默认传播器
	registerDefaultPropagators()

	return nil
}

// 构建tracer
func buildTracer(cfg *TracerConfig) (Tracer, error) {
	// 使用builder模式创建tracer
	builder := NewTracerBuilder(*cfg)
	return builder.Build()
}

// 注册默认传播器
func registerDefaultPropagators() {
	// 文本映射格式传播器
	RegisterGlobalPropagator("text_map", NewTextMapPropagator())
	// HTTP头格式传播器
	RegisterGlobalPropagator("http_headers", NewHTTPHeadersPropagator())
	// 二进制格式传播器
	RegisterGlobalPropagator("binary", NewBinaryPropagator())
}

// 设置全局tracer
func setGlobalTracer(tracer Tracer) {
	globalTracerMu.Lock()
	defer globalTracerMu.Unlock()

	// 关闭旧的tracer
	if globalTracer != nil {
		globalTracer.Close()
	}

	globalTracer = tracer

	// 注册所有传播器到新的tracer
	for format, propagator := range globalPropagator {
		if globalTracer != nil {
			globalTracer.RegisterPropagator(format, propagator)
		}
	}
}

// 获取全局tracer
func GlobalTracer() Tracer {
	globalTracerMu.RLock()
	tracer := globalTracer
	globalTracerMu.RUnlock()

	// 如果没有设置全局tracer，返回noop tracer
	if tracer == nil {
		return NewNoopTracer()
	}

	return tracer
}

// 关闭全局tracer
func CloseGlobalTracer() error {
	globalTracerMu.Lock()
	defer globalTracerMu.Unlock()

	if globalTracer != nil {
		err := globalTracer.Close()
		globalTracer = nil
		return err
	}

	return nil
}

// 创建全局span
func CreateGlobalSpan(operationName string, options ...SpanOption) (Span, context.Context) {
	tracer := GlobalTracer()
	span, ctx := tracer.StartSpanFromContext(context.Background(), operationName, options...)
	return span, ctx
}

// 从上下文中创建子span
func CreateChildSpanFromContext(ctx context.Context, operationName string, options ...SpanOption) (Span, context.Context) {
	tracer := GlobalTracer()
	span, newCtx := tracer.StartSpanFromContext(ctx, operationName, options...)
	return span, newCtx
}

// 注册全局传播器
func RegisterGlobalPropagator(format string, propagator Propagator) {
	globalPropagator[format] = propagator

	// 立即注册到当前全局tracer
	tracer := GlobalTracer()
	tracer.RegisterPropagator(format, propagator)
}

// 从载体中提取上下文
func ExtractFromCarrier(format string, carrier Carrier) (SpanContext, error) {
	tracer := GlobalTracer()
	return tracer.Extract(format, carrier)
}

// 注入上下文到载体
func InjectToCarrier(ctx SpanContext, format string, carrier Carrier) error {
	tracer := GlobalTracer()
	return tracer.Inject(ctx, format, carrier)
}

// 创建noop tracer
func NewNoopTracer() Tracer {
	return &noopTracer{}
}

// noopTracer实现了Tracer接口但不执行任何操作
// 用于禁用追踪或作为默认返回值

type noopTracer struct{}

// StartSpan实现Tracer接口
func (t *noopTracer) StartSpan(operationName string, options ...SpanOption) Span {
	return NewNoopSpan()
}

// StartSpanFromContext实现Tracer接口
func (t *noopTracer) StartSpanFromContext(ctx context.Context, operationName string, options ...SpanOption) (Span, context.Context) {
	span := NewNoopSpan()
	return span, context.WithValue(ctx, spanContextKey{}, span.Context())
}

// Extract实现Tracer接口
func (t *noopTracer) Extract(format interface{}, carrier interface{}) (SpanContext, error) {
	return EmptySpanContext(), nil
}

// Inject实现Tracer接口
func (t *noopTracer) Inject(ctx SpanContext, format interface{}, carrier interface{}) error {
	return nil
}

// RegisterPropagator实现Tracer接口
func (t *noopTracer) RegisterPropagator(format interface{}, propagator Propagator) {
}

// Close实现Tracer接口
func (t *noopTracer) Close() error {
	return nil
}

// 上下文键类型

type spanContextKey struct{}

// ContextWithSpan 将span添加到context.Context中
func ContextWithSpan(ctx context.Context, span Span) context.Context {
	return context.WithValue(ctx, spanContextKey{}, span.Context())
}

// SpanFromContext 从context.Context中提取span
func SpanFromContext(ctx context.Context) SpanContext {
	if ctx == nil {
		return EmptySpanContext()
	}

	spanCtx, ok := ctx.Value(spanContextKey{}).(SpanContext)
	if !ok || spanCtx == nil || !spanCtx.IsValid() {
		return EmptySpanContext()
	}

	return spanCtx
}
