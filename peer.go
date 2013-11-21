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
	"strconv"
	"syscall"
	"time"
)

const pstr = "BitTorrent protocol"

// PeerTuple represents a single IP+port pair of a peer
type PeerTuple struct {
	IP   net.IP
	Port uint16
}

type Peer struct {
	conn           *net.TCPConn
	amChoking      bool
	amInterested   bool
	peerChoking    bool
	peerInterested bool
	keepalive      <-chan time.Time
	read           chan []byte
	infoHash       []byte
	diskIOChans    diskIOPeerChans
	t              tomb.Tomb
}

type PeerManager struct {
	peers        map[string]*Peer
	infoHash     []byte
	serverChans  serverPeerChans
	trackerChans trackerPeerChans
	diskIOChans  diskIOPeerChans
	t            tomb.Tomb
}

type PeerComms struct {
	peerName     string
	chans 		 ControllerPeerChans	
}

func NewPeerComms(peerName string, cpc ControllerPeerChans) *PeerComms {
	pc := new(PeerComms)
	pc.peerName = peerName
	pc.chans = cpc
	return pc
}

type PeerInfo struct {
	peerName        string
	isChoked        bool // The peer is connected but choked. Defaults to TRUE (choked)
	availablePieces []bool
	activeRequests  map[int]struct{}
	qtyPiecesNeeded int                   // The quantity of pieces that this peer has that we haven't yet downloaded.
	chans 			ControllerPeerChans
}

func NewPeerInfo(quantityOfPieces int, peerComms PeerComms) *PeerInfo {
	pi := new(PeerInfo)

	pi.peerName = peerComms.peerName
	pi.chans = peerComms.chans

	pi.isChoked = true // By default, a peer starts as being choked by the other side.
	pi.availablePieces = make([]bool, quantityOfPieces)
	pi.activeRequests = make(map[int]struct{})

	return pi
}

// Sent by the peer to controller indicating a 'choke' state change. It either went from unchoked to choked,
// or from choked to unchoked.
type PeerChokeStatus struct {
	peerName   string
	isChoked bool
}

type SortedPeers []*PeerInfo

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

func sortedPeersByQtyPiecesNeeded(peers map[string]*PeerInfo) SortedPeers {
	peerInfoSlice := make(SortedPeers, 0)

	for _, peerInfo := range peers {
		peerInfoSlice = append(peerInfoSlice, peerInfo)
	}
	sort.Sort(peerInfoSlice)

	return peerInfoSlice
}

func NewPeerManager(infoHash []byte, diskIOChans diskIOPeerChans, serverChans serverPeerChans, trackerChans trackerPeerChans) *PeerManager {
	pm := new(PeerManager)
	pm.infoHash = infoHash
	pm.diskIOChans = diskIOChans
	pm.serverChans = serverChans
	pm.trackerChans = trackerChans
	pm.peers = make(map[string]*Peer)
	return pm
}

func ConnectToPeer(peerTuple PeerTuple, connCh chan *net.TCPConn) {
	raddr := net.TCPAddr{peerTuple.IP, int(peerTuple.Port), ""}
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

func NewPeer(conn *net.TCPConn, infoHash []byte, diskIOChans diskIOPeerChans) *Peer {
	p := &Peer{conn: conn, infoHash: infoHash, amChoking: true, amInterested: false, peerChoking: true, peerInterested: false, diskIOChans: diskIOChans}
	p.read = make(chan []byte)
	return p
}

func (p *Peer) Handshake() {
	log.Println("Peer : Run : Started")
	defer log.Println("Peer : Run : Completed")

	reserved := make([]byte, 8)
	buf := make([]byte, 0)
	buf = strconv.AppendInt(buf, int64(len(pstr)), 10)
	buf = append(buf, []byte(pstr)...)
	buf = append(buf, reserved...)
	buf = append(buf, p.infoHash...)
	buf = append(buf, PeerID...)
	n, err := p.conn.Write(buf)
	if err != nil {
		log.Fatal(err)
	}
	// Increment stats here
	fmt.Printf("Wrote %d bytes to peer\n", n)
}

func (p *Peer) Reader() {
	log.Println("Peer : Reader : Started")
	defer p.t.Done()

	buf := make([]byte, 1024)

	for {
		n, err := p.conn.Read(buf)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Read %d bytes\n", n)
		p.read <- buf
	}
}

func (p *Peer) Run() {
	log.Println("Peer : Run : Started")
	defer p.t.Done()
	defer log.Println("Peer : Run : Completed")

	//p.conn.SetDeadline(time.Now().Add(30 * time.Second))
	p.Handshake()
	go p.Reader()

	for {
		select {
		case <-p.keepalive:
		case buf := <-p.read:
			fmt.Println("Read from peer:", buf)
		case <-p.t.Dying():
			return
		}
	}
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
			pm.peers[conn.RemoteAddr().String()] = NewPeer(conn, pm.infoHash, pm.diskIOChans)
			go pm.peers[conn.RemoteAddr().String()].Run()
		case <-pm.t.Dying():
			return
		}
	}
}
