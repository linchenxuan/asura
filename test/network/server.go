// Copyright 2023 linchenxuan. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package network

import (
	"asura/internal/network"
	"testing"
)

func startServer(t *testing.T, addr string) {
	server := network.NewServer(network.ServerOption{
		Addr: addr,
		ConnManager: network.ConnManagerOption{
			MaxConn:     10,
			WriteBuffer: 100,
		},
	})

	server.OnStart(func() {
		t.Log("server started")
	})

	server.OnStop(func() {
		t.Log("server stopped")
	})

	server.OnConnected(func(conn network.IConnection) {
		t.Logf("client connected,connId: %v", conn.GetID())
	})

	server.OnDisconnect(func(conn network.IConnection) {
		t.Logf("client disconnected,connId: %v", conn.GetID())
	})

	server.OnReceived(func(conn network.IConnection, msg []byte) {
		t.Logf("received client msg , connId: %v , msg: %v", conn.GetID(), string(msg))
		conn.Send(msg)
		conn.SendAsync(msg)
	})

	go server.Start()
}
