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
	peerName       string
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
	contRxChans    ControllerPeerChans
	contTxChans    PeerControllerChans
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
	numPieces    int
	peerChans    peerManagerChans
	serverChans  serverPeerChans
	trackerChans trackerPeerChans
	diskIOChans  diskIOPeerChans
	contChans    ControllerPeerManagerChans
	peerContChans PeerControllerChans
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

func NewPeerManager(infoHash []byte, numPieces int, diskIOChans diskIOPeerChans, serverChans serverPeerChans, trackerChans trackerPeerChans) *PeerManager {
	pm := new(PeerManager)
	pm.infoHash = infoHash
	pm.numPieces = numPieces
	pm.diskIOChans = diskIOChans
	pm.serverChans = serverChans
	pm.trackerChans = trackerChans
	pm.peerChans.deadPeer = make(chan string)
	pm.peers = make(map[string]*Peer)
	pm.contChans.newPeer = make(chan PeerComms)
	pm.contChans.deadPeer = make(chan string)
	pm.peerContChans.chokeStatus = make(chan PeerChokeStatus)
	pm.peerContChans.havePiece = make(chan chan HavePiece)
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

func NewPeer(
			peerName string,
			infoHash []byte, 
			initiator bool, 
			numPieces int,
			diskIOChans diskIOPeerChans,
			contRxChans ControllerPeerChans,
			contTxChans PeerControllerChans) *Peer {
	p := &Peer{
			peerName: peerName,
			infoHash: infoHash, 
			peerBitfield: make([]bool, numPieces), 
			ourBitfield: make([]bool, numPieces),
			amChoking: true, 
			amInterested: false, 
			peerChoking: true, 
			peerInterested: false, 
			initiator: initiator, 
			diskIOChans: diskIOChans,
			contRxChans: contRxChans,
			contTxChans: contTxChans}
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

func (p *Peer) sendBitfieldToController(bitfield []bool) {
	haveSlice := make([]HavePiece, 0)

	for pieceNum, hasPiece := range bitfield {
		if hasPiece {
			haveSlice = append(haveSlice, HavePiece{pieceNum, p.peerName})
		}
	}

	p.sendHaveMessagesToController(haveSlice)
}

// Send one or more HavePiece messages to the controller. 
// NOTE: This function will potentially block and should be run
// as a separate goroutine. 
func (p *Peer) sendHaveMessagesToController(pieces []HavePiece) {

	if len(pieces) == 0 {
		log.Fatalf("There must be at least one Have to send to the controller")
	}

	// make an inner channel that will be used to send the individual HavePiece
	// messages to the controller. 
	innerChan := make(chan HavePiece)
	p.contTxChans.havePiece <- innerChan

	for _, havePiece := range pieces {
		innerChan <- havePiece
	}

	// close the inner channel to signal to the controller that we're finished
	// sending HavePiece messages. 
	close(innerChan)
}

/*
func (p *Peer) readBytesFromConn(numBytes int) []byte {
	result := make([]byte, numBytes)
	_, err := io.ReadFull(p.conn, result)
	if err != nil {
		log.Fatalf("Encountered an error when attempting to read %d bytes from %s", numBytes, p.peerName)
	}
	return result
}
*/

// Need to send targetSize because the byte slice will potentially have padding
// bits at the end if the bitfield size is not divisible by 8. 
func convertByteSliceToBoolSlice(targetSize int, original []byte) []bool {
	result := make([]bool, targetSize)
	if ((len(original) * 8) - targetSize) > 7 {
		log.Fatalf("Expected original slice to be roughly 8 times smaller than the target size")
	}

	for i := 0; i < len(original); i++ {
		for j := 0; j < 8; j++ {
			resultIndex := (i * 8) + j
			if resultIndex >= targetSize {
				// We've hit bit padding at the end of the byte slice. 
				break
			} 

			currentByte := original[i]
			currentBit := (currentByte >> uint32(7 - j)) & 1
			result[resultIndex] = currentBit == 1
		}
	} 
	return result
}

func (p *Peer) decodeMessage(payload []byte) {
	if len(payload) == 0 {
		// keepalive
		log.Printf("Received a Keepalive message from %s", p.peerName)
		return
	}

	messageID := payload[0]

	// Remove the messageID 
	payload = payload[1:]

	switch messageID {
	case 0:
		// Choke Message
		if len(payload) != 0 {
			log.Fatalf("Received a Choke from %s with invalid payload size of %d", p.peerName, len(payload))
		} else {
			log.Printf("Received a Choke message from %s", p.peerName)
		}

		if !p.peerChoking {
			// We're changing from being unchoked to choked
			p.peerChoking = true

			// Tell the controller that we've switched from unchoked to choked
			go func() {
				p.contTxChans.chokeStatus <- PeerChokeStatus{peerName: p.peerName, isChoked: true}
			}()
		} else {
			// Ignore choke message because we're already choked. 
		}
		break
	case 1:
		// Unchoke Message
		if len(payload) != 0 {
			log.Fatalf("Received an Unchoke from %s with invalid payload size of %d", p.peerName, len(payload))
		} else {
			log.Printf("Received a Unchoke message from %s", p.peerName)
		}
		if p.peerChoking {
			// We're changing from being choked to unchoked
			p.peerChoking = false

			// Tell the controller that we've switched from choked to unchoked
			go func() {
				p.contTxChans.chokeStatus <- PeerChokeStatus{peerName: p.peerName, isChoked: false}
			}()
		} else {
			// Ignore unchoke message because we're already unchoked. 
		}
		break
	case 2:
		// Interested Message
		if len(payload) != 0 {
			log.Fatalf("Received an Interested from %s with invalid payload size of %d", p.peerName, len(payload))
		} else {
			log.Printf("Received an Interested message from %s", p.peerName)
		}
		p.peerInterested = true

		// FIXME -- Send an unchoke to the peer

		break
	case 3:
		// Not Interested Message
		if len(payload) != 0 {
			log.Fatalf("Received a Not Interested from %s with invalid payload size of %d", p.peerName, len(payload))
		} else {
			log.Printf("Received a Not Interested message from %s", p.peerName)
		}
		p.peerInterested = false

		break
	case 4:
		// Have Message
		if len(payload) != 4 {
			log.Fatalf("Received a Have from %s with invalid payload size of %d", p.peerName, len(payload))
		}

		// Determine the piece number
		pieceNum := int(binary.BigEndian.Uint32(payload))

		log.Printf("Received a Have message for piece %d from %s", pieceNum, p.peerName)

		// Update the local peer bitfield
		p.peerBitfield[pieceNum] = true

		// Send a single HavePiece struct to the controller 
		have := make([]HavePiece, 1)
		have[0] = HavePiece{pieceNum: pieceNum, peerName: p.peerName} 
		go p.sendHaveMessagesToController(have)

		break
	case 5:
		// Bitfield Message
		log.Printf("Received a Bitfield message from %s", p.peerName)

		p.peerBitfield = convertByteSliceToBoolSlice(len(p.peerBitfield), payload)

		// Break the bitfield into a slice of HavePiece structs and send them 
		// to the controller  
		go p.sendBitfieldToController(p.peerBitfield)

		break
	case 6:
		// Request Message
		// IMPLEMENT ME
		pieceNum := 0 // FIXME

		log.Printf("Received a Request message for piece %d from %s", pieceNum, p.peerName)
		break
	case 7:
		// Piece Message
		pieceNum := 0 // FIX
		// IMPLEMENT ME

		log.Printf("Received a Piece message for piece %d from %s", pieceNum, p.peerName)
		break
	case 8:
		// Cancel Message
		// IMPLEMENT ME
		pieceNum := 0 // FIXME

		log.Printf("Received a Cancel message for piece %d from %s", pieceNum, p.peerName)
		break
	case 9:
		// Port Message
		log.Printf("Ignoring a Port message that was received from %s", p.peerName)
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
			// FIXME Are there any cases where we would not read length?
			return
		}
		payload := make([]byte, binary.BigEndian.Uint32(length))
		n, err = io.ReadFull(p.conn, payload)
		if err != nil {
			// FIXME if this is not a keepalive, we should
			// definitely get a payload 
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

func (p *Peer) sendKeepalive() {
	// IMPLEMENT ME
}

func (p *Peer) sendChoke() {
	// IMPLEMENT ME
}

func (p *Peer) sendUnchoke() {
	// IMPLEMENT ME
}

func (p *Peer) sendInterested() {
	// IMPLEMENT ME
}

func (p *Peer) sendNotInterested() {
	// IMPLEMENT ME
}

func (p *Peer) sendHave(pieceNum int) {
	// IMPLEMENT ME
}

func (p *Peer) sendBitfield() {
	// IMPLEMENT ME
}

func (p *Peer) sendRequest(int pieceNum, begin int, length int) {
	// IMPLEMENT ME
}

func (p *Peer) sendBlock(pieceNum int, begin int, block []byte) {
	// IMPLEMENT ME
}

func (p *Peer) sendCancel(pieceNum int, begin int, length int) {
	// IMPLEMENT ME
}

func (p *Peer) Stop() error {
	log.Println("Peer : Stop : Stopping")
	p.t.Kill(nil)
	return p.t.Wait()
}

func (p *Peer) Run() {
	log.Println("Peer : Run : Started")
	defer log.Println("Peer : Run : Completed")

	//initialBitfieldSentToPeer := false

	go p.sendHandshake()
	go p.Reader()

	for {
		select {
		case <-p.keepalive:
		case <-p.read:
			fmt.Println("p.read")
		//case buf := <-p.read:
			//fmt.Println("Read from peer:", buf)

			/*
		case requestPiece := <-p.contRxChans.requestPiece:
		case cancelPiece := <-p.contRxChans.cancelPiece:
		case innerChan := <-p.contRxChans.havePiece:
			// Create a slice of HaveMessage structs from all individual 
			// Have messages received from the controller
			//haveMessages := p.receiveHaveMessagesFromController(innerChan)


			// update our local bitfield based on the Have messages received from the controller. 

			if !initialBitfieldSentToPeer {
				// Send the entire bitfield to the peer

			} else {
				// Send a single have message to the peer 

			}

			// situation #2: The controller sends us  
			*/ 
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
			peerName := fmt.Sprintf("%s:%d", peer.IP.String(), peer.Port)
			_, ok := pm.peers[peerName]
			if !ok {
				// FIXME Code in this block is duplicated below

				// Create the Controller->Peer chans struct
				contTxChans := *NewControllerPeerChans()

				// Construct the Peer object
				pm.peers[peerName] = NewPeer(
					peerName,
					pm.infoHash, 
					true, 
					pm.numPieces,
					pm.diskIOChans,
					contTxChans,
					pm.peerContChans)

				// Give the controller the channels that it will use to 
				// transmit messages to this new peer
				go func() {
					pm.contChans.newPeer <- PeerComms{peerName: peerName, chans: contTxChans}
				}()

				// Have the 'peer' routine create an outbound
				// TCP connection to the remote peer
				go ConnectToPeer(peer, pm.serverChans.conns)
			}
		case conn := <-pm.serverChans.conns:
			_, ok := pm.peers[conn.RemoteAddr().String()]
			if !ok {
				// Create the Controller->Peer chans struct
				contTxChans := *NewControllerPeerChans()

				// Construct the Peer object
				peerName := conn.RemoteAddr().String()
				pm.peers[peerName] = NewPeer(
					peerName,
					pm.infoHash, 
					false, 
					pm.numPieces,
					pm.diskIOChans,
					contTxChans,
					pm.peerContChans)

				// Give the controller the channels that it will use to 
				// transmit messages to this new peer
				go func() {
					pm.contChans.newPeer <- PeerComms{peerName: peerName, chans: contTxChans}
				}()
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
