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
	blockRequest chan BlockRequest
}

type DiskIO struct {
	metaInfo  MetaInfo
	files     []*os.File
	peerChans diskIOPeerChans
	contChans ControllerDiskIOChans
	t         tomb.Tomb
}

// checkHash accepts a byte buffer and pieceIndex, computes the SHA-1 hash of
// the buffer and returns true or false if it's correct.
func (diskio *DiskIO) checkHash(buf []byte, pieceIndex int) bool {
	h := sha1.New()
	h.Write(buf)
	return bytes.Equal(h.Sum(nil), []byte(diskio.metaInfo.Info.Pieces[pieceIndex:pieceIndex+h.Size()]))
}

// Verify reads in each file and verifies the SHA-1 checksum of each piece.
// Return the boolean list pieces that are correct.
func (diskio *DiskIO) Verify() []bool {
	log.Println("DiskIO : Verify : Started")
	defer log.Println("DiskIO : Verify : Completed")

	numPieces := len(diskio.metaInfo.Info.Pieces) / 20
	finishedPieces := make([]bool, numPieces)

	buf := make([]byte, diskio.metaInfo.Info.PieceLength)
	var pieceIndex, n int
	var err error

	log.Printf("Verifying downloaded files")
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
				finishedPieces[(pieceIndex / 20)] = diskio.checkHash(buf, pieceIndex)

				// Reset partial read counter
				m = 0
				// Increment piece by the length of a SHA-1 hash (20 bytes)
				pieceIndex += 20
			}
		}
		// If the final iteration resulted in a partial read, then
		// check the hash of it and append the result
		if m > 0 {
			//finishedPieces = append(finishedPieces, diskio.checkHash(buf[:m], pieceIndex))
			finishedPieces[(pieceIndex / 20)] = diskio.checkHash(buf[:m], pieceIndex)
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
			//finishedPieces = append(finishedPieces, diskio.checkHash(buf, pieceIndex))
			finishedPieces[(pieceIndex / 20)] = diskio.checkHash(buf, pieceIndex)

			// Increment piece by the length of a SHA-1 hash (20 bytes)
			pieceIndex += 20
		}
		// If the final iteration resulted in a partial read, then compute a hash
		if n > 0 {
			//finishedPieces = append(finishedPieces, diskio.checkHash(buf[:n], pieceIndex))
			finishedPieces[(pieceIndex / 20)] = diskio.checkHash(buf[:n], pieceIndex)
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
		file, err = os.OpenFile(name, os.O_RDWR, os.ModePerm)
		checkError(err)
	}
	return
}

func NewDiskIO(metaInfo MetaInfo) *DiskIO {
	diskio := new(DiskIO)
	diskio.metaInfo = metaInfo
	diskio.peerChans.writePiece = make(chan Piece)
	diskio.peerChans.blockRequest = make(chan BlockRequest)
	diskio.contChans.receivedPiece = make(chan ReceivedPiece)
	return diskio
}

func (diskio *DiskIO) writePiece(piece Piece) {
	offset := piece.index * diskio.metaInfo.Info.PieceLength

	if len(diskio.metaInfo.Info.Files) == 0 {
		// Single file mode
		n, err := diskio.files[0].WriteAt(piece.data, int64(offset))
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("DiskIO : writePiece: wrote piece %x:%x[%x], file %s\n", piece.index, offset, n, diskio.metaInfo.Info.Name)

	} else {
		// Multiple file mode
		for i := 0; i < len(diskio.metaInfo.Info.Files); i++ {
			if offset > diskio.metaInfo.Info.Files[i].Length {
				offset -= diskio.metaInfo.Info.Files[i].Length
			} else {

				var maxWriteCurrentFile int
				if len(piece.data) < (diskio.metaInfo.Info.Files[i].Length - offset) {
					maxWriteCurrentFile = len(piece.data)
				} else {
					maxWriteCurrentFile = diskio.metaInfo.Info.Files[i].Length - offset
				}

				//log.Printf("TEMP: len(files): %d, i: %d", len(diskio.files), i)
				//log.Printf("TEMP: len(data): %d, maxWriteCurrentFile: %d", len(piece.data), maxWriteCurrentFile)
				//log.Printf("TEMP: offset: %d", offset)
				//log.Printf("TEMP: file.Length: %d", diskio.metaInfo.Info.Files[i].Length)

				n, err := diskio.files[i].WriteAt(piece.data[:maxWriteCurrentFile], int64(offset))
				if err != nil {
					log.Fatal(err)
				}
				log.Printf("DiskIO : writePiece: Wrote piece %x:%x[%x], file %s\n", piece.index, offset, n, diskio.metaInfo.Info.Files[i].Path)

				piece.data = piece.data[maxWriteCurrentFile:]
				offset = 0
				if len(piece.data) == 0 {
					break
				}
			}
		}
	}

}

func (diskio *DiskIO) Init() {
	log.Println("DiskIO : Init : Started")
	defer log.Println("DiskIO : Init : Completed")

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

func (diskio *DiskIO) readBlock(file *os.File, block BlockInfo, offset int64) []byte {
	blockData := make([]byte, block.length)
	n, err := file.ReadAt(blockData, offset)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("DiskIO : readBlock: Read block %x:%x[%x]\n", block.pieceIndex, block.begin, n)
	return blockData
}

func (diskio *DiskIO) requestBlock(block BlockInfo) BlockResponse {
	log.Println("DiskIO : requestBlock : Started")
	defer log.Println("DiskIO : requestBlock : Completed")

	offset := int64(int(block.pieceIndex)*diskio.metaInfo.Info.PieceLength + int(block.begin))
	response := BlockResponse{info: block}
	if len(diskio.metaInfo.Info.Files) == 0 {
		// Single File Mode
		response.data = diskio.readBlock(diskio.files[0], block, offset)
	} else {
		// Multiple File Mode
		for i := 0; i <= len(diskio.metaInfo.Info.Files); i++ {
			if offset > int64(diskio.metaInfo.Info.Files[i].Length) {
				offset -= int64(diskio.metaInfo.Info.Files[i].Length)
			} else {
				// FIXME: Handle offset with multiple files correctly
				response.data = diskio.readBlock(diskio.files[i], block, offset)
			}
		}
	}
	return response
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

	for {
		select {
		case piece := <-diskio.peerChans.writePiece:
			go func() {
				diskio.writePiece(piece)
				diskio.contChans.receivedPiece <- ReceivedPiece{pieceNum: piece.index, peerName: piece.peerName}
			}()
		case blockRequest := <-diskio.peerChans.blockRequest:
			log.Println("Received block request:", blockRequest)
			go func() {
				blockRequest.response <- diskio.requestBlock(blockRequest.request)
			}()
		case <-diskio.t.Dying():
			return
		}
	}
}
