package main

import (
	"bytes"
	"crypto/sha1"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
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

func parseTorrent(torrent string) (metainfo Metainfo, infoHash []byte, err error) {
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

	// populate the metainfo structure
	file.Seek(0, 0)
	bencode.Unmarshal(file, &metainfo)

	return
}

func main() {
	var announceUrl *url.URL

	if len(os.Args) != 2 {
		log.Fatalf("Usage: %s: torrent\n", os.Args[0])
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

	tracker_request := url.Values{}
	tracker_request.Set("info_hash", string(infoHash))
	tracker_request.Add("peer_id", string(PeerId[:]))
	tracker_request.Add("port", port)
	tracker_request.Add("uploaded", uploaded)
	tracker_request.Add("downloaded", downloaded)
	tracker_request.Add("left", string(metaInfo.Info.Length))
	tracker_request.Add("compact", "1")
	announceUrl.RawQuery = tracker_request.Encode()

	resp, err := http.Get(announceUrl.String())
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s\n", body)
}
