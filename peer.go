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
	diskIOChans diskIOPeerChans
}

type PeerManager struct {
	serverChans serverPeerChans
	trackerChans trackerPeerChans
	diskIOChans diskIOPeerChans
	peers	map[string]Peer
	t       tomb.Tomb
}

type PeerComms struct {
	peerID string
	requestPiece  chan<- RequestPiece // Other end is Peer. Used to tell the peer to request a particular piece.
	cancelPiece   chan<- CancelPiece  // Other end is Peer. Used to tell the peer to cancel a particular piece.
	havePiece	chan<- chan HavePiece // Other end is Peer. Used to give the peer the initial bitfield and new pieces. 
}

func NewPeerComms(peerID string) (*PeerComms, chan RequestPiece, chan CancelPiece, chan chan HavePiece) {
	pc := new(PeerComms)
	pc.peerID = peerID
	requestPieceCh := make(chan RequestPiece)
	pc.requestPiece = requestPieceCh
	cancelPieceCh := make(chan CancelPiece)
	pc.cancelPiece = cancelPieceCh
	havePieceCh := make(chan chan HavePiece)
	pc.havePiece = havePieceCh
	return pc, requestPieceCh, cancelPieceCh, havePieceCh 
}

type PeerInfo struct {
	peerID          string
	isChoked        bool // The peer is connected but choked. Defaults to TRUE (choked)
	availablePieces []bool
	activeRequests  map[int]struct{}
	qtyPiecesNeeded int                 // The quantity of pieces that this peer has that we haven't yet downloaded.
	requestPieceCh  chan<- RequestPiece // Other end is Peer. Used to tell the peer to request a particular piece.
	cancelPieceCh   chan<- CancelPiece  // Other end is Peer. Used to tell the peer to cancel a particular piece.
	havePieceCh	chan<- chan HavePiece // Other end is Peer. Used to give the peer the initial bitfield and new pieces. 
}

func NewPeerInfo(quantityOfPieces int, peerComms PeerComms) *PeerInfo {
	pi := new(PeerInfo)

	pi.peerID = peerComms.peerID
	pi.requestPieceCh = peerComms.requestPiece
	pi.cancelPieceCh = peerComms.cancelPiece
	pi.havePieceCh = peerComms.havePiece

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

func NewPeerManager(diskIOChans diskIOPeerChans, serverChans serverPeerChans, trackerChans trackerPeerChans) *PeerManager {
	pm := new(PeerManager)
	pm.diskIOChans = diskIOChans
	pm.serverChans = serverChans
	pm.trackerChans = trackerChans
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


func NewPeer(conn *net.TCPConn, diskIOChans diskIOPeerChans) (peer Peer) {
	peer.conn = conn
	peer.diskIOChans = diskIOChans
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
		case peer := <-pm.trackerChans.peers:
			peerID := fmt.Sprintf("%s:%d", peer.IP.String(), peer.Port)
			_, ok := pm.peers[peerID]
			if ok {
				// Peer already exists
				log.Printf("PeerManager : Peer %s already in map\n", peerID)
			} else {
				go ConnectToPeer(peer, pm.serverChans.conns)
			}
		case conn := <-pm.serverChans.conns:
			// Received a new peer connection, instantiate a peer
			pm.peers[conn.RemoteAddr().String()] = NewPeer(conn, pm.diskIOChans)
		case <-pm.t.Dying():
			return
		}
	}
}
