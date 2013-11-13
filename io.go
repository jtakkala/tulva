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
	// Create a buffer of size PieceLength
	pieceLength := io.metaInfo.Info.PieceLength
	buf := make([]byte, pieceLength)
	if len(io.metaInfo.Info.Files) > 0 {
		// Multiple File Mode
		var offset int64
		var j, n int
		var err error
		for i, file := range(io.metaInfo.Info.Files) {
			for j, offset = 0, 0; ; j++ {
				n, err = io.files[i].ReadAt(buf[n:], offset)
				if err != nil {
					if err == sysio.EOF {
						fmt.Printf("%v, %d, %x\n", file, n, buf[:1024])
//						fmt.Printf("file %v, offset %d, byte %d\n", file, offset, n)
						break
					}
					log.Fatal(err)
				}
				fmt.Printf("file %v, offset %d, byte %d\n", file, offset, n)
				fmt.Printf("sha1: %x\n", io.metaInfo.Info.Pieces[j:j + 20])
				offset += int64(n)
				n = 0
			}
		}
	} else {
		// Single File Mode
	}

}

func (io *IO) Stop() error {
	io.t.Kill(nil)
	return io.t.Wait()
}

func (io *IO) Init() {
	if len(io.metaInfo.Info.Files) > 0 {
		// Multiple File Mode
		directory := io.metaInfo.Info.Name
		// Create the directory if it doesn't exist
		if _, err := os.Stat(directory); os.IsNotExist(err) {
			err = os.Mkdir(directory, os.ModeDir | os.ModePerm)
			if err != nil {
				log.Fatal(err)
			}
		}
		err := os.Chdir(directory)
		if err != nil {
			log.Fatal(err)
		}
		for _, file := range io.metaInfo.Info.Files {
			// Create any sub-directories if required
			if len(file.Path) > 1 {
				directory = filepath.Join(file.Path[1:]...)
				if _, err := os.Stat(directory); os.IsNotExist(err) {
					err = os.MkdirAll(directory, os.ModeDir | os.ModePerm)
					if err != nil {
						log.Fatal(err)
					}
				}
			}
			// Create the file if it doesn't exist
			path := filepath.Join(file.Path...)
			var fh *os.File
			if _, err := os.Stat(path); os.IsNotExist(err) {
				// Create the file and return a handle
				fh, err = os.Create(path)
				if err != nil {
					log.Fatal(err)
				}
			} else {
				// Open the file and return a handle
				fh, err = os.Open(path)
				if err != nil {
					log.Fatal(err)
				}
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
