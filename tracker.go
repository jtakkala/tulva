// Copyright 2013 Jari Takkala. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"code.google.com/p/bencode-go"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
)

// Peers dictionary model
type Peers struct {
	PeerId string "peer id"
	Ip     string
	Port   int
}

// Tracker Response
type TrackerResponse struct {
	FailureReason  string "failure reason"
	WarningMessage string "warning message"
	Interval       int
	MinInterval    int    "min interval"
	TrackerId      string "tracker id"
	Complete       int
	Incomplete     int
	Peers          string "peers"
	//TODO: Figure out how to handle dict of peers
//	Peers          []Peers "peers"
}

type Tracker struct {
	url string
	trackerResponse TrackerResponse
}

func (tr *Tracker) Announce(t *Torrent) {
	var announceUrl *url.URL

	// Select the tracker to connect to, if it's a list, select the first
	// one in the list. TODO: If no response from first tracker in list,
	// then try the next one, and so on.
	if len(t.metaInfo.AnnounceList) > 0 {
		announceUrl, _ = url.Parse(t.metaInfo.AnnounceList[0][0])
	} else {
		announceUrl, _ = url.Parse(t.metaInfo.Announce)
	}
	// TODO: Implement UDP mode
	if announceUrl.Scheme != "http" {
		log.Fatalf("URL Scheme: %s not supported\n", announceUrl.Scheme)
	}

	// FIXME: Set our real listening port
	port := 6881

	// Build and encode the Tracker Request
	trackerRequest := url.Values{}
	trackerRequest.Set("info_hash", string(t.infoHash))
	trackerRequest.Add("peer_id", string(PeerId[:]))
	trackerRequest.Add("port", string(port))
	trackerRequest.Add("uploaded", string(t.uploaded))
	trackerRequest.Add("downloaded", string(t.downloaded))
//	trackerRequest.Add("left", string(metaInfo.Info.Length))
	trackerRequest.Add("compact", "1")
	announceUrl.RawQuery = trackerRequest.Encode()

	// Make a request to the tracker
	log.Printf("Announce %s\n", announceUrl.String())
	resp, err := http.Get(announceUrl.String())
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	var trackerResponse TrackerResponse

	bencode.Unmarshal(resp.Body, &trackerResponse)
	fmt.Printf("%x\n", trackerResponse)

	// Peers in binary mode. Parse the response and decode peer IP + port
	fmt.Printf("Peers: ")
	for i := 0; i < len(trackerResponse.Peers); i += 6 {
		ip := net.IPv4(trackerResponse.Peers[i], trackerResponse.Peers[i+1], trackerResponse.Peers[i+2], trackerResponse.Peers[i+3])
		pport := uint32(trackerResponse.Peers[i+4]) << 32
		pport = pport | uint32(trackerResponse.Peers[i+5])
		// TODO: Assign result to a TCPAddr type here
		fmt.Printf("%s:%d", ip, port)
	}
	fmt.Println()
}

func (tr *Tracker) Run(t *Torrent, trackerMonitor chan bool) {
	tr.Announce(t)
	// do something, loop forever and update very interval
	trackerMonitor <- true
}
