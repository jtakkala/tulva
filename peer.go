// Copyright 2013 Jari Takkala and Brian Dignan. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"launchpad.net/tomb"
	"log"
	"net"
	"reflect"
	"sort"
	"sync"
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
	MsgBlock // This is defined in the spec as piece, but in reality it's a block
	MsgCancel
	MsgPort
)

const (
	downloadBlockSize             = 16384
	maxSimultaneousBlockDownloads = 100
)

// PeerTuple represents a single IP+port pair of a peer
type PeerTuple struct {
	IP   net.IP
	Port uint16
}

type Peer struct {
	conn             *net.TCPConn
	peerName         string
	amChoking        bool
	amInterested     bool
	peerChoking      bool
	peerInterested   bool
	ourBitfield      []bool
	peerBitfield     []bool
	peerID           []byte
	keepalive        <-chan time.Time // channel for sending keepalives
	lastTxMessage    time.Time
	lastRxMessage    time.Time
	infoHash         []byte
	pieceLength      int
	sendChan         chan []byte
	totalLength	     int
	downloads        []*PieceDownload
	diskIOChans      diskIOPeerChans
	blockResponse    chan BlockResponse
	peerManagerChans peerManagerChans
	contRxChans      ControllerPeerChans
	contTxChans      PeerControllerChans
	stats            PeerStats
	statsCh		 chan PeerStats
	t                tomb.Tomb
}

type PieceDownload struct {
	pieceNum             int
	expectedHash         []byte
	data                 []byte
	numBlocksReceived    int
	numOutstandingBlocks int
	numBlocksInPiece     int
	isFinished			 bool
}

func (piece *PieceDownload) remainingRequestsToSend() int {
	return piece.numBlocksInPiece - (piece.numBlocksReceived + piece.numOutstandingBlocks)
}

func (p *Peer) newPieceDownload(requestPiece RequestPiece) *PieceDownload {
	pd := new(PieceDownload)
	pd.pieceNum = requestPiece.pieceNum
	pd.expectedHash = requestPiece.expectedHash
	pd.data = make([]byte, p.expectedLengthForPiece(requestPiece.pieceNum))
	pd.numBlocksInPiece = p.expectedNumBlocksForPiece(requestPiece.pieceNum)
	pd.isFinished = false
	return pd
}

type PeerStats struct {
	mu     sync.Mutex
	read   int
	write  int
	errors int
}

func (ps *PeerStats) addRead(value int) {
	ps.mu.Lock()
	ps.read += value
	ps.mu.Unlock()
}

func (ps *PeerStats) addWrite(value int) {
	ps.mu.Lock()
	ps.write += value
	ps.mu.Unlock()
}

func (ps *PeerStats) addError(value int) {
	ps.mu.Lock()
	ps.errors += value
	ps.mu.Unlock()
}

type PeerManager struct {
	peers         map[string]*Peer
	infoHash      []byte
	numPieces     int
	pieceLength   int
	totalLength   int
	seeding		  bool
	peerChans     peerManagerChans
	serverChans   serverPeerChans
	trackerChans  trackerPeerChans
	diskIOChans   diskIOPeerChans
	contChans     ControllerPeerManagerChans
	peerContChans PeerControllerChans
	statsCh	      chan PeerStats
	t             tomb.Tomb
}

type peerManagerChans struct {
	deadPeer chan string
}

