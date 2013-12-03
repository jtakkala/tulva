// Copyright 2013 Jari Takkala and Brian Dignan. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"launchpad.net/tomb"
	"log"
	"net"
	"reflect"
	"sort"
	//"strconv"
	"syscall"
	"time"
)

var Protocol = [19]byte{'B', 'i', 't', 'T', 'o', 'r', 'r', 'e', 'n', 't', ' ', 'p', 'r', 'o', 't', 'o', 'c', 'o', 'l'}

// Message ID values 
const (
	MsgChoke int = iota
	MsgUnchoke
	MsgInterested
	MsgNotInterested
	MsgHave
	MsgBitfield
	MsgRequest
	MsgPiece
	MsgCancel
	MsgPort
)

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
	ourBitfield    []bool
	peerBitfield   []bool
	initiator      bool
	peerID         []byte
	keepalive      <-chan time.Time // channel for sending keepalives
	lastTxKeepalive  time.Time
	lastRxKeepalive  time.Time
	read           chan []byte
	infoHash       []byte
	diskIOChans    diskIOPeerChans
	peerManagerChans peerManagerChans
	stats          PeerStats
	t              tomb.Tomb
}

type PeerStats struct {
	read int
	write int
	errors int
}

type PeerManager struct {
	peers        map[string]*Peer
	infoHash     []byte
	peerChans    peerManagerChans
	serverChans  serverPeerChans
	trackerChans trackerPeerChans
	diskIOChans  diskIOPeerChans
	t            tomb.Tomb
}

type peerManagerChans struct {
	deadPeer chan string
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

type Handshake struct {
	Len      uint8
	Protocol [19]byte
	Reserved [8]uint8
	InfoHash [20]byte
	PeerID   [20]byte
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
	pm.peerChans.deadPeer = make(chan string)
	pm.peers = make(map[string]*Peer)
	return pm
}

func ConnectToPeer(peerTuple PeerTuple, connCh chan *net.TCPConn) {
	raddr := net.TCPAddr{peerTuple.IP, int(peerTuple.Port), ""}
	log.Println("Connecting to", raddr)
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

func NewPeer(infoHash []byte, initiator bool, diskIOChans diskIOPeerChans) *Peer {
	p := &Peer{infoHash: infoHash, amChoking: true, amInterested: false, peerChoking: true, peerInterested: false, initiator: initiator, diskIOChans: diskIOChans}
	p.read = make(chan []byte)
	return p
}

func constructMessage(id int, payload []byte) (msg []byte, err error) {
	msg = make([]byte, 4)

	// Store the length of payload + id in network byte order
	binary.BigEndian.PutUint32(msg, uint32(len(payload) + 1))
	msg = append(msg, byte(id))
	msg = append(msg, payload...)

	return
}

func verifyHandshake(handshake *Handshake, infoHash []byte) error {
	if int(handshake.Len) != len(Protocol) {
		err := fmt.Sprintf("Unexpected length for pstrlen (wanted %d, got %d)", len(Protocol), int(handshake.Len))
		return errors.New(err)
	}
	if !bytes.Equal(handshake.Protocol[:], Protocol[:]) {
		err := fmt.Sprintf("Protocol mismtach: got %s, expected %s", handshake.Protocol, Protocol)
		return errors.New(err)
	}
	if !bytes.Equal(handshake.InfoHash[:], infoHash) {
		err := fmt.Sprintf("Invalid infoHash: got %x, expected %x", handshake.InfoHash, infoHash)
		return errors.New(err)
	}
	return nil
}

func (p *Peer) decodeMessage(payload []byte) {
	if len(payload) == 0 {
		// keepalive
		return
	}

	switch payload[0] {
	case 0:
		// choke
		break
	case 1:
		// unchoke
		break
	case 2:
		// interested
		break
	case 3:
		// not interested
		break
	case 4:
		// have
		break
	case 5:
		// bitfield
		break
	case 6:
		// request
		break
	case 7:
		// piece
		break
	case 8:
		// cancel
		break
	case 9:
		// port
		break
	}
}

func (p *Peer) Reader() {
	log.Println("Peer : Reader : Started")

	var handshake Handshake
	binary.Read(p.conn, binary.BigEndian, &handshake)
	err := verifyHandshake(&handshake, p.infoHash)
	if err != nil {
		p.conn.Close()
		return
	}
	p.peerID = handshake.PeerID[:]

	for {
		length := make([]byte, 4)
		n, err := io.ReadFull(p.conn, length)
		if err != nil {
			return
		}
		payload := make([]byte, binary.BigEndian.Uint32(length))
		n, err = io.ReadFull(p.conn, payload)
		if err != nil {
			return
		}
		log.Printf("Read %d bytes of %x\n", n, payload)
		p.decodeMessage(payload)
		//p.read <- buf
	}
}

func (p *Peer) sendHandshake() {
	log.Println("Peer : sendHandshake : Started")
	defer log.Println("Peer : sendHandshake : Completed")

	handshake := Handshake{
		Len: uint8(len(Protocol)),
		Protocol: Protocol,
		PeerID: PeerID,
	}
	copy(handshake.InfoHash[:], p.infoHash)
	err := binary.Write(p.conn, binary.BigEndian, &handshake)
	if err != nil {
		// TODO: Handle errors
		log.Fatal(err)
	}
	p.stats.write += int(reflect.TypeOf(handshake).Size())
}

func (p *Peer) Stop() error {
	log.Println("Peer : Stop : Stopping")
	p.t.Kill(nil)
	return p.t.Wait()
}

func (p *Peer) Run() {
	log.Println("Peer : Run : Started")
	defer log.Println("Peer : Run : Completed")

	go p.sendHandshake()
	go p.Reader()

	for {
		select {
		case <-p.keepalive:
		case <-p.read:
			fmt.Println("p.read")
		//case buf := <-p.read:
			//fmt.Println("Read from peer:", buf)
		case <-p.t.Dying():
			p.peerManagerChans.deadPeer <- p.conn.RemoteAddr().String()
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
			if !ok {
				// Construct the Peer object
				pm.peers[peerID] = NewPeer(pm.infoHash, true, pm.diskIOChans)
				go ConnectToPeer(peer, pm.serverChans.conns)
			}
		case conn := <-pm.serverChans.conns:
			_, ok := pm.peers[conn.RemoteAddr().String()]
			if !ok {
				// Construct the Peer object
				pm.peers[conn.RemoteAddr().String()] = NewPeer(pm.infoHash, false, pm.diskIOChans)
			}
			// Associate the connection with the peer object and start the peer
			pm.peers[conn.RemoteAddr().String()].conn = conn
			go pm.peers[conn.RemoteAddr().String()].Run()
		case peer := <-pm.peerChans.deadPeer:
			log.Printf("PeerManager : Deleting peer %s\n", peer)
			delete(pm.peers, peer)
		case <-pm.t.Dying():
			for _, peer := range pm.peers {
				peer.Stop()
			}
			return
		}
	}
}
