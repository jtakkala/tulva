// Copyright 2013 Jari Takkala. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
)

type Torrent struct {
	metaInfo metaInfo
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
type metaInfo struct {
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
}

func (t *Torrent) Run(complete chan bool) {
	fmt.Printf("%#v\n", t)
	complete <- true
}

/*
func doTorrent() {
	var announceUrl *url.URL

	// Select the tracker to connect to, if it's a list, select the first
	// one in the list. TODO: If no response from first tracker in list,
	// then try the next one, and so on.
	if len(metaInfo.AnnounceList) > 0 {
		announceUrl, err = url.Parse(metaInfo.AnnounceList[0][0])
	} else {
		announceUrl, err = url.Parse(metaInfo.Announce)
	}
	if err != nil {
		log.Fatal(err)
	}
	// TODO: Implement UDP mode
	if announceUrl.Scheme != "http" {
		log.Fatalf("URL Scheme: %s not supported\n", announceUrl.Scheme)
	}

	// FIXME: Statically set our port and download/upload metrics
	port := "6881"
	downloaded := "0"
	uploaded := "0"

	// Build and encode the Tracker Request
	trackerRequest := url.Values{}
	trackerRequest.Set("info_hash", string(infoHash))
	trackerRequest.Add("peer_id", string(PeerId[:]))
	trackerRequest.Add("port", port)
	trackerRequest.Add("uploaded", uploaded)
	trackerRequest.Add("downloaded", downloaded)
	trackerRequest.Add("left", string(metaInfo.Info.Length))
	trackerRequest.Add("compact", "1")
	announceUrl.RawQuery = trackerRequest.Encode()

	log.Printf("Requesting %s\n", announceUrl.String())

	// Make a request to the tracker
	resp, err := http.Get(announceUrl.String())
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	var trackerResponse TrackerResponse

	bencode.Unmarshal(resp.Body, &trackerResponse)
	fmt.Printf("%x\n", trackerResponse)

	// Peers in binary mode. Parse the response and decode peer IP + port
	for i := 0; i < len(trackerResponse.Peers); i += 6 {
		ip := net.IPv4(trackerResponse.Peers[i], trackerResponse.Peers[i+1], trackerResponse.Peers[i+2], trackerResponse.Peers[i+3])
		pport := uint32(trackerResponse.Peers[i+4]) << 32
		pport = pport | uint32(trackerResponse.Peers[i+5])
		// TODO: Assign result to a TCPAddr type here
		fmt.Println(ip, port)
	}
}
*/

/*
func TorrentMonitor() {
}
*/