type PeerComms struct {
	peerName string
	chans    ControllerPeerChans
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
	qtyPiecesNeeded int // The quantity of pieces that this peer has that we haven't yet downloaded.
	chans           ControllerPeerChans
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
	peerName string
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

func NewPeerManager(infoHash []byte, numPieces int, pieceLength int, totalLength int, diskIOChans diskIOPeerChans, serverChans serverPeerChans, statsCh chan PeerStats, trackerChans trackerPeerChans) *PeerManager {
	pm := new(PeerManager)
	pm.infoHash = infoHash
	pm.numPieces = numPieces
	pm.pieceLength = pieceLength
	pm.totalLength = totalLength
	pm.diskIOChans = diskIOChans
	pm.serverChans = serverChans
	pm.statsCh = statsCh
	pm.trackerChans = trackerChans
	pm.seeding = false
	pm.peerChans.deadPeer = make(chan string)
	pm.peers = make(map[string]*Peer)
	pm.contChans.newPeer = make(chan PeerComms)
	pm.contChans.deadPeer = make(chan string)
	pm.contChans.seeding = make(chan bool)
	pm.peerContChans.chokeStatus = make(chan PeerChokeStatus)
	pm.peerContChans.havePiece = make(chan chan HavePiece)
	return pm
}

func connectToPeer(peerTuple PeerTuple, connCh chan *net.TCPConn) {
	raddr := net.TCPAddr{peerTuple.IP, int(peerTuple.Port), ""}
	log.Println("Peer : Connecting to", raddr)
	conn, err := net.DialTCP("tcp4", nil, &raddr)
	if err != nil {
		log.Println("Peer : connectToPeer :", err)
		return
	}
	log.Println("Peer : connectToPeer : Connected to", raddr)
	connCh <- conn
}

func NewPeer(
	peerName string,
	infoHash []byte,
	numPieces int,
	pieceLength int,
	totalLength int,
	diskIOChans diskIOPeerChans,
	contRxChans ControllerPeerChans,
	contTxChans PeerControllerChans,
	peerManagerChans peerManagerChans,
	statsCh chan PeerStats) *Peer {
	p := &Peer{
		peerName:       peerName,
		infoHash:       infoHash,
		pieceLength:    pieceLength,
		totalLength:    totalLength,
		peerBitfield:   make([]bool, numPieces),
		ourBitfield:    make([]bool, numPieces),
		keepalive:      make(chan time.Time),
		lastTxMessage:  time.Now(),
		lastRxMessage:  time.Now(),
		amChoking:      true,
		amInterested:   false,
		peerChoking:    true,
		peerInterested: false,
		sendChan:       make(chan []byte),
		diskIOChans:    diskIOChans,
		blockResponse:  make(chan BlockResponse),
		contRxChans:    contRxChans,
		contTxChans:    contTxChans,
		peerManagerChans: peerManagerChans,
		statsCh:	statsCh,
		downloads: 		make([]*PieceDownload, 0)}
	return p
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

	if len(haveSlice) > 0 {
		p.sendHaveMessagesToController(haveSlice)
	}

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
			currentBit := (currentByte >> uint32(7-j)) & 1
			result[resultIndex] = currentBit == 1
		}
	}
	return result
}

func convertBoolSliceToByteSlice(bitfield []bool) []byte {
	sliceLen := len(bitfield) / 8 // 8 bits per byte
	if len(bitfield)%8 != 0 {
		// add one more byte because the bitfield doesn't fit evenly into a byte slice
		sliceLen += 1
	}
	result := make([]byte, sliceLen)

	for i := 0; i < len(result); i++ {
		orValue := byte(128)
		for j := 0; j < 8; j++ {
			bitfieldIndex := ((i * 8) + j)
			if bitfieldIndex >= len(bitfield) {
				// we've hit the end of the bitfield
				break
			}

			if bitfield[bitfieldIndex] {
				// We have this piece. Set the binary bit
				result[i] = result[i] | orValue
			}

			orValue = orValue >> 1
		}
	}
	return result
}

func checkHash(block []byte, expectedHash []byte) bool {
	h := sha1.New()
	h.Write(block)
	return bytes.Equal(h.Sum(nil), expectedHash)
}

func (p *Peer) sendFinishedPieceToDiskIO(pieceNum int, data []byte) {
	piece := new(Piece)
	piece.index = pieceNum
	piece.data = data
	piece.peerName = p.peerName
	p.diskIOChans.writePiece <- *piece
}

func (p *Peer) weShouldBeInterested() bool {
	// Loop through our bitfield to check if there are any pieces we don't have
	// that the peer does have
	for pieceNum, hasPiece := range p.ourBitfield {
		if !hasPiece && p.peerBitfield[pieceNum] {
			// We don't have this piece, but the peer does
			return true
		}
	}
	// We didn't find any pieces that the peer has but we don't have.
	return false
}

