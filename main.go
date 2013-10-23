package main

import (
	"fmt"
	"log"
	"os"
	"code.google.com/p/bencode-go"
)

type Files struct {
	Length int
	Path []string
}

type Info struct {
	Name string
	Length int
	Files []Files
	Pieces string
	PieceLength int "piece length"
}

type Metainfo struct {
	Info Info
	Announce string
	AnnounceList [][]string "announce-list"
}

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("Usage: %s: torrent\n", os.Args[0])
        }
	torrent := os.Args[1]

	file, err := os.Open(torrent)
	if err != nil {
		log.Fatal(err)
	}

	var m Metainfo

	err = bencode.Unmarshal(file, &m)
	if err != nil {
		log.Fatal(err)
	}

	if (m.Info.Length != 0) {
		fmt.Println(m.Info.Length)
	}
}
