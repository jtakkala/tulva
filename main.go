package main

import (
//	"crypto/sha1"
	"fmt"
	"log"
	"os"
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

	err = bencode.Marshal(os.Stdout, m.Info)
	if err != nil {
		log.Fatal(err)
	}

}
