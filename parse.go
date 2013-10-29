// Copyright 2013 Jari Takkala. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"code.google.com/p/bencode-go"
	"crypto/sha1"
	"errors"
	"log"
	"os"
)

// ParseTorrentFile opens the torrent filename specified and parses it,
// returning a Torrent structure with the MetaInfo and SHA-1 hash of the
// Info dictionary.
func ParseTorrentFile(filename string) (torrent Torrent, err error) {
	file, err := os.Open(filename)
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
	torrent.infoHash = append(torrent.infoHash, h.Sum(nil)...)

	// Populate the metaInfo structure
	file.Seek(0, 0)
	bencode.Unmarshal(file, &torrent.metaInfo)

	return
}

