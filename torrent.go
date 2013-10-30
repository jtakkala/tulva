// Copyright 2013 Jari Takkala. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"fmt"
)

type Torrent struct {
	tracker Tracker
	metaInfo MetaInfo
	infoHash []byte
	uploaded int
	downloaded int
	left int
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
	if t.left != 0 {
		err := errors.New("Unable to deterimine bytes left to download")
		fmt.Println(err)
		// TODO: Bail out here
	}
}

// Run starts the Torrent session and orchestrates all the child processes
func (t *Torrent) Run(complete chan bool) {
	t.Init()
	fmt.Printf("%#v\n", t)
	
	// Spawn the tracker and wait for it to complete
	trackerMonitor := make(chan bool)
	go t.tracker.Run(t, trackerMonitor)
	<-trackerMonitor

	complete <- true
}

