package main

import (
	"bytes"
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
	"code.google.com/p/bencode-go"
)

type Files struct {
	Length int
	Md5sum string
	Path []string
}

type Info struct {
	PieceLength int "piece length"
	Pieces string
	Private int
	Name string
	Length int
	Md5sum string
	Files []Files
}

type Metainfo struct {
	Info Info
	Announce string
	AnnounceList [][]string "announce-list"
	CreationDate int "creation date"
	Comment string
	CreatedBy string "created by"
	Encoding string
}

type Peers struct {
	PeerId string "peer id"
	ip string
	port int
}

type TrackerResponse struct {
	FailureReason string "failure reason"
	WarningMessage string "warning message"
	Interval int
	MinInterval int "min interval"
	TrackerId string "tracker id"
	Complete int
	Incomplete int
	Peers string "peers"
//	Peers []Peers "peers"
}

var PeerId = [20]byte {
	'-',
	'T',
	'V',
	'0',
	'0',
	'0',
	'1',
}

func init() {
	// Initialize PeerId
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 7; i < 20; i++ {
		PeerId[i] = byte(r.Intn(256))
	}
}

func parseTorrent(torrent string) (metaInfo Metainfo, infoHash []byte, err error) {
	file, err := os.Open(torrent)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	m, err := bencode.Decode(file)
	if err != nil {
		log.Fatal(err)
	}
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

	var b bytes.Buffer
	bencode.Marshal(&b, infoDict)

	// compute the info hash
	h := sha1.New()
	h.Write(b.Bytes())
	infoHash = append(infoHash, h.Sum(nil)...)

	// populate the metaInfo structure
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

	metaInfo, infoHash, err := parseTorrent(torrent)
	if (err != nil) {
		log.Fatal(err)
	}

	// select the tracker to connect to
	if len(metaInfo.AnnounceList) > 0 {
		announceUrl, err = url.Parse(metaInfo.AnnounceList[0][0])
	} else {
		announceUrl, err = url.Parse(metaInfo.Announce)
	}
	if (err != nil) {
		log.Fatal(err)
	}
	if (announceUrl.Scheme != "http") {
		log.Fatalf("URL Scheme: %s not supported\n", announceUrl.Scheme)
	}

	// statically set these for now
	port := "6881"
	downloaded := "0"
	uploaded := "0"

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

	resp, err := http.Get(announceUrl.String())
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	var trackerResponse TrackerResponse

	bencode.Unmarshal(resp.Body, &trackerResponse)
	fmt.Printf("%x\n", trackerResponse)
	for i := 0; i < len(trackerResponse.Peers); i += 6 {
		ip := net.IPv4(trackerResponse.Peers[i], trackerResponse.Peers[i + 1], trackerResponse.Peers[i + 2], trackerResponse.Peers[i + 3])
		pport := uint32(trackerResponse.Peers[i + 4]) << 32
		pport = pport | uint32(trackerResponse.Peers[i + 5])
		fmt.Println(ip, port)
	}
}
