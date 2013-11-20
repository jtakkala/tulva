// Copyright 2013 Jari Takkala. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"io"
	"launchpad.net/tomb"
	"log"
	"os"
	"path/filepath"
)

type diskIOPeerChans struct {
	// Channels to peers
	writePiece   chan Piece
	requestPiece chan RequestPieceDisk
}

type DiskIO struct {
	metaInfo MetaInfo
	files    []*os.File
	peerChans diskIOPeerChans
	t        tomb.Tomb
}

// checkHash accepts a byte buffer and pieceIndex, computes the SHA-1 hash of
// the buffer and returns true or false if it's correct.
func (diskio *DiskIO) checkHash(buf []byte, pieceIndex int) bool {
	h := sha1.New()
	h.Write(buf)
	if bytes.Equal(h.Sum(nil), []byte(diskio.metaInfo.Info.Pieces[pieceIndex:pieceIndex+h.Size()])) {
		return true
	}
	return false
}

// Verify reads in each file and verifies the SHA-1 checksum of each piece.
// Return the boolean list pieces that are correct.
func (diskio *DiskIO) Verify() (finishedPieces []bool) {
	log.Println("DiskIO : Verify : Started")
	defer log.Println("DiskIO : Verify : Completed")

	buf := make([]byte, diskio.metaInfo.Info.PieceLength)
	var pieceIndex, n int
	var err error

	fmt.Printf("Verifying downloaded files")
	if len(diskio.metaInfo.Info.Files) > 0 {
		// Multiple File Mode
		var m int
		// Iterate over each file
		for i, _ := range diskio.metaInfo.Info.Files {
			for offset := int64(0); ; offset += int64(n) {
				// Read from file at offset, up to buf size or
				// less if last read was incomplete due to EOF
				fmt.Printf(".")
				n, err = diskio.files[i].ReadAt(buf[m:], offset)
				if err != nil {
					if err == io.EOF {
						// Reached EOF. Increment partial read counter by bytes read
						m += n
						break
					}
					log.Fatal(err)
				}
				// We have a full buf, check the hash of buf and
				// append the result to the finished pieces
				finishedPieces = append(finishedPieces, diskio.checkHash(buf, pieceIndex))
				// Reset partial read counter
				m = 0
				// Increment piece by the length of a SHA-1 hash (20 bytes)
				pieceIndex += 20
			}
		}
		// If the final iteration resulted in a partial read, then
		// check the hash of it and append the result
		if m > 0 {
			finishedPieces = append(finishedPieces, diskio.checkHash(buf[:m], pieceIndex))
		}
	} else {
		// Single File Mode
		for offset := int64(0); ; offset += int64(n) {
			// Read from file at offset, up to buf size or
			// less if last read was incomplete due to EOF
			fmt.Printf(".")
			n, err = diskio.files[0].ReadAt(buf, offset)
			if err != nil {
				if err == io.EOF {
					// Reached EOF
					break
				}
				log.Fatal(err)
			}
			// We have a full buf, check the hash of buf and
			// append the result to the finished pieces
			finishedPieces = append(finishedPieces, diskio.checkHash(buf, pieceIndex))
			// Increment piece by the length of a SHA-1 hash (20 bytes)
			pieceIndex += 20
		}
		// If the final iteration resulted in a partial read, then compute a hash
		if n > 0 {
			finishedPieces = append(finishedPieces, diskio.checkHash(buf[:n], pieceIndex))
		}
	}
	fmt.Println()

	return finishedPieces
}

func checkError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

// openOrCreateFile opens the named file or creates it if it doesn't already
// exist. If successful it returns a file handle that can be used for I/O.
func openOrCreateFile(name string) (file *os.File) {
	// Create the file if it doesn't exist
	if _, err := os.Stat(name); os.IsNotExist(err) {
		// Create the file and return a handle
		file, err = os.Create(name)
		checkError(err)
	} else {
		// Open the file and return a handle
		file, err = os.Open(name)
		checkError(err)
	}
	return
}

func NewDiskIO(metaInfo MetaInfo) *DiskIO {
	diskio := new(DiskIO)
	diskio.metaInfo = metaInfo
	diskio.peerChans.writePiece = make(chan Piece)
	diskio.peerChans.requestPiece = make(chan RequestPieceDisk)
	return diskio
}

func (diskio *DiskIO) Init() {
	if len(diskio.metaInfo.Info.Files) > 0 {
		// Multiple File Mode
		directory := diskio.metaInfo.Info.Name
		// Create the directory if it doesn't exist
		if _, err := os.Stat(directory); os.IsNotExist(err) {
			err = os.Mkdir(directory, os.ModeDir|os.ModePerm)
			checkError(err)
		}
		err := os.Chdir(directory)
		checkError(err)
		for _, file := range diskio.metaInfo.Info.Files {
			// Create any sub-directories if required
			if len(file.Path) > 1 {
				directory = filepath.Join(file.Path[1:]...)
				if _, err := os.Stat(directory); os.IsNotExist(err) {
					err = os.MkdirAll(directory, os.ModeDir|os.ModePerm)
					checkError(err)
				}
			}
			// Create the file if it doesn't exist
			name := filepath.Join(file.Path...)
			diskio.files = append(diskio.files, openOrCreateFile(name))
		}
	} else {
		// Single File Mode
		diskio.files = append(diskio.files, openOrCreateFile(diskio.metaInfo.Info.Name))
	}
}

func (diskio *DiskIO) Stop() error {
	log.Println("DiskIO : Stop : Stopping")
	diskio.t.Kill(nil)
	return diskio.t.Wait()
}

func (diskio *DiskIO) Run() {
	log.Println("DiskIO : Run : Started")
	defer diskio.t.Done()
	defer log.Println("DiskIO : Run : Completed")

	diskio.Init()
	finishedPieces := diskio.Verify()
	fmt.Println(finishedPieces)

	for {
		select {
		case <-diskio.t.Dying():
			return
		}
	}
}
