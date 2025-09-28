package net

import (
	"bytes"
	"errors"

	"github.com/lcx/asura/codec"
	"google.golang.org/protobuf/proto"
)

// EncodeCSMsg encodes a TransSendPkg into separate byte slices for PreHead, PkgHead, and PkgBody.
// Returns a slice containing [PreHead, PkgHead, PkgBody] data.
// The format is: PreHead + PkgHead + PkgBody.
func EncodeCSMsg(pkg *TransSendPkg) (encodedBuf [][]byte, _ error) {
	datas := make([][]byte, 4)

	var err error
	datas[1], err = codec.Encode(pkg.PkgHdr, datas[1])
	if err != nil {
		return nil, err
	}

	datas[2], err = codec.Encode(pkg.Body, datas[2])
	if err != nil {
		return nil, err
	}

	pHdr := &PreHead{
		HdrSize:  uint32(len(datas[1])),
		BodySize: uint32(len(datas[2])),
	}

	datas[0] = EncodePreHead(pHdr)
	return datas, nil
}

// PackCSMsg packs all encoded data slices into a single buffer.
// This is useful for sending complete messages over the network.
func PackCSMsg(pkg *TransSendPkg, buf *bytes.Buffer) error {
	datas, err := EncodeCSMsg(pkg)
	if err != nil {
		return err
	}

	bufLen := 0
	for i := 0; i < len(datas); i++ {
		bufLen += len(datas[i])
	}
	buf.Grow(bufLen)

	for i := 0; i < len(datas); i++ {
		if _, err := buf.Write(datas[i]); err != nil {
			return err
		}
	}

	return nil
}

// PackCSPkg packs the message data into two parts: PreHead and the rest (PkgHead + PkgBody).
// Returns the PreHead byte slice separately while packing PkgHead + PkgBody into the buffer.
// This is useful for scenarios where PreHead needs to be processed separately.
func PackCSPkg(pkg *TransSendPkg, buf *bytes.Buffer) (preHeadBuf []byte, _ error) {
	datas, err := EncodeCSMsg(pkg)
	if err != nil {
		return nil, err
	}

	bufLen := 0
	// begin from 1 (skip PreHead)
	for i := 1; i < len(datas); i++ {
		bufLen += len(datas[i])
	}

	buf.Grow(bufLen)

	for i := 1; i < len(datas); i++ {
		if _, err := buf.Write(datas[i]); err != nil {
			return nil, err
		}
	}

	return datas[0], nil
}

// decodeCSMsgHead decodes the PreHead and PkgHead from the complete message data.
// Validates data length consistency and returns HeadDecodeResult containing parsed headers.
func decodeCSMsgHead(data []byte) (HeadDecodeResult, error) {
	var result HeadDecodeResult
	if len(data) < PRE_HEAD_SIZE {
		return result, errors.New("data too short for PreHead")
	}

	h, err := DecodePreHead(data)
	if err != nil {
		return result, err
	}

	if h.HdrSize+h.BodySize+PRE_HEAD_SIZE != uint32(len(data)) {
		return result, errors.New("data length mismatch with PreHead size, body size and tail size")
	}

	result.PreHead = h
	result.PkgHead = &PkgHead{}
	if err = codec.Decode(result.PkgHead, data[PRE_HEAD_SIZE:PRE_HEAD_SIZE+h.HdrSize]); err != nil {
		return result, err
	}
	result.BodyData = data[PRE_HEAD_SIZE+h.HdrSize : PRE_HEAD_SIZE+h.HdrSize+h.BodySize]
	return result, nil
}

// DecodeCSMsg decodes a complete message from byte data containing PreHead + PkgHead + PkgBody.
// Internally calls decodeCSMsgHead to parse headers, then creates and decodes the message body.
// Returns a TransRecvPkg containing the complete parsed message.
func DecodeCSMsg(data []byte, creator MsgCreator) (*TransRecvPkg, error) {
	if creator == nil {
		return &TransRecvPkg{}, errors.New("nil MsgCreateor")
	}
	result, err := decodeCSMsgHead(data)
	if err != nil {
		return &TransRecvPkg{}, err
	}
	body, err := creator.CreateMsg(result.PkgHead.GetMsgID())
	if err != nil {
		return &TransRecvPkg{}, err

	}

	if err := codec.Decode(body, result.BodyData); err != nil {
		return &TransRecvPkg{},
			err
	}
	return NewTransRecvPkgWithBody(nil, result.PkgHead, body), nil
}

// DecodeCSPkg decodes message data (PkgHead + PkgBody) using a pre-decoded PreHead.
// This function is used when PreHead has already been parsed separately.
// Validates that the data length matches PreHead sizes, then decodes PkgHead and PkgBody.
// Returns a TransRecvPkg containing the parsed message.
func DecodeCSPkg(h *PreHead, data []byte, creator MsgCreator) (*TransRecvPkg, error) {
	if creator == nil {
		return &TransRecvPkg{}, errors.New("nil MsgCreateor")
	}
	if h.HdrSize+h.BodySize != uint32(len(data)) {
		return &TransRecvPkg{},
			errors.New("data length mismatch with PreHead size, body size and tail size")
	}

	var hdr PkgHead
	err := codec.Decode(&hdr, data[:h.HdrSize])
	if err != nil {
		return &TransRecvPkg{}, err
	}

	var body proto.Message
	body, err = creator.CreateMsg(hdr.GetMsgID())
	if err != nil {
		return &TransRecvPkg{}, err
	}

	if err := codec.Decode(body, data[h.HdrSize:h.HdrSize+h.BodySize]); err != nil {
		return &TransRecvPkg{}, err
	}

	return NewTransRecvPkgWithBody(nil, &hdr, body), nil
}
