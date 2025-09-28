package net

import (
	"encoding/binary"
	"errors"
)

// PRE_HEAD_SIZE PreHead长度.
const PRE_HEAD_SIZE = 12

// PreHead 头部.
type PreHead struct {
	HdrSize  uint32
	BodySize uint32
}

// EncodePreHead 编码preHead.
func EncodePreHead(hdr *PreHead) []byte {
	buf := make([]byte, PRE_HEAD_SIZE)
	binary.LittleEndian.PutUint32(buf[0:4], hdr.HdrSize)
	binary.LittleEndian.PutUint32(buf[4:8], hdr.BodySize)
	return buf
}

// DecodePreHead 解prehead.
func DecodePreHead(buf []byte) (*PreHead, error) {
	if len(buf) < PRE_HEAD_SIZE {
		return &PreHead{}, errors.New("buff too small")
	}
	hdr := &PreHead{
		HdrSize:  binary.LittleEndian.Uint32(buf),
		BodySize: binary.LittleEndian.Uint32(buf[4:8]),
	}
	if hdr.HdrSize == 0 {
		return hdr, errors.New("invalid")
	}
	return hdr, nil
}

// HeadDecodeResult .
type HeadDecodeResult struct {
	PreHead  *PreHead
	RouteHdr *RouteHead
	PkgHead  *PkgHead
	BodyData []byte
}