func (p *Peer) decodeMessage(payload []byte) {
	if len(payload) == 0 {
		// keepalive
		log.Printf("Received a Keepalive message from %s", p.peerName)
		return
	}

	messageID := int(payload[0])
	// Remove the messageID
	payload = payload[1:]

	switch messageID {
	case MsgChoke:
		if len(payload) != 0 {
			log.Fatalf("Received a Choke from %s with invalid payload size of %d", p.peerName, len(payload))
		} else {
			log.Printf("Received a Choke message from %s", p.peerName)
		}
		if !p.peerChoking {
			// We're changing from being unchoked to choked
			p.peerChoking = true
			// Clear out any unfinished work
			for _, download := range p.downloads {
				download.isFinished = true
				download.numBlocksReceived = 0
				download.numOutstandingBlocks = 0
			} 
			// Tell the controller that we've switched from unchoked to choked
			go func() {
				p.contTxChans.chokeStatus <- PeerChokeStatus{peerName: p.peerName, isChoked: true}
			}()
		} else {
			// Ignore choke message because we're already choked.
		}
	case MsgUnchoke:
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
	case MsgInterested:
		if len(payload) != 0 {
			log.Fatalf("Received an Interested from %s with invalid payload size of %d", p.peerName, len(payload))
		} else {
			log.Printf("\033[31mReceived an Interested message from %s\033[0m", p.peerName)
		}
		p.peerInterested = true
		p.sendUnchoke()
	case MsgNotInterested:
		// Not Interested Message
		if len(payload) != 0 {
			log.Fatalf("Received a Not Interested from %s with invalid payload size of %d", p.peerName, len(payload))
		} else {
			log.Printf("Received a Not Interested message from %s", p.peerName)
		}
		p.peerInterested = false
		p.sendChoke()
	case MsgHave:
		if len(payload) != 4 {
			log.Fatalf("Received a Have from %s with invalid payload size of %d", p.peerName, len(payload))
		}

		// Determine the piece number
		pieceNum := int(binary.BigEndian.Uint32(payload))
		log.Printf("Received a Have message for piece %x from %s", pieceNum, p.peerName)

		// Update the local peer bitfield
		p.peerBitfield[pieceNum] = true

		// Send a single HavePiece struct to the controller
		have := make([]HavePiece, 1)
		have[0] = HavePiece{pieceNum: pieceNum, peerName: p.peerName}
		go p.sendHaveMessagesToController(have)

		if !p.amInterested {
			// Determine if we should switch from not interested to interested
			if p.weShouldBeInterested() {
				p.sendInterested()
			}
		}
	case MsgBitfield:
		log.Printf("Received a Bitfield message from %s with payload %x", p.peerName, payload)

		p.peerBitfield = convertByteSliceToBoolSlice(len(p.peerBitfield), payload)

		// Break the bitfield into a slice of HavePiece structs and send them
		// to the controller
		go p.sendBitfieldToController(p.peerBitfield)

		if !p.amInterested {
			// Determine if we should switch from not interested to interested
			if p.weShouldBeInterested() {
				p.sendInterested()
			}
		}
	case MsgRequest:
		var blockInfo BlockInfo
		blockInfo.pieceIndex = binary.BigEndian.Uint32(payload[0:4])
		blockInfo.begin = binary.BigEndian.Uint32(payload[4:8])
		blockInfo.length = binary.BigEndian.Uint32(payload[8:12])
		blockRequest := BlockRequest{request: blockInfo, response: p.blockResponse}
		p.diskIOChans.blockRequest <- blockRequest
		log.Printf("\033[31mReceived a Request message for %v from %s\033[0m", blockInfo, p.peerName)
	case MsgBlock:
		if len(payload) < 9 {
			log.Fatalf("Received a Block (Piece) message from %s with invalid payload size of %d.", p.peerName, len(payload))
		}

		pieceNum := int(binary.BigEndian.Uint32(payload[0:4]))
		begin := int(binary.BigEndian.Uint32(payload[4:8]))
		blockData := payload[8:]

		blockNum := begin / downloadBlockSize

		if !p.haveCurrentDownloads() {
			log.Printf("WARNING: Received piece %x:%x from %s but there aren't any current downloads", pieceNum, begin, p.peerName)
			return
		} else if begin%downloadBlockSize != 0 {
			log.Fatalf("Received a Block (Piece) message from %s with an invalid begin value of %x", p.peerName, begin)
		} else if len(blockData) != p.expectedLengthForBlock(pieceNum, blockNum) {
			log.Fatalf("Received a Block (Piece) message from %s with an invalid block size of %x. Expected %x", p.peerName, len(blockData), p.expectedLengthForBlock(pieceNum, blockNum))
		} else {
			//log.Printf("Received a Block (Piece) message from %s for piece %x:%x[%x]", p.peerName, pieceNum, begin, len(blockData))
		}

		piece := p.getPieceDownload(pieceNum)
		if piece == nil {
			log.Printf("WARNING: The block from %s for piece %x doesn't match the current or next download pieces", p.peerName, pieceNum)
			return
		}

		// The block (piece) message is valid. Write the contents to the buffer.
		copy(piece.data[begin:], blockData)

		piece.numBlocksReceived += 1
		piece.numOutstandingBlocks -= 1

		if piece.numBlocksReceived == piece.numBlocksInPiece {
			log.Printf("Finished downloading all blocks for piece %x from %s", pieceNum, p.peerName)

			// SHA1 check the entire piece
			if !checkHash(piece.data, piece.expectedHash) {
				// The piece received from this peer didn't pass the checksum.
				log.Printf("ERROR: Checksum for piece %x received from %s did NOT match what's expected. Disconnecting.", pieceNum, p.peerName)
				p.Stop()
				return
			}

			log.Printf("Checksum for piece %x received from %s matches what's expected", pieceNum, p.peerName)
			
			piece.isFinished = true
			p.moveFinishedPieceDownloadsToEnd()

			p.sendFinishedPieceToDiskIO(pieceNum, piece.data)

			// if nextDownload was previosly nil, then currentDownload will now be nil, because we
			// copied the reference from nextDownload to currentDownload.
			if !p.haveCurrentDownloads() {
				log.Printf("Peer %s has no active or next pieces. It will be idle until given more pieces to download", p.peerName)
				return		
			}
		}

		p.sendOneOrMoreRequests()
	case MsgCancel:
		// TODO: Implement cancel handling
		pieceIndex := binary.BigEndian.Uint32(payload[0:4])
		begin := binary.BigEndian.Uint32(payload[4:8])
		length := binary.BigEndian.Uint32(payload[8:12])
		log.Printf("Received a Cancel message for piece %x:%x[%x] from %s", pieceIndex, begin, length, p.peerName)
	case MsgPort:
		log.Printf("Ignoring a Port message that was received from %s", p.peerName)
	}
}

