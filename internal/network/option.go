// Copyright 2023 linchenxuan. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package network

type ServerOption struct {
	Addr        string
	ConnManager ConnManagerOption
}

type ConnManagerOption struct {
	MaxConn     int
	WriteBuffer int
}
