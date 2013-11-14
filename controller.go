// Copyright 2013 Jari Takkala and Brian Dignan. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"sort"
	"log"
)

type RarityMap struct {
	data map[int][]int
	sortedKeys []int
}

func NewRarityMap() *RarityMap {
	r := new(RarityMap)
	r.data = make(map[int][]int)
	r.sortedKeys = make([]int, 0)
	return r
}

func (r *RarityMap) Len() int {
	return len(sortedKeys)
}

func (r *RarityMap) Less(i, j int) bool {
	return sortedKeys[i] <= sortedKeys[i]
} 

func (r *RarityMap) Swap(i, j int) {
	tmp := sortedKeys[i]
	sortedKeys[i] = sortedKeys[j]
	sortedKeys[j] = tmp
}

func (r *RarityMap) sortKeys() {
	r.sortedKeys = make([]int, 0)
	for key, _ := range r.data {
		r.sortedKeys = append(r.sortedKeys, key)
	}
	sort.Sort(r)
}

func (r *RarityMap) put(rarity int, pieceNum int) {
	if _, ok := r.data[rarity]; !ok {
		r.data[rarity] = make([]int)
	}

	r.data[rarity] = append(r.data[rarity], pieceNum)
}

func (r *RarityMap) getPiecesByRarity() []int {
	pieces := make([]int, 0)

	r.sortKeys()
	for _, rarity := range r.sortedKeys {
		pieceNums := r.data[rarity]
		pieces = append(pieces, pieceNums...)
	}

	return pieces
}



type Controller struct {
	finishedPieces []bool
	pieceHashes []string
	activeRequestsTotals []int 
	peerPieceTotals []int
	peers map[string]PeerInfo
	receivedPieceCh <-chan ReceivedPiece  // Other end is IO 
	newPeerCh <-chan PeerInfo  // Other end is the PeerManager
	cancelPieceCh <-chan CancelPiece  // Other end is Peer. Used when the peer is unable to retrieve a piece
	havePieceCh <-chan HavePiece // Other end is Peer. used When the peer receives a HAVE message
	t tomb.Tomb
}

// Sent from the controller to the peer to request a particular piece
type RequestPiece struct {
	index int
	expectedHash string
}

// Sent by the peer to the controller when it receives a HAVE message
type HavePiece struct {
	index int
	peer string  
	haveMore bool  // set to 1 when the peer is initially breaking a bitfield into individual HAVE messages
}

// Sent from the controller to the peer to cancel an outstanding request
// Also sent from the peer to the controller when it's been choked or 
// when it loses its network connection 
type CancelPiece struct {
	index int
	peer string // needed for when the peer sends a cancel to the controller
}

// Sent from IO to the controller indicating that a piece has been 
// received and written to disk
type ReceivedPiece struct {
	index int
	peer string
}

func NewController(finishedPieces []int, pieceHashes []string) *Controller {
	cont := new(Controller)
	cont.finishedPieces = finishedPieces
	cont.pieceHashes = pieceHashes
	cont.receivedPieceCh = make(chan ReceivedPiece)
	cont.newPeerCh = make(chan string)
	cont.cancelPieceCh = make(CancelPiece)
	cont.havePieceCh = make(HavePiece)
	cont.peers = make(map[string]PeerInfo)
	cont.activeRequestsTotals = make([]int, len(finishedPieces))
	return cont
}

func (cont *Controller) Stop() error {
	log.Println("Controller : Stop : Stopping")
	cont.t.Kill(nil)
	return cont.t.Wait()
}


func (cont *Controller) createRaritySlice() []int {

	rarityMap := NewRarityMap()

	for pieceNum, total := range cont.peerPieceTotals {
		if cont.finishedPieces[pieceNum] {
			continue
		} 

		rarityMap.put(total, pieceNum)
	}

	return rarityMap.getPiecesByRarity()
}

func (cont *Controller) recreateDownloadPriorities(raritySlice []int) {
	for _, peerInfo := range cont.peers {
		downloadPriority := make([]int, 0)
		for _, pieceNum := range raritySlice {
			if peerInfo.availablePieces[pieceNum] == 1 {
				downloadPriority = append(downloadPriority, pieceNum)
			}
		}
		peerInfo.downloadPriority = downloadPriority
	}
}

func (cont *Controller) sendRequests(sortedPeerIds []string) {
	for _, peerId := range sortedPeerIds {
		peerInfo := peers[peerId]
		for peerInfo.activeRequests < maxSimultaneousDownloads && 
	}
}

const (
	maxSimultaneousDownloads = 5
)

func (cont *Controller) Run() {
	log.Println("Controller : Run : Started")
	defer pm.t.Done()
	defer log.Println("Controller : Run : Completed")

	for {
		select {
		case piece := <- cont.receivedPieceCh:
			// Update our bitfield to show that we now have that piece
			cont.finishedPieces[piece.index] = true

			// Create a slice of piece sorted by rarity
			raritySlice := cont.createRaritySlice()

			// Recreate all download priority slices for each PeerInfo struct
			cont.recreateDownloadPriorities(raritySlice)

			// Create a PeerInfo slice, then sort it by downloadPriority length
			sortedPeerIds := sortedPeerIds(peers)

			// Iterate through PeerInfo slice, for each Peer that isn't requesting
			// the max amount, send more requests. 
			sendRequests(sortedPeerIds)


		case piece := <- cont.cancelPieceCh:

		case peer := <- cont.newPeerCh:
			// Throw an error if the peer is duplicate (same IP/Port. should never happen)

			// Create a new PeerInfo struct 

			// Add PeerInfo to the peers map using IP:Port as the key

			// 

		case piece := <- cont.havePieceCh:

		case <- cont.t.Dying():
			return
		}
	}

}