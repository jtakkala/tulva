// Copyright 2013 Jari Takkala. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"launchpad.net/tomb"
	"log"
	"net"
)

// PeerTuple represents a single IP+port pair of a peer
type PeerTuple struct {
	IP net.IP
	Port uint16
}

type Peer struct {
	peer PeerTuple
}

type PeerManager struct {
	peersCh <-chan PeerTuple
	statsCh chan Stats
	t tomb.Tomb
}

type PeerInfo struct {
	peerId string
	availablePieces []int
	activeRequests map[int]struct{}
	downloadPriority []int
	requestPieceCh chan<- RequestPiece  // Other end is Peer. Used to tell the peer to request a particular piece.  
	cancelPieceCh chan<- CancelPiece  // Other end is Peer. Used to tell the peer to cancel a particular piece. 
}

type SortedPeers []PeerNeediness

func (sp *SortedPeers) Less(i, j int) bool {
	return sp[i].numPiecesNeeded <= sp[j].numPiecesNeeded
}

func (sp *SortedPeers) Swap(i, j int) {
	tmp := sp[i]
	sp[i] = sp[j]
	sp[j] = tmp
}

func (sp *SortedPeers) Len() int {
	len(sp)
}

type PeerNeediness struct {
	peerId string
	numPiecesNeeded int
}

func sortedPeerIds(peers map[string]PeerInfo) []string {
	sorted := make(SortedPeers, 0)
	for peerId, peerInfo := range peers {
		sorted = append(sorted, PeerNeediness{peerId, len(peerInfo.downloadPriority)})
	}
	sort.Sort(sorted)

	sortedPeerIds := make([]string, 0)
	for _, pn := range sorted {
		sortedPeerIds = append(sortedPeerIds, pn.peerId)
	}

	return sortedPeerIds
}

func NewPeerManager(peersCh chan PeerTuple, statsCh chan Stats) *PeerManager {
	pm := new(PeerManager)
	pm.peersCh = peersCh
	pm.statsCh = statsCh
	return pm
}

func NewPeerInfo(quantityOfPieces int) *PeerInfo {
	pi := new(PeerInfo)
	pi.availablePieces = make([]int, quantityOfPieces)
	pi.activeRequests = make(map[int]struct{})
	pi.requestPieceCh = make(chan<- RequestPiece)
	pi.cancelPieceCh = make(chan<- CancelPiece)
	return pi
}

func (pm *PeerManager) Stop() error {
	log.Println("PeerManager : Stop : Stopping")
	pm.t.Kill(nil)
	return pm.t.Wait()
}

func (pm *PeerManager) Run() {
	log.Println("PeerManager : Run : Started")
	defer pm.t.Done()
	defer log.Println("PeerManager : Run : Completed")

	for {
		select {
		case peer := <- pm.peersCh:
			/*
			_, ok := peers[peer]
			if ok {
				// peer already exists
				fmt.Println("Peer already in map")
			} else {
				peers[peer] = "foo"
			}
			*/
			fmt.Println(peer)
		case <- pm.t.Dying():
			return
		}
	}
}
