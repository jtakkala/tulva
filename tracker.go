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

const (
	_ int = iota
	Started
	Stopped
	Completed
)

type Peer struct {
	IP net.IP
	Port uint16
}

type Stats struct {
	Uploaded int
	Downloaded int
	Left int
}

type TrackerManager struct {
	completedCh chan bool
	statsCh chan Stats
	peersCh chan Peer
	t tomb.Tomb
}

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
	response TrackerResponse
	completedCh chan bool
	statsCh chan Stats
	peersCh chan Peer
	stats Stats
	infoHash []byte
	t tomb.Tomb
}

func (tr *Tracker) Announce(event int) {
	log.Println("Tracker : Announce : Started")
	defer log.Println("Tracker : Announce : Completed")

	if (tr.infoHash == nil) {
		log.Println("Tracker : Announce : Error: infoHash undefined")
		return
	}

	// FIXME: Set our real listening port
	port := 6881

	// Build and encode the Tracker Request
	urlParams := url.Values{}
	urlParams.Set("info_hash", string(tr.infoHash))
	urlParams.Add("peer_id", string(PeerId[:]))
	urlParams.Add("port", strconv.Itoa(port))
	urlParams.Add("uploaded", strconv.Itoa(tr.stats.Uploaded))
	urlParams.Add("downloaded", strconv.Itoa(tr.stats.Downloaded))
	urlParams.Add("left", strconv.Itoa(tr.stats.Left))
	urlParams.Add("compact", "1")
	switch (event) {
	case Started:
		urlParams.Add("event", "started")
	case Stopped:
		urlParams.Add("event", "stopped")
	case Completed:
		urlParams.Add("event", "completed")
	}
	tr.announceUrl.RawQuery = urlParams.Encode()

	// Make a request to the tracker
	log.Printf("Announce: %s\n", tr.announceUrl.String())
	resp, err := http.Get(tr.announceUrl.String())
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	bencode.Unmarshal(resp.Body, &tr.response)

	if event != Stopped {
		// Parse peers in binary mode and return peer IP + port
		var peer Peer
		for i := 0; i < len(tr.response.Peers); i += 6 {
			ip := net.IPv4(tr.response.Peers[i], tr.response.Peers[i+1], tr.response.Peers[i+2], tr.response.Peers[i+3])
			pport := uint16(tr.response.Peers[i+4]) << 8
			pport = pport | uint16(tr.response.Peers[i+5])
			peer.IP = ip
			peer.Port = pport
			tr.peersCh <- peer
		}
	}
}

func (tr *Tracker) Stop() error {
	tr.Announce(Stopped)
	tr.t.Kill(nil)
	return tr.t.Wait()
}

func (tr *Tracker) Run() {
	log.Printf("Tracker : Run : Started (%s)\n", tr.announceUrl)
	defer tr.t.Done()
	defer log.Printf("Tracker : Run : Completed (%s)\n", tr.announceUrl)

	// TODO: Start this in a go-routine?
	go tr.Announce(Started)
	for {
		select {
		case <- tr.t.Dying():
			return
		case <- tr.completedCh:
			go tr.Announce(Completed)
		}
	}
}

func (trm *TrackerManager) Stop() error {
	trm.t.Kill(nil)
	return trm.t.Wait()
}

// NewTracker spanws trackers
func (trm *TrackerManager) Run(m MetaInfo, infoHash []byte) {
	log.Println("Tracker : TrackerManager : Started")
	defer trm.t.Done()
	defer log.Println("Tracker : TrackerManager : Completed")

	// TODO: Handle multiple announce URL's
	/*
	for announceUrl := m.AnnounceList {
		tr := new(Tracker)
		tr.metaInfo = m
		tr.announceUrl = announceUrl
		tr.Run()
	}
	*/

	tr := new(Tracker)
	tr.statsCh = trm.statsCh
	tr.peersCh = trm.peersCh
	tr.completedCh = trm.completedCh
	tr.infoHash = make([]byte, len(infoHash))
	copy(tr.infoHash, infoHash)
	tr.announceUrl, _ = url.Parse(m.Announce)
	go tr.Run()

	for {
		select {
		case <- trm.t.Dying():
			tr.Stop()
			return
		}
	}
}