func (p *Peer) haveCurrentDownloads() bool {
	for _, download := range p.downloads {
		if !download.isFinished {
			return true
		}
	}
	return false
}

func (p *Peer) getPieceDownload(pieceNum int) *PieceDownload {
	for _, download := range p.downloads {
		if !download.isFinished && download.pieceNum == pieceNum {
			return download
		}
	}
	return nil
}

func (p *Peer) moveFinishedPieceDownloadsToEnd() {
	replacement := make([]*PieceDownload, 0)
	for _, download := range p.downloads {
		if !download.isFinished {
			replacement = append(replacement, download)
		}
	}
	for _, download := range p.downloads {
		if download.isFinished {
			replacement = append(replacement, download)
		}
	}
	p.downloads = replacement
}

func (p *Peer) reader() {
	log.Printf("Peer (%s) : reader : Started", p.peerName)
	defer log.Printf("Peer (%s) : reader : Completed", p.peerName)

	var handshake Handshake
	err := binary.Read(p.conn, binary.BigEndian, &handshake)
	if err != nil {
		log.Printf("Peer (%s) error in reader() doing binary.Read(): %s", p.peerName, err)
		p.Stop()
		return
	}

	p.lastRxMessage = time.Now()
	p.stats.addRead(int(reflect.TypeOf(handshake).Size()))

	err = verifyHandshake(&handshake, p.infoHash)
	if err != nil {
		log.Printf("Peer (%s) verifyandshake returned: %s", p.peerName, err)
		p.Stop()
		return
	}
	p.peerID = handshake.PeerID[:]

	for {
		length := make([]byte, 4)
		n, err := io.ReadFull(p.conn, length)
		if err != nil {
			log.Printf("Peer (%s) error in reader() doing io.ReadFull(): %s", p.peerName, err)
			p.Stop()
			return
		}
		p.lastRxMessage = time.Now()
		p.stats.addRead(n)

		payload := make([]byte, binary.BigEndian.Uint32(length))
		n, err = io.ReadFull(p.conn, payload)
		if err != nil {
			log.Printf("Peer (%s) error in reader() doing io.ReadFull(): %s", p.peerName, err)
			p.Stop()
			return
		}
		p.lastRxMessage = time.Now()
		p.stats.addRead(n)

		//log.Printf("Peer (%s) read %d bytes", p.peerName, n + 4)
		go p.decodeMessage(payload)
	}
}

