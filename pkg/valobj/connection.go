// Copyright 2023 linchenxuan. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package valobj

import "github.com/rs/xid"

type ConnectionID string

func GenConnectionID() ConnectionID {
	return ConnectionID(xid.New().String())
}

type ConnState int

const (
	ConnClosed ConnState = iota + 1
)
