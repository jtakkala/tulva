// Copyright 2013 Jari Takkala. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
//	"errors"
	"log"
	"math/rand"
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

	// Launch the torrent
	go t.Run()
	/*
	for {
		select {
		case <- t.Quit:
			fmt.Println("Got quit in main()")
			break
		}
	}
	*/
	time.Sleep(time.Second)
	t.Quit <- true
	<-t.Quit
	log.Println("main : main : Exiting")
}
