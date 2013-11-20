// Copyright 2013 Jari Takkala and Brian Dignan. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"launchpad.net/tomb"
	"log"
	"net"
	"sort"
	"syscall"
)

// PeerTuple represents a single IP+port pair of a peer
type PeerTuple struct {
	IP   net.IP
	Port uint16
}

type Peer struct {
	conn *net.TCPConn
}

type PeerManager struct {
	peersCh <-chan PeerTuple
	statsCh chan Stats
	connsCh chan *net.TCPConn
	peers	map[string]Peer
	t       tomb.Tomb
}

type PeerComms struct {
	peerID string
	requestPieceCh  chan<- RequestPiece // Other end is Peer. Used to tell the peer to request a particular piece.
	cancelPieceCh   chan<- CancelPiece  // Other end is Peer. Used to tell the peer to cancel a particular piece.
	havePieceCh	chan<- chan<- HavePiece 	// Other end is Peer. Used to give the peer the initial bitfield and new pieces. 
}

func NewPeerComms(peerID string) *PeerComms {
	pc := new(PeerComms)
	pc.peerID = peerID
	pc.requestPieceCh = make(chan RequestPiece)
	pc.cancelPieceCh = make(chan CancelPiece)
	pc.havePieceCh = make(chan chan<- HavePiece)
	return pc
}

type PeerInfo struct {
	peerID          string
	isChoked        bool // The peer is connected but choked. Defaults to TRUE (choked)
	availablePieces []bool
	activeRequests  map[int]struct{}
	qtyPiecesNeeded int                 // The quantity of pieces that this peer has that we haven't yet downloaded.
	requestPieceCh  chan<- RequestPiece // Other end is Peer. Used to tell the peer to request a particular piece.
	cancelPieceCh   chan<- CancelPiece  // Other end is Peer. Used to tell the peer to cancel a particular piece.
	havePieceCh	chan<- chan<- HavePiece 	// Other end is Peer. Used to give the peer the initial bitfield and new pieces. 
}

func NewPeerInfo(quantityOfPieces int, peerComms PeerComms) *PeerInfo {
	pi := new(PeerInfo)

	pi.peerID = peerComms.peerID
	pi.requestPieceCh = peerComms.requestPieceCh
	pi.cancelPieceCh = peerComms.cancelPieceCh
	pi.havePieceCh = peerComms.havePieceCh

	pi.isChoked = false // By default, a peer starts as being choked by the other side. 
	pi.availablePieces = make([]bool, quantityOfPieces)
	pi.activeRequests = make(map[int]struct{})

	return pi
}

// Sent by the peer to controller indicating a 'choke' state change. It either went from unchoked to choked,
// or from choked to unchoked. 
type PeerChokeStatus struct {
	peerID string
	isChoked bool	
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

func NewPeerManager(peersCh chan PeerTuple, statsCh chan Stats, connsCh chan *net.TCPConn) *PeerManager {
	pm := new(PeerManager)
	pm.peersCh = peersCh
	pm.statsCh = statsCh
	pm.connsCh = connsCh
	pm.peers = make(map[string]Peer)
	return pm
}

func ConnectToPeer(peerTuple PeerTuple, connCh chan *net.TCPConn) {
	raddr := net.TCPAddr { peerTuple.IP, int(peerTuple.Port), "" }
	conn, err := net.DialTCP("tcp4", nil, &raddr)
	if err != nil {
		if e, ok := err.(*net.OpError); ok {
			if e.Err == syscall.ECONNREFUSED {
				log.Println("ConnectToPeer : Connection Refused:", raddr)
				return
			}
		}
		log.Fatal(err)
	}
	log.Println("ConnectToPeer : Connected:", raddr)
	connCh <- conn
}


func NewPeer(conn *net.TCPConn) (peer Peer) {
	peer.conn = conn
	return
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
		case peer := <-pm.peersCh:
			peerID := fmt.Sprintf("%s:%d", peer.IP.String(), peer.Port)
			_, ok := pm.peers[peerID]
			if ok {
				// Peer already exists
				log.Printf("PeerManager : Peer %s already in map\n", peerID)
			} else {
				go ConnectToPeer(peer, pm.connsCh)
			}
		case conn := <-pm.connsCh:
			pm.peers[conn.RemoteAddr().String()] = NewPeer(conn)
		case <-pm.t.Dying():
			return
		}
	}
}