func (p *Peer) sendHandshake() {
	log.Printf("Peer : sendHandshake : Sending handshake to %s", p.peerName)

	handshake := Handshake{
		Len:      uint8(len(Protocol)),
		Protocol: Protocol,
		PeerID:   PeerID,
	}
	copy(handshake.InfoHash[:], p.infoHash)

	err := binary.Write(p.conn, binary.BigEndian, &handshake)
	if err != nil {
		log.Printf("Peer (%s) error in sendHandshake() doing binary.Write(): %s", p.peerName, err)
		p.Stop()
		return
	}

	p.lastTxMessage = time.Now()
	p.stats.addWrite(int(reflect.TypeOf(&handshake).Size()))
}

func (p *Peer) sendKeepalive() {
	log.Printf("Peer : sendKeepalive : Sending keepalive to %s", p.peerName)

	message := make([]byte, 4)

	// Untested
	err := binary.Write(p.conn, binary.BigEndian, &message)
	if err != nil {
		log.Printf("Peer (%s) error in sendKeepalive() doing binary.Write(): %s", p.peerName, err)
		p.Stop()
		return
	}

	p.lastTxMessage = time.Now()
	p.stats.addWrite(4)
}

func (p *Peer) writer() {
	log.Println("Peer : writer : Started:", p.peerName)
	defer log.Println("Peer : writer : Completed:", p.peerName)

	for {
		select {
		case message := <-p.sendChan:
			n, err := p.conn.Write(message)
			if err != nil {
				log.Printf("Peer (%s) error in writer() doing Write(): %s", p.peerName, err)
				p.Stop()
				return
			}
			p.lastTxMessage = time.Now()
			p.stats.addWrite(n)
			//log.Printf("Peer (%s) wrote %d bytes", p.peerName, n)
		}
	}
}

// Sends any message besides a handshake or a keepalive, both of which
// don't have a beginning LEN-ID structure. The length is automatically calculated.
func (p *Peer) constructMessage(ID int, payload interface{}) {
	// Write the payload to a slice of bytes so the length can be computed
	payloadBuffer := new(bytes.Buffer)
	err := binary.Write(payloadBuffer, binary.BigEndian, payload)
	if err != nil {
		log.Fatal(err)
	}
	payloadBytes := payloadBuffer.Bytes()

	messageBuffer := new(bytes.Buffer)

	// Write a 4-byte length field to the buffer
	lengthField := uint32(len(payloadBytes) + 1) // plus 1 to account for the ID

	err = binary.Write(messageBuffer, binary.BigEndian, lengthField)
	if err != nil {
		log.Fatal(err)
	}

	// Write a 1-byte ID field to the buffer
	err = binary.Write(messageBuffer, binary.BigEndian, uint8(ID))
	if err != nil {
		log.Fatal(err)
	}

	// Write the variable length payload to the buffer (potentially 0 bytes)
	err = binary.Write(messageBuffer, binary.BigEndian, payloadBytes)
	if err != nil {
		log.Fatal(err)
	}

	// Send the message to the peer
	p.sendChan <- messageBuffer.Bytes()
}

func (p *Peer) sendChoke() {
	log.Printf("Peer : sendChoke : Sending choke to %s", p.peerName)
	go p.constructMessage(MsgChoke, make([]byte, 0))
	p.amChoking = true
}

func (p *Peer) sendUnchoke() {
	log.Printf("Peer : sendUnchoke : Sending unchoke to %s", p.peerName)
	go p.constructMessage(MsgUnchoke, make([]byte, 0))
	p.amChoking = false
}

