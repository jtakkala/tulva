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
	file, err := os.Open("archlinux-2013.10.01-dual.iso.torrent")
	if err != nil {
		log.Fatal(err)
	}

	var m Metainfo

	err = bencode.Unmarshal(file, &m)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(m.Announce)
}
