
// Copyright 2013 Jari Takkala. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"code.google.com/p/bencode-go"
//	"fmt"
	"launchpad.net/tomb"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
)

type Peer struct {
	IP net.IP
	Port uint16
}

type AnnounceResponse struct {
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

type Announcer struct {
	announceUrl *url.URL
	torrent Torrent
	response AnnounceResponse
	event string
	t tomb.Tomb
}

func (ar *Announcer) Announce(peerCh chan Peer) {
	log.Println("Announcer : Announce : Started")
	defer log.Println("Announcer : Announce : Completed")

	if (ar.torrent.infoHash == nil) {
		log.Println("Announce: Error: infoHash undefined")
		return
	}

	// FIXME: Set our real listening port
	port := 6881

	// Build and encode the Tracker Request
	u := url.Values{}
	u.Set("info_hash", string(ar.torrent.infoHash))
	u.Add("peer_id", string(PeerId[:]))
	u.Add("port", strconv.Itoa(port))
	u.Add("uploaded", strconv.Itoa(0))
	u.Add("downloaded", strconv.Itoa(0))
	u.Add("left", strconv.Itoa(ar.torrent.metaInfo.Info.Length))
//	u.Add("uploaded", strconv.Itoa(t.uploaded))
//	u.Add("downloaded", strconv.Itoa(t.downloaded))
	u.Add("compact", "1")
	if ar.event != "" {
		u.Add("event", ar.event)
	}
	ar.announceUrl.RawQuery = u.Encode()

	// Make a request to the tracker
	log.Printf("Announce %s\n", ar.announceUrl.String())
	resp, err := http.Get(ar.announceUrl.String())
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	bencode.Unmarshal(resp.Body, &ar.response)

	// Peers in binary mode. Parse the response and decode peer IP + port
	var peer Peer
	for i := 0; i < len(ar.response.Peers); i += 6 {
		ip := net.IPv4(ar.response.Peers[i], ar.response.Peers[i+1], ar.response.Peers[i+2], ar.response.Peers[i+3])
		pport := uint16(ar.response.Peers[i+4]) << 8
		pport = pport | uint16(ar.response.Peers[i+5])
//		fmt.Printf("%s:%d ", ip, pport)
		peer.IP = ip
		peer.Port = pport
		peerCh <- peer
	}
	// Unset ar.event
	ar.event = ""
}

func (ar *Announcer) Stop() error {
	ar.t.Kill(nil)
	return ar.t.Wait()
}

func (ar *Announcer) Run(torrent chan Torrent, announce chan bool, event chan string, peerCh chan Peer) {
	log.Println("Announcer : Run : Started")
	defer ar.t.Done()
	defer log.Println("Announcer : Run : Completed")
	for {
		select {
		case <- announce:
			log.Println("Announcer: received announce request")
			ar.Announce(peerCh)
		case e := <- event:
			log.Println("Announcer: received event", ar.event)
			ar.event = e
		case t := <- torrent:
			log.Printf("Announcer: received torrent with info_hash %x\n", t.infoHash)
			ar.torrent = t
		case <- ar.t.Dying():
			return
		}
	}
}