func (p *Peer) sendInterested() {
	log.Printf("Peer : sendInterested : Sending interested to %s", p.peerName)
	go p.constructMessage(MsgInterested, make([]byte, 0))
	p.amInterested = true
}

func (p *Peer) sendNotInterested() {
	log.Printf("Peer : sendNotInterested : Sending not-interested to %s", p.peerName)
	go p.constructMessage(MsgNotInterested, make([]byte, 0))
	p.amInterested = false
}

func (p *Peer) sendHave(pieceNum int) {
	log.Printf("Peer : sendHave : Sending have to %s for piece %x", p.peerName, pieceNum)
	payloadBuffer := new(bytes.Buffer)
	err := binary.Write(payloadBuffer, binary.BigEndian, uint32(pieceNum))
	if err != nil {
		log.Fatal(err)
	}
	p.constructMessage(MsgHave, payloadBuffer.Bytes())
}

func (p *Peer) sendBitfield() {
	compacted := convertBoolSliceToByteSlice(p.ourBitfield)
	log.Printf("Peer : sendBitfield : Sending bitfield to %s with payload %x", p.peerName, compacted)
	p.constructMessage(MsgBitfield, compacted)
}

func (p *Peer) sendRequest(pieceNum int, begin int, length int) {
	//log.Printf("Peer : sendRequest : Sending Request to %s for piece %x:%x[%x]", p.peerName, pieceNum, begin, length)
	buffer := new(bytes.Buffer)

	ints := []uint32{uint32(pieceNum), uint32(begin), uint32(length)}

	err := binary.Write(buffer, binary.BigEndian, ints)
	if err != nil {
		log.Fatal(err)
	}

	p.constructMessage(MsgRequest, buffer.Bytes())
}

func (p *Peer) expectedLengthForBlock(pieceNum int, blockNum int) int {
	if pieceNum == (len(p.ourBitfield) - 1) {
		// This is the last piece. Check to see if it's the last block
		lastPieceLength := p.expectedLengthForPiece(pieceNum)

		if ((blockNum * downloadBlockSize) + downloadBlockSize) > lastPieceLength {
			// This is the last block of the last piece
			return (p.totalLength % p.pieceLength) % downloadBlockSize
		} else {
			// This is the last piece, but not the last block.  
			return downloadBlockSize
		}
	} else {
		// This is not the last piece. 
		return downloadBlockSize
	}
}

func (p *Peer) expectedLengthForPiece(pieceNum int) int {
	if pieceNum == (len(p.ourBitfield) - 1) {
		// This is the last piece
		pieceLength := p.totalLength % p.pieceLength
		if pieceLength == 0 {
			// The last piece of this torrent is the same size as every other piece
			return p.pieceLength
		} else {
			// The last piece is smaller than the other pieces. 
			return pieceLength
		}
	} else {
		// this is not the last piece
		return p.pieceLength
	}
}

func (p *Peer) expectedNumBlocksForPiece(pieceNum int) int {
	if pieceNum == (len(p.ourBitfield) - 1) {
		// This is the last piece
		lengthOfLastPiece := p.expectedLengthForPiece(pieceNum)
		if lengthOfLastPiece == p.pieceLength {
			return p.pieceLength / downloadBlockSize 
		} else {
			return (lengthOfLastPiece / downloadBlockSize) + 1
		}
	} else {
		// this is not the last piece
		return p.pieceLength / downloadBlockSize
	}
}

func (p *Peer) sendRequestByBlockNum(pieceNum int, blockNum int) {
	begin := downloadBlockSize * blockNum
	length := p.expectedLengthForBlock(pieceNum, blockNum)
	p.sendRequest(pieceNum, begin, length)
}

