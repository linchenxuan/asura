// Copyright 2023 linchenxuan. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package network

import (
	"asura/pkg/asuraError"
	"asura/pkg/valobj"
	"net"
)

type IConnManager interface {
	// Add 添加链接
	Add(conn net.Conn) error

	// Remove 删除连接
	Remove(connID valobj.ConnectionID)

	// Get 利用ConnID获取链接
	Get(connID valobj.ConnectionID) (IConnection, error)

	// Clear 清理所有链接
	Clear()

	// OnConnected 监听连接打开
	OnConnected(handler ConnectedHookFunc)

	// OnDisconnect 监听连接断开
	OnDisconnect(handler DisconnectHookFunc)

	// OnReceived 监听接收消息
	OnReceived(handler ReceivedMsgHookFunc)

	GetOnConnected() ConnectedHookFunc

	GetOnDisconnect() DisconnectHookFunc

	GetOnReceived() ReceivedMsgHookFunc

	Option() ConnManagerOption
}

type _ConnManager struct {
	ConnManagerOption
	connections    map[valobj.ConnectionID]IConnection
	connectedHook  ConnectedHookFunc
	disconnectHook DisconnectHookFunc
	receivedHook   ReceivedMsgHookFunc
}

func NewConnManager(opt ConnManagerOption) IConnManager {
	return &_ConnManager{
		ConnManagerOption: opt,
		connections:       make(map[valobj.ConnectionID]IConnection),
	}
}

func (mgr *_ConnManager) Add(conn net.Conn) error {
	if len(mgr.connections) >= mgr.MaxConn {
		return asuraError.ExceededMaxConnLimit
	}

	asuraConn := NewConnection(mgr, conn)

	mgr.connections[asuraConn.GetID()] = asuraConn

	if mgr.connectedHook != nil {
		mgr.connectedHook(asuraConn)
	}

	return nil
}

func (mgr *_ConnManager) Remove(connID valobj.ConnectionID) {
	conn, ok := mgr.connections[connID]
	if !ok {
		return
	}

	if mgr.disconnectHook != nil {
		mgr.disconnectHook(conn)
	}

	conn.Close()

	delete(mgr.connections, connID)
}

func (mgr *_ConnManager) Get(connID valobj.ConnectionID) (IConnection, error) {
	conn, ok := mgr.connections[connID]
	if !ok {
		return nil, asuraError.InvalidConnectionID
	}
	return conn, nil
}

func (mgr *_ConnManager) Clear() {
	for _, conn := range mgr.connections {
		conn.Close()
	}

	mgr.connections = make(map[valobj.ConnectionID]IConnection)
}

func (mgr *_ConnManager) OnConnected(cb ConnectedHookFunc) {
	mgr.connectedHook = cb
}

func (mgr *_ConnManager) OnReceived(cb ReceivedMsgHookFunc) {
	mgr.receivedHook = cb
}

func (mgr *_ConnManager) OnDisconnect(cb DisconnectHookFunc) {
	mgr.disconnectHook = cb
}

func (mgr *_ConnManager) GetOnReceived() ReceivedMsgHookFunc {
	return mgr.receivedHook
}

func (mgr *_ConnManager) GetOnConnected() ConnectedHookFunc {
	return mgr.connectedHook
}

func (mgr *_ConnManager) GetOnDisconnect() DisconnectHookFunc {
	return mgr.disconnectHook
}

func (mgr *_ConnManager) Option() ConnManagerOption {
	return mgr.ConnManagerOption
}
