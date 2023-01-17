// Copyright 2023 linchenxuan. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package network

import (
	"net"
)

type IServer interface {
	// Addr 监听地址
	Addr() string

	// Start 启动服务器
	Start()

	// Stop 关闭服务器
	Stop()

	// OnStart 监听服务器启动
	OnStart(handler ServerStartHookFunc)

	// OnStop 监听服务器关闭
	OnStop(handler ServerCloseHookFunc)

	// OnConnected 监听连接打开
	OnConnected(handler ConnectedHookFunc)

	// OnDisconnect 监听连接断开
	OnDisconnect(handler DisconnectHookFunc)

	// OnReceived 监听接收消息
	OnReceived(handler ReceivedMsgHookFunc)
}

type _Server struct {
	ServerOption
	connManager IConnManager // 连接管理

	startHook ServerStartHookFunc // 服务器启动hook函数
	stopHook  ServerCloseHookFunc // 服务器关闭hook函数
}

func NewServer(opt ServerOption) IServer {
	return &_Server{
		ServerOption: opt,
		connManager:  NewConnManager(opt.ConnManager),
	}
}

func (server *_Server) Addr() string {
	return server.ServerOption.Addr
}

func (server *_Server) Start() {
	listener, err := net.Listen("tcp", server.ServerOption.Addr)
	if err != nil {
		panic(err)
	}
	defer listener.Close()

	if server.startHook != nil {
		server.startHook()
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}

		// TODO handle error
		server.connManager.Add(conn)
	}
}

func (server *_Server) Stop() {
	server.connManager.Clear()

	if server.stopHook != nil {
		server.stopHook()
	}
}

func (server *_Server) OnStart(cb ServerStartHookFunc) {
	server.startHook = cb
}

func (server *_Server) OnStop(cb ServerCloseHookFunc) {
	server.stopHook = cb
}

func (server *_Server) OnConnected(cb ConnectedHookFunc) {
	server.connManager.OnConnected(cb)
}

func (server *_Server) OnReceived(cb ReceivedMsgHookFunc) {
	server.connManager.OnReceived(cb)
}

func (server *_Server) OnDisconnect(cb DisconnectHookFunc) {
	server.connManager.OnDisconnect(cb)
}
