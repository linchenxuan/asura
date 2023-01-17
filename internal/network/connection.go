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
	id      valobj.ConnectionID
	rawConn net.Conn
	connMgr IConnManager
	writeCh chan []byte
	ctx     context.Context
	cancel  context.CancelFunc
	state   valobj.ConnState
}

func NewConnection(mgr IConnManager, conn net.Conn) IConnection {
	ctx, cancel := context.WithCancel(context.Background())

	asuraConn := &_Connection{
		id:      valobj.GenConnectionID(),
		rawConn: conn,
		connMgr: mgr,
		ctx:     ctx,
		cancel:  cancel,
		writeCh: make(chan []byte, mgr.Option().WriteBuffer),
	}

	if hook := mgr.GetOnConnected(); hook != nil {
		hook(asuraConn)
	}

	go asuraConn.readServe()
	go asuraConn.writeServe()

	return asuraConn
}

func (connection *_Connection) Send(data []byte) error {
	if connection.state == valobj.ConnClosed {
		return asuraError.ConnectionClosed
	}

	_DPLocker.RLock()
	err := _DataPackInstance.Pack(connection.rawConn, data)
	_DPLocker.RUnlock()

	return err
}

func (connection *_Connection) SendAsync(data []byte) error {
	if connection.state == valobj.ConnClosed {
		return asuraError.ConnectionClosed
	}

	connection.writeCh <- data
	return nil
}

func (connection *_Connection) Close() {
	if disHook := connection.connMgr.GetOnDisconnect(); disHook != nil {
		disHook(connection)
	}

	connection.cancel()
	connection.rawConn.Close()
	connection.state = valobj.ConnClosed
}

func (connection *_Connection) GetID() valobj.ConnectionID {
	return connection.id
}

func (connection *_Connection) readServe() {
	defer connection.Close()

	for {
		select {
		case <-connection.ctx.Done():
			return
		default:
			_DPLocker.RLock()
			data, err := _DataPackInstance.Unpack(connection.rawConn)
			_DPLocker.RUnlock()
			// TODO 处理不同错误类型
			if err != nil {
				return
			}
			if recived := connection.connMgr.GetOnReceived(); recived != nil {
				recived(connection, data)
			}
		}
	}
}

func (connection *_Connection) writeServe() {
	for {
		select {
		case <-connection.ctx.Done():
			return
		case data := <-connection.writeCh:
			_DPLocker.RLock()
			_DataPackInstance.Pack(connection.rawConn, data)
			_DPLocker.RUnlock()
		}
	}
}
