// Copyright 2013 Jari Takkala and Brian Dignan. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

// Piece represents a piece number and data
type Piece struct {
	index    int
	data     []byte
	peerName string
}

type Request struct {
	index  int
	begin  int
	length int
}

// RequestPieceDisk used by peer for requsting pieces from DiskIO
type RequestPieceDisk struct {
	request      Request
	responseChan chan Piece // channel that diskIO should send the response on
}

// Sent from the controller to the peer to request a particular piece
type RequestPiece struct {
	pieceNum     int
	expectedHash []byte
}

// Sent by the peer to the controller when it receives a HAVE message
type HavePiece struct {
	pieceNum int
	peerName string
}

// Sent from the controller to the peer to cancel an outstanding request
type CancelPiece struct {
	pieceNum int
}

// Sent from DiskIO to the controller indicating that a piece has been
// received and written to disk
type ReceivedPiece struct {
	pieceNum int
	peerName string
}
