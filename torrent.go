// Copyright 2013 Jari Takkala. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
//	"fmt"
	"launchpad.net/tomb"
	"log"
)

type Torrent struct {
	metaInfo MetaInfo
	infoHash []byte
	left int
	peer chan Peer
	t tomb.Tomb
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

func (t *Torrent) Stop() error {
	t.t.Kill(nil)
	return t.t.Wait()
}

// Run starts the Torrent session and orchestrates all the child processes
func (t *Torrent) Run() {
	log.Println("Torrent : Run : Started")
	defer t.t.Done()
	defer log.Println("Torrent : Run : Completed")
	t.Init()

	trackerEvent := make(chan string)
	peersCh := make(chan Peer)
	tr := new(Tracker)
	go tr.Run(t, trackerEvent, peersCh)

	peers := make(map[string]uint16)

	trackerEvent <- "started"
	for {
		select {
		case <- t.t.Dying():
			tr.Stop()
			return
		case peer := <- peersCh:
			peers[peer.IP.String()] = peer.Port
		}
	}
}

