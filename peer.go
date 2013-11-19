// Copyright 2013 Jari Takkala. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"launchpad.net/tomb"
	"log"
	"net"
	"sort"
)

// PeerTuple represents a single IP+port pair of a peer
type PeerTuple struct {
	IP   net.IP
	Port uint16
}

type Peer struct {
	peer PeerTuple
}

type PeerManager struct {
	peersCh <-chan PeerTuple
	statsCh chan Stats
	connsCh <-chan net.Conn
	peers	map[string]PeerInfo
	t       tomb.Tomb
}

type PeerInfo struct {
	peerId          string
	isActive        bool // The peer is connected and unchoked
	availablePieces []bool
	activeRequests  map[int]struct{}
	qtyPiecesNeeded int                 // The quantity of pieces that this peer has that we haven't yet downloaded.
	requestPieceCh  chan<- RequestPiece // Other end is Peer. Used to tell the peer to request a particular piece.
	cancelPieceCh   chan<- CancelPiece  // Other end is Peer. Used to tell the peer to cancel a particular piece.
	havePieceCh		chan<- HavePiece 	// Other end is Peer. Used to tell the peer that we have a new piece. 
}

type SortedPeers []PeerInfo

func (sp SortedPeers) Less(i, j int) bool {
	return sp[i].qtyPiecesNeeded <= sp[j].qtyPiecesNeeded
}

func (sp SortedPeers) Swap(i, j int) {
	tmp := sp[i]
	sp[i] = sp[j]
	sp[j] = tmp
}

func (sp SortedPeers) Len() int {
	return len(sp)
}

func sortedPeersByQtyPiecesNeeded(peers map[string]PeerInfo) SortedPeers {
	peerInfoSlice := make(SortedPeers, 0)

	for _, peerInfo := range peers {
		peerInfoSlice = append(peerInfoSlice, peerInfo)
	}
	sort.Sort(peerInfoSlice)

	return peerInfoSlice
}

func NewPeerManager(peersCh chan PeerTuple, statsCh chan Stats, connsCh chan net.Conn) *PeerManager {
	pm := new(PeerManager)
	pm.peersCh = peersCh
	pm.statsCh = statsCh
	pm.connsCh = connsCh
	return pm
}

func NewPeerInfo(quantityOfPieces int) *PeerInfo {
	pi := new(PeerInfo)
	pi.availablePieces = make([]bool, quantityOfPieces)
	pi.activeRequests = make(map[int]struct{})

	// FIXME Not finished. Need to hook these channels into the Peer struct
	pi.requestPieceCh = make(chan<- RequestPiece)
	pi.cancelPieceCh = make(chan<- CancelPiece)
	return pi
}

/*
func NewPeer() {
}

func NewPeerConn(net.Conn) {
}

func NewPeerTuple() {
}
*/

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
		case peer := <-pm.peersCh:
			_, ok := peers[peer]
			if ok {
				// peer already exists
				log.Printf("Peer %s:%d already in map\n", peer.IP.String, peer.Port)
			} else {
				peers[peer] = "foo"
			}
			fmt.Println(peer)
		case conn := <-pm.connsCh:
			fmt.Println(conn)
		case <-pm.t.Dying():
			return
		}
	}
}
