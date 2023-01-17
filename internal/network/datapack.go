// Copyright 2023 linchenxuan. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package network

import (
	"encoding/binary"
	"io"
)

type IDataPack interface {
	// Pack 封包
	Pack(io.Writer, []byte) error
	// Unpack 解包
	Unpack(io.Reader) ([]byte, error)
}

type _DataPack struct {
}

func NewDataPack() IDataPack {
	return &_DataPack{}
}

func (dp *_DataPack) Pack(writer io.Writer, msg []byte) (err error) {
	if err = binary.Write(writer, binary.LittleEndian, uint32(len(msg))); err != nil {
		return
	}

	if err = binary.Write(writer, binary.LittleEndian, msg); err != nil {
		return
	}

	return
}

func (dp *_DataPack) Unpack(reader io.Reader) (msg []byte, err error) {
	var msgLen uint32
	if err = binary.Read(reader, binary.LittleEndian, &msgLen); err != nil {
		return
	}

	if msgLen > 0 {
		msg = make([]byte, msgLen)
		if err = binary.Read(reader, binary.LittleEndian, &msg); err != nil {
			return
		}
	}

	return
}
