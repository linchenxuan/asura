package tracing

import (
	"encoding/base64"
	"strings"
)

// TextMapPropagator 实现文本映射格式的上下文传播
// 负责在不同进程间传递SpanContext信息
type TextMapPropagator struct{}

// NewTextMapPropagator 创建一个新的文本映射传播器
// 用于在文本格式的载体中传播SpanContext
func NewTextMapPropagator() Propagator {
	return &TextMapPropagator{}
}

// Inject 将SpanContext注入到文本映射载体中
// 按照OpenTracing标准格式设置key-value对
func (p *TextMapPropagator) Inject(ctx SpanContext, carrier Carrier) error {
	if !ctx.IsValid() || carrier == nil {
		return nil
	}

	// 设置TraceID和SpanID
	carrier.Set("trace-id", ctx.TraceID())
	carrier.Set("span-id", ctx.SpanID())

	// 注入baggage项
	ctx.ForeachBaggageItem(func(k, v string) bool {
		encodedKey := "baggage-" + k
		carrier.Set(encodedKey, v)
		return true
	})

	return nil
}

// Extract 从文本映射载体中提取SpanContext
// 解析OpenTracing标准格式的key-value对
func (p *TextMapPropagator) Extract(carrier Carrier) (SpanContext, error) {
	if carrier == nil {
		return EmptySpanContext(), nil
	}

	// 提取TraceID和SpanID
	traceID := carrier.Get("trace-id")
	spanID := carrier.Get("span-id")

	// 如果没有有效的TraceID和SpanID，返回空上下文
	if traceID == "" || spanID == "" {
		return EmptySpanContext(), nil
	}

	// 创建span上下文
	ctx := &spanContext{
		traceID: traceID,
		spanID:  spanID,
	}

	// 提取baggage项
	for _, key := range carrier.Keys() {
		// 检查是否为baggage项
		if strings.HasPrefix(key, "baggage-") {
			// 移除前缀获取原始键名
			baggageKey := key[len("baggage-"):]
			baggageValue := carrier.Get(key)
			if baggageKey != "" && baggageValue != "" {
				ctx.SetBaggageItem(baggageKey, baggageValue)
			}
		}
	}

	return ctx, nil
}

// HTTPHeadersPropagator 实现HTTP头格式的上下文传播
// 针对HTTP环境优化的传播器实现
type HTTPHeadersPropagator struct{}

// NewHTTPHeadersPropagator 创建一个新的HTTP头传播器
// 用于在HTTP请求头中传播SpanContext
func NewHTTPHeadersPropagator() Propagator {
	return &HTTPHeadersPropagator{}
}

// Inject 将SpanContext注入到HTTP头载体中
// 使用HTTP标准的大驼峰命名格式
func (p *HTTPHeadersPropagator) Inject(ctx SpanContext, carrier Carrier) error {
	if !ctx.IsValid() || carrier == nil {
		return nil
	}

	// 设置标准的HTTP头格式
	carrier.Set("Trace-Id", ctx.TraceID())
	carrier.Set("Span-Id", ctx.SpanID())

	// 注入baggage项，使用Baggage-前缀
	ctx.ForeachBaggageItem(func(k, v string) bool {
		// 对键名进行编码，确保符合HTTP头规范
		encodedKey := "Baggage-" + sanitizeHTTPHeaderKey(k)
		carrier.Set(encodedKey, v)
		return true
	})

	return nil
}

// Extract 从HTTP头载体中提取SpanContext
// 支持多种常见的Trace ID HTTP头格式
func (p *HTTPHeadersPropagator) Extract(carrier Carrier) (SpanContext, error) {
	if carrier == nil {
		return EmptySpanContext(), nil
	}

	// 尝试从多种常见的HTTP头格式中提取TraceID和SpanID
	traceID := ""
	spanID := ""

	// 检查标准头格式
	if t := carrier.Get("Trace-Id"); t != "" {
		traceID = t
	}
	if s := carrier.Get("Span-Id"); s != "" {
		spanID = s
	}

	// 检查替代格式
	if traceID == "" {
		traceID = carrier.Get("trace-id")
	}
	if spanID == "" {
		spanID = carrier.Get("span-id")
	}

	// 如果没有有效的TraceID和SpanID，返回空上下文
	if traceID == "" || spanID == "" {
		return EmptySpanContext(), nil
	}

	// 创建span上下文
	ctx := &spanContext{
		traceID: traceID,
		spanID:  spanID,
	}

	// 提取baggage项
	for _, key := range carrier.Keys() {
		// 检查是否为baggage项（忽略大小写）
		if strings.HasPrefix(strings.ToLower(key), "baggage-") {
			// 移除前缀获取原始键名
			baggageKey := key[len("baggage-"):]
			baggageValue := carrier.Get(key)
			if baggageKey != "" && baggageValue != "" {
				ctx.SetBaggageItem(baggageKey, baggageValue)
			}
		}
	}

	return ctx, nil
}

// sanitizeHTTPHeaderKey 清理HTTP头键名，确保符合规范
// 移除或替换不符合HTTP头规范的字符
func sanitizeHTTPHeaderKey(key string) string {
	// 简单实现：将非字母数字和连字符的字符替换为连字符
	var result strings.Builder
	for _, c := range key {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' {
			result.WriteRune(c)
		} else {
			result.WriteRune('-')
		}
	}
	return result.String()
}

// BinaryPropagator 实现二进制格式的上下文传播
// 适用于需要高效二进制传输的场景
type BinaryPropagator struct{}

// NewBinaryPropagator 创建一个新的二进制传播器
// 用于在二进制格式的载体中传播SpanContext
func NewBinaryPropagator() Propagator {
	return &BinaryPropagator{}
}

// Inject 将SpanContext注入到二进制载体中
// 使用base64编码实现二进制传输
func (p *BinaryPropagator) Inject(ctx SpanContext, carrier Carrier) error {
	// 二进制传播器的实现示例
	// 在实际应用中，应该使用更高效的二进制编码方案

	if !ctx.IsValid() || carrier == nil {
		return nil
	}

	// 简单实现：使用base64编码文本格式
	// 实际应用中应该使用更高效的二进制序列化
	traceID := ctx.TraceID()
	spanID := ctx.SpanID()

	// 创建一个简单的二进制表示
	binaryData := []byte(traceID + ":" + spanID)
	encodedData := base64.StdEncoding.EncodeToString(binaryData)

	carrier.Set("binary-trace-context", encodedData)

	return nil
}

// Extract 从二进制载体中提取SpanContext
// 解析base64编码的二进制数据
func (p *BinaryPropagator) Extract(carrier Carrier) (SpanContext, error) {
	if carrier == nil {
		return EmptySpanContext(), nil
	}

	// 获取并解码二进制数据
	encodedData := carrier.Get("binary-trace-context")
	if encodedData == "" {
		return EmptySpanContext(), nil
	}

	// 解码base64数据
	binaryData, err := base64.StdEncoding.DecodeString(encodedData)
	if err != nil {
		return EmptySpanContext(), err
	}

	// 解析格式：traceID:spanID
	parts := strings.Split(string(binaryData), ":")
	if len(parts) < 2 {
		return EmptySpanContext(), nil
	}

	// 创建并返回SpanContext
	return &spanContext{
		traceID: parts[0],
		spanID:  parts[1],
	}, nil
}
