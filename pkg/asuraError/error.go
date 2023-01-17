// Copyright 2023 linchenxuan. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package asuraError

import "errors"

var (
	ExceededMaxConnLimit = errors.New("maximum connection limit exceeded")
	InvalidConnectionID  = errors.New("invalid connection id")
	ConnectionClosed     = errors.New("connection closed")
)
