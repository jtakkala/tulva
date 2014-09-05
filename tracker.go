// Copyright 2013-2014 Jari Takkala. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/hex"
	"log"
	"math/rand"
	"net/url"
	"strings"
	"time"
)

// Possible reasons for tracker requests with the event parameter
const (
	Interval int = iota
	Started
	Stopped
	Completed
)

type trackerPeerChans struct {
	stats chan Stats
	peers chan PeerTuple
}

type trackerManager struct {
	peerChans trackerPeerChans
	port      uint16
	quit      chan struct{}
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

type Tracker interface {
	Announce(event int)
	Run()
}

type tracker struct {
	announceURL *url.URL
	response    TrackerResponse
	peerChans   trackerPeerChans
	completedCh chan bool
	timer       <-chan time.Time
	stats       Stats
	key         string
	port        uint16
	infoHash    []byte
	quit        chan struct{}
}

func initKey() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	key := make([]byte, 4)
	for i := 0; i < 4; i++ {
		key[i] = byte(r.Intn(256))
	}
	return hex.EncodeToString(key)
}

func newTracker(key string, chans trackerPeerChans, port uint16, infoHash []byte, announce string, quit chan struct{}) Tracker {
	announceURL, err := url.Parse(announce)
	if err != nil {
		log.Fatal(err)
	}
	if len(key) < 8 {
		log.Fatalf("newTracker: key too short %d (expected at least 8 bytes)\n", len(key))
	}

	if strings.HasPrefix(announceURL.String(), "udp://") {
		tracker := NewUdpTracker(key, chans, port, infoHash, announceURL)
		tracker.infoHash = make([]byte, len(infoHash))
		tracker.quit = quit
		copy(tracker.infoHash, infoHash)
		return tracker
	}

	tracker := &HttpTracker{key: key, peerChans: chans, port: port, infoHash: infoHash, announceURL: announceURL}
	tracker.infoHash = make([]byte, len(infoHash))
	tracker.quit = quit
	copy(tracker.infoHash, infoHash)

	return tracker
}

func NewTrackerManager(port uint16) *trackerManager {
	chans := new(trackerPeerChans)
	chans.peers = make(chan PeerTuple)
	chans.stats = make(chan Stats)
	return &trackerManager{peerChans: *chans, port: port, quit: make(chan struct{})}
}

// Run spawns trackers for each announce URL
func (tm *trackerManager) Run(m MetaInfo, infoHash []byte) {
	log.Println("TrackerManager : Run : Started")
	defer log.Println("TrackerManager : Run : Completed")

	// Handle multiple announce URL's
	for i := range m.AnnounceList {
		for _, announceURL := range m.AnnounceList[i] {
			log.Println("TrackerManager : Starting Tracker", announceURL)
			tr := newTracker(initKey(), tm.peerChans, tm.port, infoHash, announceURL, tm.quit)
			go tr.Run()
		}
	}
	// Handle a single announce URL
	if len(m.Announce) > 0 {
		log.Println("TrackerManager : Starting Tracker", m.Announce)
		tr := newTracker(initKey(), tm.peerChans, tm.port, infoHash, m.Announce, tm.quit)
		go tr.Run()
	}

	for {
		select {
		case <-tm.quit:
			log.Println("TrackerManager : Run : Stopping")
			return
		}
	}
}
