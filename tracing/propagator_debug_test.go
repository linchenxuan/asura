package tracing

import (
	"testing"
)

// 直接测试TextMapPropagator的功能，不通过全局tracer
func TestTextMapPropagatorDirect(t *testing.T) {
	// 创建传播器
	propagator := NewTextMapPropagator()
	if propagator == nil {
		t.Fatal("NewTextMapPropagator returned nil")
	}

	// 创建一个带有baggage项的span上下文
	originalCtx := &spanContext{
		traceID: "test-trace-id",
		spanID:  "test-span-id",
		baggage: make(map[string]string),
	}
	originalCtx.SetBaggageItem("test-key", "test-value")

	// 创建载体
	carrier := NewTextMapCarrier()

	// 注入上下文
	err := propagator.Inject(originalCtx, carrier)
	if err != nil {
		t.Errorf("Expected no error when injecting, got %v", err)
	}

	// 检查载体中的值
	traceID := carrier.Get("trace-id")
	if traceID != "test-trace-id" {
		t.Errorf("Expected trace-id to be 'test-trace-id', got %s", traceID)
	}

	spanID := carrier.Get("span-id")
	if spanID != "test-span-id" {
		t.Errorf("Expected span-id to be 'test-span-id', got %s", spanID)
	}

	baggageValue := carrier.Get("baggage-test-key")
	if baggageValue != "test-value" {
		t.Errorf("Expected baggage-test-key to be 'test-value', got %s", baggageValue)
	}

	// 提取上下文
	extractedCtx, err := propagator.Extract(carrier)
	if err != nil {
		t.Errorf("Expected no error when extracting, got %v", err)
	}

	// 检查提取的上下文
	if extractedCtx.TraceID() != "test-trace-id" {
		t.Errorf("Expected extracted trace-id to be 'test-trace-id', got %s", extractedCtx.TraceID())
	}

	if extractedCtx.SpanID() != "test-span-id" {
		t.Errorf("Expected extracted span-id to be 'test-span-id', got %s", extractedCtx.SpanID())
	}

	// 关键测试：检查baggage项是否正确传播
	extractedBaggage := extractedCtx.GetBaggageItem("test-key")
	if extractedBaggage != "test-value" {
		t.Errorf("Expected baggage item 'test-key' to be 'test-value', got %s", extractedBaggage)
	}
}
