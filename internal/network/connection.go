// Copyright 2023 linchenxuan. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package network

import (
	"asura/pkg/asuraError"
	"asura/pkg/valobj"
	"context"
	"net"
	"sync"
)

var (
	_DataPackInstance = NewDataPack()
	_DPLocker         sync.RWMutex
)

func SetDataPack(dp IDataPack) {
	_DPLocker.Lock()
	defer _DPLocker.Unlock()

	_DataPackInstance = dp
}

type IConnection interface {
	Send(data []byte) error
	SendAsync(data []byte) error
	Close()
	GetID() valobj.ConnectionID
}

type _Connection struct {
	Id      valobj.ConnectionID
	Conn    net.Conn
	ConnMgr IConnManager
	WriteCh chan []byte
	Ctx     context.Context
	Cancel  context.CancelFunc
	State   valobj.ConnState
}

func NewConnection(mgr IConnManager, conn net.Conn) IConnection {
	ctx, cancel := context.WithCancel(context.Background())

	asuraConn := &_Connection{
		Id:      valobj.GenConnectionID(),
		Conn:    conn,
		ConnMgr: mgr,
		Ctx:     ctx,
		Cancel:  cancel,
		WriteCh: make(chan []byte, mgr.Option().WriteBuffer),
	}

	if hook := mgr.GetOnConnected(); hook != nil {
		hook(asuraConn)
	}

	go asuraConn.readServe()
	go asuraConn.writeServe()

	return asuraConn
}

func (conn *_Connection) Send(data []byte) error {
	if conn.State == valobj.ConnClosed {
		return asuraError.ConnectionClosed
	}

	_DPLocker.RLock()
	err := _DataPackInstance.Pack(conn.Conn, data)
	_DPLocker.RUnlock()

	return err
}

func (conn *_Connection) SendAsync(data []byte) error {
	if conn.State == valobj.ConnClosed {
		return asuraError.ConnectionClosed
	}

	conn.WriteCh <- data
	return nil
}

func (conn *_Connection) Close() {
	if disHook := conn.ConnMgr.GetOnDisconnect(); disHook != nil {
		disHook(conn)
	}

	conn.Cancel()
	conn.Conn.Close()
	conn.State = valobj.ConnClosed
}

func (conn *_Connection) GetID() valobj.ConnectionID {
	return conn.Id
}

func (conn *_Connection) readServe() {
	defer conn.Close()

	for {
		select {
		case <-conn.Ctx.Done():
			return
		default:
			_DPLocker.RLock()
			data, err := _DataPackInstance.Unpack(conn.Conn)
			_DPLocker.RUnlock()
			// TODO 处理不同错误类型
			if err != nil {
				return
			}
			if recived := conn.ConnMgr.GetOnReceived(); recived != nil {
				recived(conn, data)
			}
		}
	}
}

func (conn *_Connection) writeServe() {
	for {
		select {
		case <-conn.Ctx.Done():
			return
		case data := <-conn.WriteCh:
			_DPLocker.RLock()
			_DataPackInstance.Pack(conn.Conn, data)
			_DPLocker.RUnlock()
		}
	}
}
