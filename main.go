// Copyright 2013 Jari Takkala. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	//"errors"
	"log"
	"math/rand"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Unique client ID, encoded as '-' + 'TV' + <version number> + random digits
var PeerID = [20]byte{'-', 'T', 'V', '0', '0', '0', '1'}

// init initializes a random PeerID for this client
func init() {
	// Initialize PeerID
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 7; i < 20; i++ {
		PeerID[i] = byte(r.Intn(256))
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

	// Launch pprof
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	// Signal handler to catch Ctrl-C and SIGTERM from 'kill' command
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		log.Println("Received Interrupt")
		t.Stop()
		os.Exit(1)
	}()

	// Launch the torrent
	t.Run()
}
