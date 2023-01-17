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
	Connections    map[valobj.ConnectionID]IConnection
	ConnectedHook  ConnectedHookFunc
	DisconnectHook DisconnectHookFunc
	ReceivedHook   ReceivedMsgHookFunc
}

func NewConnManager(opt ConnManagerOption) IConnManager {
	return &_ConnManager{
		ConnManagerOption: opt,
		Connections:       make(map[valobj.ConnectionID]IConnection),
	}
}

func (mgr *_ConnManager) Add(conn net.Conn) error {
	if len(mgr.Connections) >= mgr.MaxConn {
		return asuraError.ExceededMaxConnLimit
	}

	asuraConn := NewConnection(mgr, conn)

	mgr.Connections[asuraConn.GetID()] = asuraConn

	if mgr.ConnectedHook != nil {
		mgr.ConnectedHook(asuraConn)
	}

	return nil
}

func (mgr *_ConnManager) Remove(connID valobj.ConnectionID) {
	conn, ok := mgr.Connections[connID]
	if !ok {
		return
	}

	if mgr.DisconnectHook != nil {
		mgr.DisconnectHook(conn)
	}

	conn.Close()

	delete(mgr.Connections, connID)
}

func (mgr *_ConnManager) Get(connID valobj.ConnectionID) (IConnection, error) {
	conn, ok := mgr.Connections[connID]
	if !ok {
		return nil, asuraError.InvalidConnectionID
	}
	return conn, nil
}

func (mgr *_ConnManager) Clear() {
	for _, conn := range mgr.Connections {
		conn.Close()
	}

	mgr.Connections = make(map[valobj.ConnectionID]IConnection)
}

func (mgr *_ConnManager) OnConnected(cb ConnectedHookFunc) {
	mgr.ConnectedHook = cb
}

func (mgr *_ConnManager) OnReceived(cb ReceivedMsgHookFunc) {
	mgr.ReceivedHook = cb
}

func (mgr *_ConnManager) OnDisconnect(cb DisconnectHookFunc) {
	mgr.DisconnectHook = cb
}

func (mgr *_ConnManager) GetOnReceived() ReceivedMsgHookFunc {
	return mgr.ReceivedHook
}

func (mgr *_ConnManager) GetOnConnected() ConnectedHookFunc {
	return mgr.ConnectedHook
}

func (mgr *_ConnManager) GetOnDisconnect() DisconnectHookFunc {
	return mgr.DisconnectHook
}

func (mgr *_ConnManager) Option() ConnManagerOption {
	return mgr.ConnManagerOption
}
