package metrics

import (
	"testing"
)

func TestValueOperations(t *testing.T) {
	var v1 Value = 42.5
	var v2 Value = 7.5

	// 测试加法
	result := v1 + v2
	if result != 50.0 {
		t.Errorf("Expected 50.0, got %v", result)
	}

	// 测试减法
	result = v1 - v2
	if result != 35.0 {
		t.Errorf("Expected 35.0, got %v", result)
	}

	// 测试乘法
	result = v1 * v2
	if result != 318.75 {
		t.Errorf("Expected 318.75, got %v", result)
	}

	// 测试除法
	result = v1 / v2
	if result != 5.666666666666667 {
		t.Errorf("Expected 5.666666666666667, got %v", result)
	}
}

func TestDimensionOperations(t *testing.T) {
	dim := Dimension{
		"host":   "server1",
		"region": "us-west",
		"env":    "prod",
	}

	// 测试获取值
	if dim["host"] != "server1" {
		t.Errorf("Expected 'server1', got '%s'", dim["host"])
	}

	if dim["region"] != "us-west" {
		t.Errorf("Expected 'us-west', got '%s'", dim["region"])
	}

	// 测试设置值
	dim["host"] = "server2"
	if dim["host"] != "server2" {
		t.Errorf("Expected 'server2', got '%s'", dim["host"])
	}

	// 测试添加新键值
	dim["version"] = "1.0.0"
	if dim["version"] != "1.0.0" {
		t.Errorf("Expected '1.0.0', got '%s'", dim["version"])
	}

	// 测试删除键值
	delete(dim, "env")
	if _, exists := dim["env"]; exists {
		t.Error("Expected 'env' key to be deleted")
	}
}

func TestDimensionComparison(t *testing.T) {
	dim1 := Dimension{"host": "server1", "region": "us-west"}
	dim2 := Dimension{"host": "server1", "region": "us-west"}
	dim3 := Dimension{"host": "server2", "region": "us-west"}
	dim4 := Dimension{"host": "server1", "region": "us-east"}
	dim5 := Dimension{"host": "server1"} // 缺少region

	// 测试相同的维度
	if len(dim1) != len(dim2) {
		t.Error("Expected dimensions to have same length")
	}

	for k, v := range dim1 {
		if dim2[k] != v {
			t.Errorf("Expected dimension '%s' to be '%s', got '%s'", k, v, dim2[k])
		}
	}

	// 测试不同的维度值
	if dim1["host"] == dim3["host"] {
		t.Error("Expected different host values")
	}

	if dim1["region"] == dim4["region"] {
		t.Error("Expected different region values")
	}

	// 测试不同长度的维度
	if len(dim1) == len(dim5) {
		t.Error("Expected different dimension lengths")
	}
}

func TestPolicyConstants(t *testing.T) {
	// 测试策略常量的值
	if PolicyNone != 0 {
		t.Errorf("Expected PolicyNone to be 0, got %d", PolicyNone)
	}

	if PolicySet != 1 {
		t.Errorf("Expected PolicySet to be 1, got %d", PolicySet)
	}

	if PolicySum != 2 {
		t.Errorf("Expected PolicySum to be 2, got %d", PolicySum)
	}

	if PolicyAvg != 3 {
		t.Errorf("Expected PolicyAvg to be 3, got %d", PolicyAvg)
	}

	if PolicyMax != 4 {
		t.Errorf("Expected PolicyMax to be 4, got %d", PolicyMax)
	}

	if PolicyMin != 5 {
		t.Errorf("Expected PolicyMin to be 5, got %d", PolicyMin)
	}

	if PolicyMid != 6 {
		t.Errorf("Expected PolicyMid to be 6, got %d", PolicyMid)
	}

	if PolicyStopwatch != 7 {
		t.Errorf("Expected PolicyStopwatch to be 7, got %d", PolicyStopwatch)
	}

	if PolicyHistogram != 8 {
		t.Errorf("Expected PolicyHistogram to be 8, got %d", PolicyHistogram)
	}
}
