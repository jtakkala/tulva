// Copyright 2013 Jari Takkala. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
//	"errors"
	"log"
	"math/rand"
	"net/http"
	_ "net/http/pprof"
	"os"
	"time"
)

// Unique client ID, encoded as '-' + 'TV' + <version number> + random digits
var PeerId = [20]byte{'-', 'T', 'V', '0', '0', '0', '1'}

// init initializes a random PeerId for this client
func init() {
	// Initialize PeerId
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 7; i < 20; i++ {
		PeerId[i] = byte(r.Intn(256))
	}
}

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("Usage: %s: <torrent file>\n", os.Args[0])
	}
	t, err := ParseTorrentFile(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	log.Println("main : main : Started")
	defer log.Println("main : main : Exiting")

	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	// Launch the torrent
	go t.Run()
	time.Sleep(5 * time.Second)
	log.Println("main : sending stop signal")
	t.Stop()
}
