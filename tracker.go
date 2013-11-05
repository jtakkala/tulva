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
	"time"
)

/*
// Peers dictionary model
type Peers struct {
	PeerId string "peer id"
	Ip     string
	Port   int
}
*/

type Peer struct {
	IP net.IP
	Port uint16
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
	Quit chan bool
}

func (tr *Tracker) Announce(t *Torrent, peerCh chan Peer, event string) {
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
	trackerRequest.Add("uploaded", string(0))
	trackerRequest.Add("downloaded", string(0))
	trackerRequest.Add("left", string(t.metaInfo.Info.Length))
//	trackerRequest.Add("uploaded", string(t.uploaded))
//	trackerRequest.Add("downloaded", string(t.downloaded))
//	trackerRequest.Add("left", string(metaInfo.Info.Length))
	trackerRequest.Add("compact", "1")
	if event != "" {
		trackerRequest.Add("event", event)
	}
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
	fmt.Printf("%#v\n", trackerResponse)

	// Peers in binary mode. Parse the response and decode peer IP + port
	var peer Peer
	for i := 0; i < len(trackerResponse.Peers); i += 6 {
		ip := net.IPv4(trackerResponse.Peers[i], trackerResponse.Peers[i+1], trackerResponse.Peers[i+2], trackerResponse.Peers[i+3])
		pport := uint16(trackerResponse.Peers[i+4]) << 8
		pport = pport | uint16(trackerResponse.Peers[i+5])
//		fmt.Printf("%s:%d ", ip, pport)
		peer.IP = ip
		peer.Port = pport
		select {
		case <- t.Quit:
			t.Quit <- true
			return
		case peerCh <- peer:
			continue
		}
	}
}

func (tr *Tracker) Run(t *Torrent, event chan string, peer chan Peer) {
	for {
		select {
		case e := <- event:
			go tr.Announce(t, peer, e)
			time.Sleep(5 * time.Second)
			t.Quit <- true
		case <- t.Quit:
			log.Println("Quitting Tracker")
			t.Quit <- true
			return
		}
	}
}
