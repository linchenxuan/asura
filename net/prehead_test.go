package net

import (
	"bytes"
	"testing"
)

func TestEncodePreHead(t *testing.T) {
	tests := []struct {
		name string
		hdr  *PreHead
	}{
		{
			name: "normal case",
			hdr: &PreHead{
				HdrSize:  100,
				BodySize: 200,
			},
		},
		{
			name: "zero values",
			hdr: &PreHead{
				HdrSize:  0,
				BodySize: 0,
			},
		},
		{
			name: "max uint32 values",
			hdr: &PreHead{
				HdrSize:  0xFFFFFFFF,
				BodySize: 0xFFFFFFFF,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EncodePreHead(tt.hdr)
			if len(result) != PRE_HEAD_SIZE {
				t.Errorf("EncodePreHead() length = %v, want %v", len(result), PRE_HEAD_SIZE)
			}
		})
	}
}

func TestDecodePreHead(t *testing.T) {
	// 创建测试数据
	createTestData := func(hdrSize, bodySize uint32) []byte {
		data := make([]byte, PRE_HEAD_SIZE)
		// 小端序编码前8字节
		data[0] = byte(hdrSize)
		data[1] = byte(hdrSize >> 8)
		data[2] = byte(hdrSize >> 16)
		data[3] = byte(hdrSize >> 24)

		data[4] = byte(bodySize)
		data[5] = byte(bodySize >> 8)
		data[6] = byte(bodySize >> 16)
		data[7] = byte(bodySize >> 24)
		return data
	}

	tests := []struct {
		name        string
		buf         []byte
		wantHdr     *PreHead
		wantErr     bool
		errContains string
	}{
		{
			name: "normal case",
			buf:  createTestData(100, 200),
			wantHdr: &PreHead{
				HdrSize:  100,
				BodySize: 200,
			},
			wantErr: false,
		},
		{
			name: "zero header size - should error",
			buf:  createTestData(0, 200),
			wantHdr: &PreHead{
				HdrSize:  0,
				BodySize: 200,
			},
			wantErr:     true,
			errContains: "invalid",
		},
		{
			name:        "buffer too small",
			buf:         []byte{1, 2, 3, 4, 5, 6, 7, 8},
			wantHdr:     &PreHead{},
			wantErr:     true,
			errContains: "buff too small",
		},
		{
			name:        "empty buffer",
			buf:         []byte{},
			wantHdr:     &PreHead{},
			wantErr:     true,
			errContains: "buff too small",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DecodePreHead(tt.buf)

			if (err != nil) != tt.wantErr {
				t.Errorf("DecodePreHead() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err != nil {
				if tt.errContains != "" && !bytes.Contains([]byte(err.Error()), []byte(tt.errContains)) {
					t.Errorf("DecodePreHead() error = %v, want error containing %v", err, tt.errContains)
				}
				return
			}

			if !tt.wantErr {
				if got.HdrSize != tt.wantHdr.HdrSize {
					t.Errorf("DecodePreHead() HdrSize = %v, want %v", got.HdrSize, tt.wantHdr.HdrSize)
				}
				if got.BodySize != tt.wantHdr.BodySize {
					t.Errorf("DecodePreHead() BodySize = %v, want %v", got.BodySize, tt.wantHdr.BodySize)
				}
			}
		})
	}
}

func TestPreHeadConstants(t *testing.T) {
	if PRE_HEAD_SIZE != 12 {
		t.Errorf("PRE_HEAD_SIZE = %v, want %v", PRE_HEAD_SIZE, 12)
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	testCases := []struct {
		name     string
		original *PreHead
	}{
		{
			name: "standard packet",
			original: &PreHead{
				HdrSize:  128,
				BodySize: 1024,
			},
		},
		{
			name: "small packet",
			original: &PreHead{
				HdrSize:  32,
				BodySize: 64,
			},
		},
		{
			name: "zero body",
			original: &PreHead{
				HdrSize:  64,
				BodySize: 0,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 编码
			encoded := EncodePreHead(tc.original)

			// 解码
			decoded, err := DecodePreHead(encoded)
			if err != nil {
				t.Fatalf("DecodePreHead() unexpected error = %v", err)
			}

			// 验证
			if decoded.HdrSize != tc.original.HdrSize {
				t.Errorf("Round trip HdrSize mismatch: got %v, want %v", decoded.HdrSize, tc.original.HdrSize)
			}
			if decoded.BodySize != tc.original.BodySize {
				t.Errorf("Round trip BodySize mismatch: got %v, want %v", decoded.BodySize, tc.original.BodySize)
			}
		})
	}
}

// BenchmarkEncodePreHead 基准测试编码性能
func BenchmarkEncodePreHead(b *testing.B) {
	hdr := &PreHead{
		HdrSize:  64,
		BodySize: 1024,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = EncodePreHead(hdr)
	}
}

// BenchmarkDecodePreHead 基准测试解码性能
func BenchmarkDecodePreHead(b *testing.B) {
	// 创建测试数据
	data := make([]byte, PRE_HEAD_SIZE)
	hdrSize := uint32(64)
	bodySize := uint32(1024)

	// 小端序编码
	data[0] = byte(hdrSize)
	data[1] = byte(hdrSize >> 8)
	data[2] = byte(hdrSize >> 16)
	data[3] = byte(hdrSize >> 24)

	data[4] = byte(bodySize)
	data[5] = byte(bodySize >> 8)
	data[6] = byte(bodySize >> 16)
	data[7] = byte(bodySize >> 24)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = DecodePreHead(data)
	}
}

// BenchmarkEncodeDecodePreHead 基准测试编码+解码性能
func BenchmarkEncodeDecodePreHead(b *testing.B) {
	hdr := &PreHead{
		HdrSize:  64,
		BodySize: 1024,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		encoded := EncodePreHead(hdr)
		_, _ = DecodePreHead(encoded)
	}
}
