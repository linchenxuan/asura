// Copyright 2023 linchenxuan. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package network

type (
	ServerStartHookFunc func()

	ServerCloseHookFunc func()

	ConnectedHookFunc func(conn IConnection)

	DisconnectHookFunc func(conn IConnection)

	ReceivedMsgHookFunc func(conn IConnection, msg []byte)
)
