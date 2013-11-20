// Copyright 2013 Jari Takkala. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"code.google.com/p/bencode-go"
	"encoding/hex"
	//"fmt"
	"launchpad.net/tomb"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Possible reasons for tracker requests with the event parameter
const (
	Interval int = iota
	Started
	Stopped
	Completed
)

type trackerChans struct {
	completed chan bool
	stats     chan Stats
	peers     chan PeerTuple
}

type trackerManager struct {
	chans trackerChans
	port  uint16
	t     tomb.Tomb
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

type tracker struct {
	announceURL *url.URL
	response    TrackerResponse
	chans       trackerChans
	timer     <-chan time.Time
	stats       Stats
	key         string
	port        uint16
	infoHash    []byte
	t           tomb.Tomb
}

func initKey() (string) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	key := make([]byte, 4)
	for i := 0; i < 4; i++ {
		key[i] = byte(r.Intn(256))
	}
	return hex.EncodeToString(key)
}

func (tr *tracker) Announce(event int) {
	log.Println("Tracker : Announce : Started")
	defer log.Println("Tracker : Announce : Completed")

	if tr.infoHash == nil {
		log.Println("Tracker : Announce : Error: infoHash undefined")
		return
	}

	// Build and encode the Tracker Request
	urlParams := url.Values{}
	urlParams.Set("info_hash", string(tr.infoHash))
	urlParams.Set("peer_id", string(PeerId[:]))
	urlParams.Set("key", tr.key)
	urlParams.Set("port", strconv.FormatUint(uint64(tr.port), 10))
	urlParams.Set("uploaded", strconv.Itoa(tr.stats.Uploaded))
	urlParams.Set("downloaded", strconv.Itoa(tr.stats.Downloaded))
	urlParams.Set("left", strconv.Itoa(tr.stats.Left))
	urlParams.Set("compact", "1")
	switch event {
	case Started:
		urlParams.Set("event", "started")
	case Stopped:
		urlParams.Set("event", "stopped")
	case Completed:
		urlParams.Set("event", "completed")
	}
	announceURL := *tr.announceURL
	announceURL.RawQuery = urlParams.Encode()

	// Send a request to the Tracker
	log.Printf("Announce: %s\n", announceURL.String())
	resp, err := http.Get(announceURL.String())
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	// Unmarshall the Tracker Response
	err = bencode.Unmarshal(resp.Body, &tr.response)
	if err != nil {
		log.Println(err)
		return
	}

	// Schedule a timer to poll this announce URL every interval
	if tr.response.Interval != 0 && event != Stopped {
		nextAnnounce := time.Second * time.Duration(tr.response.Interval)
		log.Printf("Tracker : Announce : Scheduling next announce in %v\n", nextAnnounce)
		tr.timer = time.After(nextAnnounce)
	}

	// If we're not stopping, send the list of peers to the peers channel
	if event != Stopped {
		// Parse peers in binary mode and return peer IP + port
		for i := 0; i < len(tr.response.Peers); i += 6 {
			peerIP := net.IPv4(tr.response.Peers[i], tr.response.Peers[i+1], tr.response.Peers[i+2], tr.response.Peers[i+3])
			peerPort := uint16(tr.response.Peers[i+4]) << 8
			peerPort = peerPort | uint16(tr.response.Peers[i+5])
			// Send the peer IP+port to the Torrent Manager
			go func() { tr.chans.peers <- PeerTuple{peerIP, peerPort} }()
		}
	}
}

func (tr *tracker) Stop() error {
	log.Println("Tracker : Stop : Stopping")
	tr.Announce(Stopped)
	tr.t.Kill(nil)
	return tr.t.Wait()
}

func (tr *tracker) Run() {
	log.Printf("Tracker : Run : Started (%s)\n", tr.announceURL)
	defer tr.t.Done()
	defer log.Printf("Tracker : Run : Completed (%s)\n", tr.announceURL)

	tr.timer = make(<-chan time.Time)
	tr.Announce(Started)

	for {
		select {
		case <-tr.t.Dying():
			return
		case <-tr.chans.completed:
			go tr.Announce(Completed)
		case <-tr.timer:
			log.Printf("Tracker : Run : Interval Timer Expired (%s)\n", tr.announceURL)
			go tr.Announce(Interval)
		case stats := <-tr.chans.stats:
			log.Println("read from stats", stats)
		}
	}
}

func newTracker(key string, chans trackerChans, port uint16, infoHash []byte, announce string) *tracker {
	announceURL, err := url.Parse(announce)
	if err != nil {
		log.Fatal(err)
	}
	if len(key) < 8 {
		log.Fatalf("newTracker: key too short %d (expected at least 8 bytes)\n", len(key))
	}
	tracker := &tracker{key: key, chans: chans, port: port, infoHash: infoHash, announceURL: announceURL}
	tracker.infoHash = make([]byte, len(infoHash))
	copy(tracker.infoHash, infoHash)
	return tracker
}

func NewTrackerManager(port uint16) *trackerManager {
	chans := new(trackerChans)
	chans.completed = make(chan bool)
	chans.peers = make(chan PeerTuple)
	chans.stats = make(chan Stats)
	return &trackerManager{chans: *chans, port: port}
}

func (tm *trackerManager) Stop() error {
	log.Println("TrackerManager : Stop : Stopping")
	tm.t.Kill(nil)
	return tm.t.Wait()
}

// Run spawns trackers for each announce URL
func (tm *trackerManager) Run(m MetaInfo, infoHash []byte) {
	log.Println("TrackerManager : Run : Started")
	defer tm.t.Done()
	defer log.Println("TrackerManager : Run : Completed")

	// TODO: Handle multiple announce URL's
	/*
		for announceURL := m.AnnounceList {
			tr := new(Tracker)
			tr.metaInfo = m
			tr.announceURL = announceURL
			tr.Run()
		}
	*/

	tr := newTracker(initKey(), tm.chans, tm.port, infoHash, m.Announce)
	go tr.Run()

	for {
		select {
		case <-tm.t.Dying():
			tr.Stop()
			return
		}
	}
}