func (p *Peer) sendOneOrMoreRequests() {
	for {

		numOutstandingBlocks := p.totalOutstandingBlocks()

		if numOutstandingBlocks > maxSimultaneousBlockDownloads {
			log.Fatalf("Peer : sendOneOrMoreRequests : State Error: Somehow there are %d outstanding blocks, which is more than %d", numOutstandingBlocks, maxSimultaneousBlockDownloads)
		} else if numOutstandingBlocks == maxSimultaneousBlockDownloads {
			// We're maxxed out on the number of outstanding blocks to this peer.
			// Wait until blocks are received before sending more requests.
			return
		} else {
			// We need to send more requests now. First check if we need to send
			// any more requests in the currentDownload piece, which has higher
			// priority than nextDownload
			
			for _, piece := range p.downloads {
				if !piece.isFinished && piece.remainingRequestsToSend() > 0 {
					blockNum := piece.numBlocksReceived + piece.numOutstandingBlocks
					go p.sendRequestByBlockNum(piece.pieceNum, blockNum)
					piece.numOutstandingBlocks += 1

					// Make a recursive call to attempt to send more requests
					p.sendOneOrMoreRequests()
					return
				}
			}

			// We cycled through every piece and still didn't hit maxSimultaneousBlockDownloads
			return
		}
	}
}

func (p *Peer) totalOutstandingBlocks() int {
	outstandingBlocks := 0
	for _, download := range p.downloads {
		if !download.isFinished {
			outstandingBlocks += download.numOutstandingBlocks
		}
	}
	return outstandingBlocks
}

func (p *Peer) sendBlock(pieceNum uint32, begin uint32, block []byte) {
	buffer := new(bytes.Buffer)

	ints := []uint32{pieceNum, begin}

	err := binary.Write(buffer, binary.BigEndian, ints)
	if err != nil {
		log.Fatal(err)
	}

	err = binary.Write(buffer, binary.BigEndian, block)
	if err != nil {
		log.Fatal(err)
	}

	p.constructMessage(MsgBlock, buffer.Bytes())
}

func (p *Peer) sendCancel(pieceNum int, begin int, length int) {
	buffer := new(bytes.Buffer)

	ints := []uint32{uint32(pieceNum), uint32(begin), uint32(length)}

	err := binary.Write(buffer, binary.BigEndian, ints)
	if err != nil {
		log.Fatal(err)
	}

	p.constructMessage(MsgCancel, buffer.Bytes())
}

func (p *Peer) receiveHavesFromController(innerChan chan HavePiece) []HavePiece {
	// Create a slice of HaveMessage structs from all individual
	// Have messages received from the controller
	havePieces := make([]HavePiece, 0)
	for havePiece := range innerChan {
		havePieces = append(havePieces, havePiece)
	}

	log.Printf("Peer : Run : %s received %d HavePiece messages from Controller", p.peerName, len(havePieces))

	return havePieces

}

func (p *Peer) updateOurBitfield(havePieces []HavePiece) {
	// update our local bitfield based on the Have messages received from the controller.
	for _, havePiece := range havePieces {
		p.ourBitfield[havePiece.pieceNum] = true
	}
}

func (p *Peer) Stop() error {
	log.Println("Peer : Stop : Stopping:", p.peerName)
	p.t.Kill(nil)
	return p.t.Wait()
}

func (p *Peer) processCancelFromController(cancelPiece CancelPiece) {
	if !p.haveCurrentDownloads() {
		log.Printf("Peer : Run : WARNING - Controller told %s to cancel pieceNum %d, but this peer isn't working on anything", p.peerName, cancelPiece.pieceNum)
		return 
	}

	piece := p.getPieceDownload(cancelPiece.pieceNum)

	if piece == nil {
		log.Printf("Peer : Run : WARNING - Controller told %s to cancel pieceNum %d, but this peer isn't working on that piece", p.peerName, cancelPiece.pieceNum)
	} else {
		log.Printf("Peer : Run : Controller told %s to cancel pieceNum %d.", p.peerName, cancelPiece.pieceNum)
		piece.isFinished = true
		p.moveFinishedPieceDownloadsToEnd()
	}
}

