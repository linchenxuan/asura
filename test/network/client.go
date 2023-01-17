// Copyright 2023 linchenxuan. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package network

import (
	"asura/internal/network"
	"encoding/json"
	"log"
	"net"
	"testing"
)

func startClient(t *testing.T, addr string) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}

	m := map[string]any{
		"name": "lcx",
		"age":  23,
	}

	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}

	dp := network.NewDataPack()

	dp.Pack(conn, data)

	for {
		msg, err := dp.Unpack(conn)
		if err != nil {
			log.Fatal(err)
		}
		t.Logf("recevied server msg: %v", string(msg))
	}
}
