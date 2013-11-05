// Copyright 2013 Jari Takkala. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"log"
)

type Torrent struct {
	metaInfo MetaInfo
	infoHash []byte
	left int
	Quit chan bool
	peer chan Peer
}

// Multiple File Mode
type Files struct {
	Length int
	Md5sum string
	Path   []string
}

// Info dictionary
type Info struct {
	PieceLength int "piece length"
	Pieces      string
	Private     int
	Name        string
	Length      int
	Md5sum      string
	Files       []Files
}

// Metainfo structure
type MetaInfo struct {
	Info         Info
	Announce     string
	AnnounceList [][]string "announce-list"
	CreationDate int        "creation date"
	Comment      string
	CreatedBy    string "created by"
	Encoding     string
}

// Init completes the initalization of the Torrent structure
func (t *Torrent) Init() {
	t.Quit = make(chan bool)
	// Initialize bytes left to download
	if len(t.metaInfo.Info.Files) > 0 {
		for _, file := range(t.metaInfo.Info.Files) {
			t.left += file.Length
		}
	} else {
		t.left = t.metaInfo.Info.Length
	}
	if t.left == 0 {
		log.Fatal("Unable to deterimine bytes left to download")
	}
}

// Run starts the Torrent session and orchestrates all the child processes
func (t *Torrent) Run() {
	t.Init()

	command := make(chan string)
	peer := make(chan Peer)
	tr := new(Tracker)
	go tr.Run(t, command, peer)

	command <- "started"
	for {
		select {
		case <- t.Quit:
			log.Println("Quitting Torrent")
			t.Quit <- true
			return
		case peer := <- peer:
			fmt.Println("Peer:", peer.IP.String(), peer.Port)
		}
	}
	fmt.Println("Exiting t.Run()")
}