func (p *Peer) Run() {
	log.Println("Peer : Run : Started:", p.peerName)
	defer log.Println("Peer : Run : Completed:", p.peerName)

	p.keepalive = time.Tick(time.Second * 1)

	p.sendHandshake()

	// Block on this because it simplifies the logic for
	// sending the initial bitfield to the peer
	havePieces := p.receiveHavesFromController(<-p.contRxChans.havePiece)
	p.updateOurBitfield(havePieces)
	go p.sendBitfield()
	go p.writer()
	go p.reader()

	log.Printf("Peer : Run : %s finished initializing reader and writer", p.peerName)

	for {
		select {
		case t := <-p.keepalive:
			if p.lastTxMessage.Add(time.Second * 30).Before(t) {
				log.Println("No txMessage for 30 seconds", p.peerName, p.lastTxMessage.Unix(), t.Unix())
				p.sendKeepalive()
			}
			if p.lastRxMessage.Add(time.Second * 120).Before(t) {
				log.Println("No RxMessage for 120 seconds", p.peerName, p.lastRxMessage.Unix(), t.Unix())
				p.Stop()
			}
			go func() {
				p.statsCh <- p.stats
			}()
		case blockResponse := <-p.blockResponse:
			go p.sendBlock(blockResponse.info.pieceIndex, blockResponse.info.begin, blockResponse.data)
		case requestPiece := <-p.contRxChans.requestPiece:
			log.Printf("Peer : Run : Controller told %s to get piece %x", p.peerName, requestPiece.pieceNum)

			p.initializePieceDownload(requestPiece)

			// Send the first set of block requests all at once. When we get response (piece) messages,
			// we'll then determine if more need to be set.
			p.sendOneOrMoreRequests()

		case cancelPiece := <-p.contRxChans.cancelPiece:
			p.processCancelFromController(cancelPiece)

		case innerChan := <-p.contRxChans.havePiece:
			log.Printf("Peer : %s received a HavePiece innerChan from controller.", p.peerName)

			havePieces := p.receiveHavesFromController(innerChan)
			p.updateOurBitfield(havePieces)

			// Send have messages to the peer. Since this is not the initial bitfield
			// from the controller, there should only be one.
			for _, havePiece := range havePieces {
				go p.sendHave(havePiece.pieceNum)
			}

			if p.amInterested && !p.weShouldBeInterested() {
				p.sendNotInterested()
			}

		case <-p.t.Dying():
			p.conn.Close()
			p.peerManagerChans.deadPeer <- p.peerName
			return
		}
	}
}

func (p *Peer) initializePieceDownload(requestPiece RequestPiece) {
	var piece *PieceDownload
	for _, download := range p.downloads {
		if download.isFinished {
			piece = download
			break
		}
	}

	if piece == nil {
		piece = p.newPieceDownload(requestPiece)
		p.downloads = append(p.downloads, piece)
	}

	piece.isFinished = false
	piece.pieceNum = requestPiece.pieceNum
	piece.expectedHash = requestPiece.expectedHash
	if len(piece.data) != p.expectedLengthForPiece(requestPiece.pieceNum) {
		log.Printf("REMADE PieceDownload for %s. Previous length: %d. New length: %d", p.peerName, len(piece.data), p.expectedLengthForPiece(requestPiece.pieceNum))
		piece.data = make([]byte, p.expectedLengthForPiece(requestPiece.pieceNum))
	}
	piece.numBlocksInPiece = p.expectedNumBlocksForPiece(requestPiece.pieceNum)
	piece.numBlocksReceived = 0
	piece.numOutstandingBlocks = 0


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
			select {
			case finished := <- pm.contChans.seeding:
				if !finished {
					log.Fatalf("PeerManager : Unexpectedly received a false over the seeding channel.")
				} else {
					pm.seeding = true
				}
			default:
			}
			if !pm.seeding {
				peerName := fmt.Sprintf("%s:%d", peer.IP.String(), peer.Port)
				_, ok := pm.peers[peerName]
				if !ok {
					// Have the 'peer' routine create an outbound
					// TCP connection to the remote peer
					go connectToPeer(peer, pm.serverChans.conns)
				}
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
					pm.numPieces,
					pm.pieceLength,
					pm.totalLength,
					pm.diskIOChans,
					contTxChans,
					pm.peerContChans,
					pm.peerChans,
					pm.statsCh)

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
			// Tell the controller that this peer is dead
			go func() {
				pm.contChans.deadPeer <- peer
			}()
			delete(pm.peers, peer)
		case <-pm.t.Dying():
			for _, peer := range pm.peers {
				go peer.Stop()
			}
			return
		}
	}
}
