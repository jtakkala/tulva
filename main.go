// Copyright 2013 Jari Takkala. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"code.google.com/p/bencode-go"
	"crypto/sha1"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"
)

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
type Metainfo struct {
	Info         Info
	Announce     string
	AnnounceList [][]string "announce-list"
	CreationDate int        "creation date"
	Comment      string
	CreatedBy    string "created by"
	Encoding     string
}

// Peers dictionary model
type Peers struct {
	PeerId string "peer id"
	ip     string
	port   int
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

// Unique client ID, encoded as '-' + 'TV' + <version number> + random digits
var PeerId = [20]byte{'-', 'T', 'V', '0', '0', '0', '1'}

// init initializes a random PeerId for this client
func init() {
	// Initialize PeerId
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 7; i < 20; i++ {
		PeerId[i] = byte(r.Intn(256))
	}
}

// parseTorrentFile opens the torrent filename specified and parses it,
// returning a Metainfo structure and SHA-1 hash of the Info dictionary.
func parseTorrentFile(torrent string) (metaInfo Metainfo, infoHash []byte, err error) {
	file, err := os.Open(torrent)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	// Decode the file into a generic bencode representation
	m, err := bencode.Decode(file)
	if err != nil {
		log.Fatal(err)
	}
	// WTF?: Understand the next line
	metaMap, ok := m.(map[string]interface{})
	if !ok {
		err = errors.New("Couldn't parse torrent file")
		return
	}
	infoDict, ok := metaMap["info"]
	if !ok {
		err = errors.New("Unable to locate info dict in torrent file")
		return
	}

	// Create an Info dict based on the decoded file
	var b bytes.Buffer
	bencode.Marshal(&b, infoDict)

	// Compute the info hash
	h := sha1.New()
	h.Write(b.Bytes())
	infoHash = append(infoHash, h.Sum(nil)...)

	// Populate the metaInfo structure
	file.Seek(0, 0)
	bencode.Unmarshal(file, &metaInfo)

	return
}

func main() {
	var announceUrl *url.URL

	if len(os.Args) != 2 {
		log.Fatalf("Usage: %s: <torrent file>\n", os.Args[0])
	}
	torrent := os.Args[1]

	// Get the Metainfo and Info hash for this torrent
	metaInfo, infoHash, err := parseTorrentFile(torrent)
	if err != nil {
		log.Fatal(err)
	}

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
