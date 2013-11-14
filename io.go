// Copyright 2013 Jari Takkala. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
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

func (io *IO) checkHash(buf []byte, pieceIndex int) {
	h := sha1.New()
	h.Write(buf)
	if bytes.Equal(h.Sum(nil), []byte(io.metaInfo.Info.Pieces[pieceIndex:pieceIndex + h.Size()])) {
		fmt.Printf("SHA1 match: %x\n", h.Sum(nil))
	}
}

// Reads in files and verifies them, returns a map of pieces we already have
func (io *IO) Verify() {
	pieceLength := io.metaInfo.Info.PieceLength
	buf := make([]byte, pieceLength)

	if len(io.metaInfo.Info.Files) > 0 {
		// Multiple File Mode
		var piece, n, m int
		var err error
		// Iterate over each file
		for i, _ := range io.metaInfo.Info.Files {
			for offset := int64(0); ; {
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
				// We have a full buf, generate a hash and compare with
				// corresponding pieces part of the torrent file
				io.checkHash(buf, piece)
				// Reset partial read counter
				m = 0
				// Increment offset by number of bytes read
				offset += int64(n)
				// Increment piece by the length of a SHA-1 hash (20 bytes)
				piece += 20
			}
		}
		// If the final iteration resulted in a partial read, then compute a hash
		if (m > 0) {
			io.checkHash(buf[:m], piece)
		}
	} else {
		// Single File Mode
		var piece, n int
		var err error
		for offset := int64(0); ; {
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
			// We have a full buf, generate a hash and compare with
			// corresponding pieces part of the torrent file
			io.checkHash(buf, piece)
			// Increment offset by number of bytes read
			offset += int64(n)
			// Increment piece by the length of a SHA-1 hash (20 bytes)
			piece += 20
		}
		// If the final iteration resulted in a partial read, then compute a hash
		if (n > 0) {
			io.checkHash(buf[:n], piece)
		}
	}
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
			name := filepath.Join(file.Path...)
			io.files = append(io.files, openOrCreateFile(name))
		}
	} else {
		// Single File Mode
		io.files = append(io.files, openOrCreateFile(io.metaInfo.Info.Name))
	}
}

func (io *IO) Stop() error {
	io.t.Kill(nil)
	return io.t.Wait()
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
