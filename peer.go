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
	"sync"
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
	MsgBlock // This is defined in the spec as piece, but in reality it's a block
	MsgCancel
	MsgPort
)

const (
	downloadBlockSize = 16384
	maxSimultaneousBlockDownloads = 5
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
	pieceLength		 int
	currentDownload  *PieceDownload
	diskIOChans      diskIOPeerChans
	peerManagerChans peerManagerChans
	contRxChans      ControllerPeerChans
	contTxChans      PeerControllerChans
	stats            PeerStats
	t                tomb.Tomb
}

type PieceDownload struct {
	pieceNum int
	expectedHash []byte
	piece []byte
	numBlocksReceived int
	numOutstandingBlocks int
	totalNumBlocks int
}

func NewPieceDownload(requestPiece RequestPiece, pieceLength int) *PieceDownload {
	pd := new(PieceDownload)
	pd.pieceNum = requestPiece.pieceNum
	pd.expectedHash = requestPiece.expectedHash
	pd.piece = make([]byte, pieceLength)
	pd.totalNumBlocks = pieceLength / downloadBlockSize
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
	pieceLength     int
	peerChans     peerManagerChans
	serverChans   serverPeerChans
	trackerChans  trackerPeerChans
	diskIOChans   diskIOPeerChans
	contChans     ControllerPeerManagerChans
	peerContChans PeerControllerChans
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

func NewPeerManager(infoHash []byte, numPieces int, pieceLength int, diskIOChans diskIOPeerChans, serverChans serverPeerChans, trackerChans trackerPeerChans) *PeerManager {
	pm := new(PeerManager)
	pm.infoHash = infoHash
	pm.numPieces = numPieces
	pm.pieceLength = pieceLength
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

func connectToPeer(peerTuple PeerTuple, connCh chan *net.TCPConn) {
	raddr := net.TCPAddr{peerTuple.IP, int(peerTuple.Port), ""}
	log.Println("Connecting to", raddr)
	conn, err := net.DialTCP("tcp4", nil, &raddr)
	if err != nil {
		if e, ok := err.(*net.OpError); ok {
			if e.Err == syscall.ECONNREFUSED {
				log.Println("Peer : connectToPeer : ", raddr, err)
				return
			}
			if e.Err == syscall.ETIMEDOUT {
				log.Println("Peer : connectToPeer : ", raddr, err)
				return
			}
		}
		log.Fatal("Peer : connectToPeer : ", raddr, err)
	}
	log.Println("Peer : connectToPeer : Connected:", raddr)
	connCh <- conn
}

func NewPeer(
	peerName string,
	infoHash []byte,
	numPieces int,
	pieceLength int,
	diskIOChans diskIOPeerChans,
	contRxChans ControllerPeerChans,
	contTxChans PeerControllerChans) *Peer {
	p := &Peer{
		peerName:       peerName,
		infoHash:       infoHash,
		pieceLength:    pieceLength,
		peerBitfield:   make([]bool, numPieces),
		ourBitfield:    make([]bool, numPieces),
		keepalive:	make(chan time.Time),
		amChoking:      true,
		amInterested:   false,
		peerChoking:    true,
		peerInterested: false,
		diskIOChans:    diskIOChans,
		contRxChans:    contRxChans,
		contTxChans:    contTxChans}
	return p
}

func constructMessage(id int, payload []byte) (msg []byte, err error) {
	msg = make([]byte, 4)

	// Store the length of payload + id in network byte order
	binary.BigEndian.PutUint32(msg, uint32(len(payload)+1))
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
	sliceLen := len(bitfield)/8 // 8 bits per byte
	if len(bitfield) % 8 != 0 {
		// add one more byte because the bitfield doesn't fit evenly into a byte slice
		sliceLen += 1 
	}
	result := make([]byte, sliceLen)

	for i := 0; i < len(result); i++ {
		orValue := byte(128)
		for j := 0; j < 8; j++ {
			bitfieldIndex := ((i * 8) + j)
			if bitfieldIndex > len(bitfield) {
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

			// Tell the controller that we've switched from unchoked to choked
			go func() {
				p.contTxChans.chokeStatus <- PeerChokeStatus{peerName: p.peerName, isChoked: true}
			}()
		} else {
			// Ignore choke message because we're already choked.
		}
		break
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
		break
	case MsgInterested:
		if len(payload) != 0 {
			log.Fatalf("Received an Interested from %s with invalid payload size of %d", p.peerName, len(payload))
		} else {
			log.Printf("Received an Interested message from %s", p.peerName)
		}
		p.peerInterested = true
		p.sendUnchoke()

		break
	case MsgNotInterested:
		// Not Interested Message
		if len(payload) != 0 {
			log.Fatalf("Received a Not Interested from %s with invalid payload size of %d", p.peerName, len(payload))
		} else {
			log.Printf("Received a Not Interested message from %s", p.peerName)
		}
		p.peerInterested = false
		p.sendChoke()

		break
	case MsgHave:
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
	case MsgBitfield:
		log.Printf("Received a Bitfield message from %s", p.peerName)

		p.peerBitfield = convertByteSliceToBoolSlice(len(p.peerBitfield), payload)

		// Break the bitfield into a slice of HavePiece structs and send them
		// to the controller
		go p.sendBitfieldToController(p.peerBitfield)

		break
	case MsgRequest:
		// IMPLEMENT ME
		pieceNum := 0 // FIXME

		log.Printf("Received a Request message for piece %d from %s", pieceNum, p.peerName)
		break
	case MsgBlock:
		expectedPayloadSize := 8 + downloadBlockSize
		if len(payload) != expectedPayloadSize {
			log.Fatalf("Received a Block (Piece) message from %s with invalid payload size of %d. Expected %d", p.peerName, len(payload), expectedPayloadSize)
		}

		pieceNumBytes := payload[0:4]
		beginBytes := payload[4:8]
		blockBytes := payload[8:]

		pieceNum := binary.BigEndian.Uint32(pieceNumBytes)
		begin := binary.BigEndian.Uint32(beginBytes)


		if p.currentDownload == nil {
			log.Fatalf("Received a Block (Piece) message from %s but there aren't any current downloads", p.peerName)
		} else if begin % downloadBlockSize != 0 {
			log.Fatalf("Received a Block (Piece) message from %s with an invalid begin value of %d", p.peerName, begin)
		} else {
			log.Printf("Received a Block (Piece) message from %s for piece %d begin %d with %d bytes of block data", p.peerName, pieceNum, begin, len(blockBytes))
		}

		// The block (piece) message is valid. Write the contents to the buffer. 
		//p.currentDownload.piece


		
		break
	case MsgCancel:
		// IMPLEMENT ME
		pieceNum := 0 // FIXME

		log.Printf("Received a Cancel message for piece %d from %s", pieceNum, p.peerName)
		break
	case MsgPort:
		log.Printf("Ignoring a Port message that was received from %s", p.peerName)
		break
	}
}

func (p *Peer) reader() {
	log.Println("Peer : reader : Started")
	defer log.Println("Peer : reader : Completed")

	var handshake Handshake
	err := binary.Read(p.conn, binary.BigEndian, &handshake)
	if err != nil {
		if err == io.EOF {
			log.Println("Peer : reader : binary.Read :", p.peerName, err)
			p.Stop()
			return
		}
		log.Fatal("Peer : reader : binary.Read :", p.peerName, err)
	}

	p.lastRxMessage = time.Now()
	p.stats.addRead(int(reflect.TypeOf(handshake).Size()))

	err = verifyHandshake(&handshake, p.infoHash)
	if err != nil {
		p.conn.Close()
		return
	}
	p.peerID = handshake.PeerID[:]

	for {
		length := make([]byte, 4)
		n, err := io.ReadFull(p.conn, length)
		if err != nil {
			if err == io.EOF {
				log.Println("Peer : reader : io.ReadFull :", p.peerName, err)
				p.Stop()
				return
			}
			log.Fatal("Peer : reader : io.ReadFull :", p.peerName, err)
		}
		p.lastRxMessage = time.Now()
		p.stats.addRead(n)

		payload := make([]byte, binary.BigEndian.Uint32(length))
		n, err = io.ReadFull(p.conn, payload)
		if err != nil {
			// FIXME if this is not a keepalive, we should
			// definitely get a payload
			if err == io.EOF {
				log.Println("Peer : reader : io.ReadFull :", p.peerName, err)
				p.Stop()
				return
			}
			log.Fatal("Peer : reader : io.ReadFull :", p.peerName, err)
		}
		p.lastRxMessage = time.Now()
		p.stats.addRead(n)

		log.Printf("Read %d bytes of %x\n", (n + 4), payload)
		p.decodeMessage(payload)
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
		// TODO: Handle errors
		log.Fatal(err)
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
		if err.Error() == "use of closed network connection" {
			log.Println(err)
		} else {
			log.Fatal(err)
		}
		return
	}

	p.lastTxMessage = time.Now()
	p.stats.addWrite(4)
}

// Sends any message besides a handshake or a keepalive, both of which
// don't have a beginning LEN-ID structure. The length is automatically calculated.
func (p *Peer) sendMessage(ID int, payload interface{}) {

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

	// Write the message over TCP to the peer
	message := messageBuffer.Bytes()
	log.Printf("TEMP: Sending over TCP: %v", message)
	err = binary.Write(p.conn, binary.BigEndian, message)
	if err != nil {
		log.Fatal(err)
	}
	p.lastTxMessage = time.Now()
	p.stats.addWrite(len(message))
}

func (p *Peer) sendChoke() {
	log.Printf("Peer : sendChoke : Sending choke to %s", p.peerName)
	go p.sendMessage(MsgChoke, make([]byte, 0))
	p.amChoking = true
}

func (p *Peer) sendUnchoke() {
	log.Printf("Peer : sendUnchoke : Sending unchoke to %s", p.peerName)
	go p.sendMessage(MsgUnchoke, make([]byte, 0))
	p.amChoking = false
}

func (p *Peer) sendInterested() {
	log.Printf("Peer : sendInterested : Sending interested to %s", p.peerName)
	p.sendMessage(MsgInterested, make([]byte, 0))
	p.amInterested = true
}

func (p *Peer) sendNotInterested() {
	log.Printf("Peer : sendNotInterested : Sending not-interested to %s", p.peerName)
	p.sendMessage(MsgNotInterested, make([]byte, 0))
	p.amInterested = false
}

func (p *Peer) sendHave(pieceNum int) {
	log.Printf("Peer : sendHave : Sending have to %s for piece %d", p.peerName, pieceNum)
	payloadBuffer := new(bytes.Buffer)
	err := binary.Write(payloadBuffer, binary.BigEndian, uint32(pieceNum))
	if err != nil {log.Fatal(err)}
	p.sendMessage(MsgHave, payloadBuffer.Bytes())
}

func (p *Peer) sendBitfield() {
	log.Printf("Peer : sendBitfield : Sending bitfield to %s", p.peerName)
	compacted := convertBoolSliceToByteSlice(p.ourBitfield)
	p.sendMessage(MsgBitfield, compacted)
}

func (p *Peer) sendRequest(pieceNum int, begin int, length int) {
	log.Printf("Peer : sendBitfield : Sending Request to %s for piece %d with begin %d and offset %d", p.peerName, pieceNum, begin, length)
	buffer := new(bytes.Buffer)

	ints := []uint32{uint32(pieceNum), uint32(begin), uint32(length)}

	err := binary.Write(buffer, binary.BigEndian, ints)
	if err != nil {log.Fatal(err)}

	p.sendMessage(MsgRequest, buffer.Bytes())
}

func (p *Peer) sendRequestByNum(pieceNum int, blockNum int) {
	begin := downloadBlockSize * blockNum
	p.sendRequest(pieceNum, begin, downloadBlockSize)
}

func (p *Peer) sendInitialRequests() {
	for p.currentDownload.numOutstandingBlocks < maxSimultaneousBlockDownloads && 
		(p.currentDownload.numOutstandingBlocks + p.currentDownload.numBlocksReceived) < p.currentDownload.totalNumBlocks {

		// We haven't yet hit the max simultaneous request limit.
		// We also haven't yet requested every block.

		// Send a request for another block. 
		blockNum := p.currentDownload.numBlocksReceived + p.currentDownload.numOutstandingBlocks
		p.sendRequestByNum(p.currentDownload.pieceNum, blockNum)
		p.currentDownload.numOutstandingBlocks += 1
	}
}

func (p *Peer) sendBlock(pieceNum int, begin int, block []byte) {
	buffer := new(bytes.Buffer)

	ints := []uint32{uint32(pieceNum), uint32(begin)}

	err := binary.Write(buffer, binary.BigEndian, ints)
	if err != nil {log.Fatal(err)}

	err = binary.Write(buffer, binary.BigEndian, block)
	if err != nil {log.Fatal(err)}

	p.sendMessage(MsgBlock, buffer.Bytes())
}

func (p *Peer) sendCancel(pieceNum int, begin int, length int) {
	buffer := new(bytes.Buffer)

	ints := []uint32{uint32(pieceNum), uint32(begin), uint32(length)}

	err := binary.Write(buffer, binary.BigEndian, ints)
	if err != nil {log.Fatal(err)}

	p.sendMessage(MsgCancel, buffer.Bytes())
}

func (p *Peer) Stop() error {
	log.Println("Peer : Stop : Stopping:", p.peerName)
	p.t.Kill(nil)
	return p.t.Wait()
}

func (p *Peer) Run() {
	log.Println("Peer : Run : Started")
	defer log.Println("Peer : Run : Completed")

	//initialBitfieldSentToPeer := false
	p.keepalive = time.Tick(time.Second * 1)

	p.sendHandshake()
	go p.reader()

	for {
		select {
		case t := <-p.keepalive:
			if p.lastTxMessage.Add(time.Second * 120).Before(t) {
				log.Println("No txMessage for 120 seconds", p.peerName, p.lastTxMessage.Unix(), t.Unix())
				p.sendKeepalive()
			}
			if p.lastRxMessage.Add(time.Second * 120).Before(t) {
				log.Println("No RxMessage for 120 seconds", p.peerName, p.lastRxMessage.Unix(), t.Unix())
				p.Stop()
			}
		case requestPiece := <-p.contRxChans.requestPiece:
			log.Printf("Peer : Run : Controller told %s to get piece number %d", p.peerName, requestPiece.pieceNum)

			// check to see if we're currently downloading another piece. If so, then there's a 
			// bug because the controller should only ask us to download one at a time. 
			if p.currentDownload != nil {
				log.Fatal("Peer : Run : %s was told to download piece %d, but it's still downloading piece %d", p.peerName, requestPiece.pieceNum, p.currentDownload.pieceNum)
			}

			// Create a new PieceDownload struct for the piece that we're told to download
			p.currentDownload = NewPieceDownload(requestPiece, p.pieceLength)

			// Send the first set of block requests all at once. When we get response (piece) messages,
			// we'll then determine if more need to be set. 
			go p.sendInitialRequests()

		/*
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
					pm.numPieces,
					pm.pieceLength,
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
				go connectToPeer(peer, pm.serverChans.conns)
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
