package main

import (
	"github.com/jackpal/bencode-go"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type HttpTracker tracker

func (tr *HttpTracker) Announce(event int) {
	log.Println("Tracker : Announce : Started")
	defer log.Println("Tracker : Announce : Completed")

	if tr.infoHash == nil {
		log.Println("Tracker : Announce : Error: infoHash undefined")
		return
	}

	// Build and encode the Tracker Request
	urlParams := url.Values{}
	urlParams.Set("info_hash", string(tr.infoHash))
	urlParams.Set("peer_id", string(PeerID[:]))
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
			go func() { tr.peerChans.peers <- PeerTuple{peerIP, peerPort} }()
		}
	}
}

func (tr *HttpTracker) Run() {
	log.Printf("Tracker : Run : Started (%s)\n", tr.announceURL)
	defer log.Printf("Tracker : Run : Completed (%s)\n", tr.announceURL)

	tr.timer = make(<-chan time.Time)
	tr.Announce(Started)

	for {
		select {
		case <-tr.quit:
			log.Println("Tracker : Stop : Stopping")
			tr.Announce(Stopped)
			return
		case <-tr.completedCh:
			go tr.Announce(Completed)
		case <-tr.timer:
			log.Printf("Tracker : Run : Interval Timer Expired (%s)\n", tr.announceURL)
			go tr.Announce(Interval)
		case stats := <-tr.peerChans.stats:
			log.Println("read from stats", stats)
		}
	}
}
