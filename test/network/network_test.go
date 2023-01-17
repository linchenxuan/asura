// Copyright 2023 linchenxuan. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package network

import (
	"os"
	"os/signal"
	"syscall"
	"testing"
)

func TestNetwork(t *testing.T) {
	qc := make(chan os.Signal, 1)
	signal.Notify(qc, syscall.SIGINT, syscall.SIGTERM)

	addr := "127.0.0.1:9999"
	go startServer(t, addr)
	go startClient(t, addr)

	<-qc
}
