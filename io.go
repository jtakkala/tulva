// Copyright 2013 Jari Takkala. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"crypto/sha1"
	"fmt"
	sysio "io"
	"path/filepath"
	"launchpad.net/tomb"
	"log"
	"os"
)

type IO struct {
	metaInfo MetaInfo
	files []*os.File
	t tomb.Tomb
}

// Reads in files and verifies them, returns a map of pieces we already have
func (io *IO) Verify() {
	pieceLength := io.metaInfo.Info.PieceLength

	// Create a buffer of size PieceLength
	buf := make([]byte, pieceLength)
	if len(io.metaInfo.Info.Files) > 0 {
		// Multiple File Mode
		var offset int64
		var piece, n, m int
		var err error
		h := sha1.New()
		// Iterate over each file
		for i, file := range io.metaInfo.Info.Files {
			for offset = 0; offset < int64(file.Length); {
				// Reset out hash
				h.Reset()
				// Read from file at offset, up to buf size or
				// less if last read was incomplete due to EOF
				n, err = io.files[i].ReadAt(buf[m:], offset)
				if err != nil {
					if err == sysio.EOF {
						fmt.Printf("%s %d\n", file.Path, n)
						// Reached EOF
						m += n
						break
					}
					log.Fatal(err)
				}
				// We have a full buf, generate a hash and compare with
				// corresponding pieces part of the torrent file
				h.Write(buf)
				fmt.Printf("sha1: %x %x\n", h.Sum(nil), io.metaInfo.Info.Pieces[piece:piece + 20])
				// Increment offset by number of bytes read
				offset += int64(n)
				m = 0
				// Increment piece by the length of a SHA-1 hash (20 bytes)
				piece += 20
			}
			if (n == pieceLength) {
				h.Write(buf)
				fmt.Printf("sha1: %x %x\n", h.Sum(nil), io.metaInfo.Info.Pieces[piece:piece + 20])
			}
		}
	} else {
		// Single File Mode
	}

}

func (io *IO) Stop() error {
	log.Println("IO : Stop : Stopping")
	io.t.Kill(nil)
	return io.t.Wait()
}

func checkError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func (io *IO) Init() {
	if len(io.metaInfo.Info.Files) > 0 {
		// Multiple File Mode
		directory := io.metaInfo.Info.Name
		// Create the directory if it doesn't exist
		if _, err := os.Stat(directory); os.IsNotExist(err) {
			err = os.Mkdir(directory, os.ModeDir | os.ModePerm)
			checkError(err)
		}
		err := os.Chdir(directory)
		checkError(err)
		for _, file := range io.metaInfo.Info.Files {
			// Create any sub-directories if required
			if len(file.Path) > 1 {
				directory = filepath.Join(file.Path[1:]...)
				if _, err := os.Stat(directory); os.IsNotExist(err) {
					err = os.MkdirAll(directory, os.ModeDir | os.ModePerm)
					checkError(err)
				}
			}
			// Create the file if it doesn't exist
			path := filepath.Join(file.Path...)
			var fh *os.File
			if _, err := os.Stat(path); os.IsNotExist(err) {
				// Create the file and return a handle
				fh, err = os.Create(path)
				checkError(err)
			} else {
				// Open the file and return a handle
				fh, err = os.Open(path)
				checkError(err)
			}
			io.files = append(io.files, fh)
		}
	} else {
		// Single File Mode
	}
}

func (io *IO) Run() {
	log.Println("IO : Run : Started")
	defer io.t.Done()
	defer log.Println("IO : Run : Completed")

	io.Init()
	io.Verify()

	for {
		select {
		case <- io.t.Dying():
			return
		}
	}
}
