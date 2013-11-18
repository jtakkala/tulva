// Copyright 2013 Jari Takkala. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	sysio "io"
	"launchpad.net/tomb"
	"log"
	"os"
	"path/filepath"
)

type IO struct {
	metaInfo MetaInfo
	files    []*os.File
	t        tomb.Tomb
}

// checkHash accepts a byte buffer and pieceIndex, computes the SHA-1 hash of
// the buffer and returns true or false if it's correct.
func (io *IO) checkHash(buf []byte, pieceIndex int) bool {
	h := sha1.New()
	h.Write(buf)
	if bytes.Equal(h.Sum(nil), []byte(io.metaInfo.Info.Pieces[pieceIndex:pieceIndex+h.Size()])) {
		return true
	}
	return false
}

// Verify reads in each file and verifies the SHA-1 checksum of each piece.
// Return the boolean list pieces that are correct.
func (io *IO) Verify() (finishedPieces []bool) {
	log.Println("IO : Verify : Started")
	defer log.Println("IO : Verify : Completed")

	pieceLength := io.metaInfo.Info.PieceLength
	buf := make([]byte, pieceLength)
	var pieceIndex, n int
	var err error

	if len(io.metaInfo.Info.Files) > 0 {
		// Multiple File Mode
		var m int
		// Iterate over each file
		for i, _ := range io.metaInfo.Info.Files {
			for offset := int64(0); ; offset += int64(n) {
				// Read from file at offset, up to buf size or
				// less if last read was incomplete due to EOF
				n, err = io.files[i].ReadAt(buf[m:], offset)
				if err != nil {
					if err == sysio.EOF {
						// Reached EOF. Increment partial read counter by bytes read
						m += n
						break
					}
					log.Fatal(err)
				}
				// We have a full buf, check the hash of buf and
				// append the result to the finished pieces
				finishedPieces = append(finishedPieces, io.checkHash(buf, pieceIndex))
				// Reset partial read counter
				m = 0
				// Increment piece by the length of a SHA-1 hash (20 bytes)
				pieceIndex += 20
			}
		}
		// If the final iteration resulted in a partial read, then
		// check the hash of it and append the result
		if m > 0 {
			finishedPieces = append(finishedPieces, io.checkHash(buf[:m], pieceIndex))
		}
	} else {
		// Single File Mode
		for offset := int64(0); ; offset += int64(n) {
			// Read from file at offset, up to buf size or
			// less if last read was incomplete due to EOF
			n, err = io.files[0].ReadAt(buf, offset)
			if err != nil {
				if err == sysio.EOF {
					// Reached EOF
					break
				}
				log.Fatal(err)
			}
			// We have a full buf, check the hash of buf and
			// append the result to the finished pieces
			finishedPieces = append(finishedPieces, io.checkHash(buf, pieceIndex))
			// Increment piece by the length of a SHA-1 hash (20 bytes)
			pieceIndex += 20
		}
		// If the final iteration resulted in a partial read, then compute a hash
		if n > 0 {
			finishedPieces = append(finishedPieces, io.checkHash(buf[:n], pieceIndex))
		}
	}

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

func (io *IO) Init() {
	if len(io.metaInfo.Info.Files) > 0 {
		// Multiple File Mode
		directory := io.metaInfo.Info.Name
		// Create the directory if it doesn't exist
		if _, err := os.Stat(directory); os.IsNotExist(err) {
			err = os.Mkdir(directory, os.ModeDir|os.ModePerm)
			checkError(err)
		}
		err := os.Chdir(directory)
		checkError(err)
		for _, file := range io.metaInfo.Info.Files {
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
			io.files = append(io.files, openOrCreateFile(name))
		}
	} else {
		// Single File Mode
		io.files = append(io.files, openOrCreateFile(io.metaInfo.Info.Name))
	}
}

func (io *IO) Stop() error {
	log.Println("IO : Stop : Stopping")
	io.t.Kill(nil)
	return io.t.Wait()
}

func (io *IO) Run() {
	log.Println("IO : Run : Started")
	defer io.t.Done()
	defer log.Println("IO : Run : Completed")

	io.Init()
	finishedPieces := io.Verify()
	fmt.Println(finishedPieces)

	for {
		select {
		case <-io.t.Dying():
			return
		}
	}
}
