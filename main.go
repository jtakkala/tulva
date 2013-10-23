package main

import (
	"crypto/sha1"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"
	"code.google.com/p/bencode-go"
)

type Files struct {
	Length int "length"
	Path []string "path"
}

type Info struct {
	Name string "name"
	Length int "length"
//	Files []Files "files"
	Pieces string "pieces"
	PieceLength int "piece length"
}

type Metainfo struct {
	Info Info "info"
	Announce string "announce"
	AnnounceList [][]string "announce-list"
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

func main() {
	var m Metainfo

	if len(os.Args) != 2 {
		log.Fatalf("Usage: %s: torrent\n", os.Args[0])
        }
	torrent := os.Args[1]

	file, err := os.Open(torrent)
	if err != nil {
		log.Fatal(err)
	}

	err = bencode.Unmarshal(file, &m)
	if err != nil {
		log.Fatal(err)
	}

	if (m.Info.Length != 0) {
		log.Println("Single File Mode")
		fmt.Println(m.Info.Length)
	} else {
		log.Fatal("Multiple File Mode not implemented")
	}

	h := sha1.New()

	err = bencode.Marshal(h, m.Info)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(h.Sum(nil))

}
