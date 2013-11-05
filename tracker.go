// Copyright 2013 Jari Takkala. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"code.google.com/p/bencode-go"
	"fmt"
	"launchpad.net/tomb"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
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
	announceUrl *url.URL
	t tomb.Tomb
}

func (tr *Tracker) Announce(t *Torrent, peerCh chan Peer, event string) {
	log.Println("Tracker : Announce : Started")
	defer log.Println("Tracker : Announce : Completed")

	// FIXME: Set our real listening port
	port := 6881

	// Build and encode the Tracker Request
	trackerRequest := url.Values{}
	trackerRequest.Set("info_hash", string(t.infoHash))
	trackerRequest.Add("peer_id", string(PeerId[:]))
	trackerRequest.Add("port", strconv.Itoa(port))
	trackerRequest.Add("uploaded", strconv.Itoa(0))
	trackerRequest.Add("downloaded", strconv.Itoa(0))
	trackerRequest.Add("left", strconv.Itoa(t.metaInfo.Info.Length))
//	trackerRequest.Add("uploaded", strconv.Itoa(t.uploaded))
//	trackerRequest.Add("downloaded", strconv.Itoa(t.downloaded))
	trackerRequest.Add("compact", "1")
	if event != "" {
		trackerRequest.Add("event", event)
	}
	tr.announceUrl.RawQuery = trackerRequest.Encode()

	// Make a request to the tracker
	log.Printf("Announce %s\n", tr.announceUrl.String())
	resp, err := http.Get(tr.announceUrl.String())
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
		peerCh <- peer
		/*
		select {
		case <- t.Quit:
			t.Quit <- true
			return
		case peerCh <- peer:
			continue
		}
		*/
	}
}

func (tr *Tracker) Stop() error {
	tr.t.Kill(nil)
	return tr.t.Wait()
}

func (tr *Tracker) Run(t *Torrent, event chan string, peer chan Peer) {
	log.Println("Tracker : Run : Started")
	defer tr.t.Done()
	defer log.Println("Tracker : Run : Completed")
	for {
		select {
		case e := <- event:
			go tr.Announce(t, peer, e)
			time.Sleep(time.Second)
		case <- tr.t.Dying():
			return
		}
	}
}
