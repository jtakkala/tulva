package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net"
	"net/url"
	"strconv"
	"time"
)

type UdpTracker struct {
	*tracker
	connectionId  uint64
	transactionId uint32
	serverAddr    *net.UDPAddr
	conn          *net.UDPConn
}

type connectRequest struct {
	connectionId  uint64
	action        uint32
	transactionId uint32
}

type connectResponse struct {
	action        uint32
	transactionId uint32
	connectionId  uint64
}

type errorResponse struct {
	action        uint32
	transactionId uint32
	message       string
}

type shortString []byte

type announceRequest struct {
	connectionId  uint64
	action        uint32
	transactionId uint32
	infoHash      shortString
	peerId        shortString
	downloaded    uint64
	left          uint64
	uploaded      uint64
	event         uint32
	ipAddr        uint32
	key           uint32
	numWant       int32
	port          uint16
}

type announceResponse struct {
	action        uint32
	transactionId uint32
	interval      uint32
	leechers      uint32
	seeders       uint32
	peers         []PeerTuple
}

func (r *connectRequest) MarshalBinary() ([]byte, error) {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.BigEndian, r.connectionId)
	err = binary.Write(buf, binary.BigEndian, r.action)
	err = binary.Write(buf, binary.BigEndian, r.transactionId)
	return buf.Bytes(), err
}

func (r *connectResponse) UnmarshalBinary(data []byte) error {
	buf := bytes.NewReader(data[4:])
	actionBytes := append([]byte{0, 0, 0}, data[0])
	err := binary.Read(bytes.NewReader(actionBytes), binary.BigEndian, &r.action)
	err = binary.Read(buf, binary.BigEndian, &r.transactionId)
	err = binary.Read(buf, binary.BigEndian, &r.connectionId)
	return err
}

func (r *connectResponse) MarshalBinary() ([]byte, error) {
	var b bytes.Buffer
	fmt.Fprintln(&b, r.connectionId, r.action, r.transactionId)
	return b.Bytes(), nil
}

func (r *errorResponse) UnmarshalBinary(data []byte) error {
	actionBytes := append([]byte{0, 0, 0}, data[0])
	err := binary.Read(bytes.NewReader(actionBytes), binary.BigEndian, &r.action)
	err = binary.Read(bytes.NewReader(data[4:]), binary.BigEndian, &r.transactionId)
	r.message = string(data[8:])
	return err
}

func (r *announceRequest) MarshalBinary() ([]byte, error) {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.BigEndian, r.connectionId)
	err = binary.Write(buf, binary.BigEndian, r.action)
	err = binary.Write(buf, binary.BigEndian, r.transactionId)
	err = binary.Write(buf, binary.BigEndian, r.infoHash)
	err = binary.Write(buf, binary.BigEndian, r.peerId)
	err = binary.Write(buf, binary.BigEndian, r.downloaded)
	err = binary.Write(buf, binary.BigEndian, r.left)
	err = binary.Write(buf, binary.BigEndian, r.uploaded)
	err = binary.Write(buf, binary.BigEndian, r.event)
	err = binary.Write(buf, binary.BigEndian, r.ipAddr)
	err = binary.Write(buf, binary.BigEndian, r.key)
	err = binary.Write(buf, binary.BigEndian, r.numWant)
	err = binary.Write(buf, binary.BigEndian, r.port)
	return buf.Bytes(), err
}

func (r *announceResponse) UnmarshalBinary(data []byte) error {
	buf := bytes.NewReader(data[4:])
	actionBytes := append([]byte{0, 0, 0}, data[0])
	err := binary.Read(bytes.NewReader(actionBytes), binary.BigEndian, &r.action)
	err = binary.Read(buf, binary.BigEndian, &r.transactionId)
	err = binary.Read(buf, binary.BigEndian, &r.interval)
	err = binary.Read(buf, binary.BigEndian, &r.leechers)
	err = binary.Read(buf, binary.BigEndian, &r.seeders)

	if len(data) > 20 {
		peerBytes := data[20:]
		peers := len(peerBytes) / 6

		for i := 0; i < peers; i++ {
			peer := PeerTuple{}

			ipBytes := peerBytes[i*6 : i*6+4]
			//log.Printf("ipbytes: %v\n", ipBytes)
			peer.IP = net.IPv4(ipBytes[0], ipBytes[1], ipBytes[2], ipBytes[3])
			binary.Read(bytes.NewReader(peerBytes[i*6+4:i*6+6]), binary.BigEndian, &peer.Port)

			log.Printf("peer: %s:%d\n", peer.IP.String(), peer.Port)
			r.peers = append(r.peers, peer)
		}
	}

	return err
}

func NewUdpTracker(key string, chans trackerPeerChans, port uint16, infoHash []byte, announce *url.URL) *UdpTracker {
	return &UdpTracker{&tracker{key: key, peerChans: chans, port: port, infoHash: infoHash, announceURL: announce}, 0, 0, &net.UDPAddr{}, &net.UDPConn{}}
}

func (p *PeerTuple) String() string {
	return fmt.Sprintf("%s:%d\n", p.IP.String(), p.Port)
}

func (tr *UdpTracker) Announce(event int) {
	err := tr.connect()
	if err != nil {
		log.Printf("Tracker : Could not connect to tracker %s: \n", tr.announceURL.String(), err)
		return
	}

	key, _ := strconv.ParseUint(tr.key, 16, 4)
	announce := &announceRequest{
		connectionId:  tr.connectionId,
		action:        1,
		transactionId: tr.transactionId,
		infoHash:      tr.infoHash,
		peerId:        PeerID[:],
		downloaded:    uint64(tr.stats.Downloaded),
		left:          uint64(tr.stats.Left),
		uploaded:      uint64(tr.stats.Uploaded),
		event:         uint32(event),
		ipAddr:        0,
		key:           uint32(key),
		numWant:       -1,
		port:          6881,
	}
	announceBytes, _ := announce.MarshalBinary()

	buff := make([]byte, 60000)
	length := tr.request(announceBytes, buff)

	if length >= 20 {
		var response announceResponse
		response.UnmarshalBinary(buff[:length])

		log.Printf("announce response: %+v\n", response.peers)

		if event != Stopped {
			if response.interval != 0 {
				nextAnnounce := time.Second * 120
				log.Printf("Tracker : Announce : Scheduling next announce in %v\n", nextAnnounce)
				tr.timer = time.After(nextAnnounce)
			}

			for _, peer := range response.peers {
				tr.peerChans.peers <- peer
			}
		}
	}
}

func (tr *UdpTracker) connect() error {
	log.Printf("Tracker : Connect (%v)", tr.announceURL)

	connectReq := connectRequest{connectionId: 0x41727101980, action: 0, transactionId: tr.transactionId}
	connectBytes, _ := connectReq.MarshalBinary()

	buff := make([]byte, 150)
	length := tr.request(connectBytes, buff)

	if length >= 16 {
		var response connectResponse
		response.UnmarshalBinary(buff[:length])

		if response.action == 3 || tr.transactionId != response.transactionId {
			error := errorResponse{}
			error.UnmarshalBinary(buff)
			return errors.New("Response Error: " + error.message)
		}

		tr.connectionId = response.connectionId
		return nil
	}

	return errors.New("Invalid connect response length")
}

func (tr *UdpTracker) Run() {
	log.Printf("Tracker : Run : Started (%s)\n", tr.announceURL)
	defer log.Printf("Tracker : Run : Completed (%s)\n", tr.announceURL)

	rand.Seed(time.Now().UnixNano())

	serverAddr, err := net.ResolveUDPAddr("udp", tr.announceURL.Host)
	if err != nil {
		log.Println("Tracker : Could not resolve tracker host!")
		return
	}

	var conn *net.UDPConn
	for port := 6881; err == nil && port < 6890; port++ {
		conn, err = net.ListenUDP("udp", &net.UDPAddr{Port: port})
	}
	if err != nil {
		log.Println("Tracker : Could not bind to any port in range 6881-6889")
		return
	}

	tr.transactionId = rand.Uint32()
	tr.serverAddr = serverAddr
	tr.conn = conn
	tr.Announce(Started)

	for {
		select {
		case <-tr.quit:
			log.Println("Tracker : Stop : Stopping")
			tr.Announce(Stopped)
			return
		case <-tr.completedCh:
			go tr.Announce(Completed)
		case <-tr.timer:
			log.Printf("Tracker : Run : Interval Timer Expired (%s)\n", tr.announceURL)
			go tr.Announce(Interval)
		case stats := <-tr.peerChans.stats:
			log.Println("read from stats", stats)
		}
	}
}

// Send a udp packet to the tracker, fill in the dest buffer with the response
func (tr *UdpTracker) request(payload []byte, dest []byte) int {
	n := 0
	totalAttempts := 0

	// notify the sender when a response is recieved
	recvChan := make(chan bool)

	// keep sending the packet and wait for the specifed time
	// before trying again, limit n to 8
	go func() {

		// initial send
		tr.conn.WriteTo(payload, tr.serverAddr)
	Listen:
		for {
			// timeout: 15 * 2 ^ n (0-8)
			timeout := time.Second * time.Duration(15 * int(math.Pow(2.0, float64(n))))
			timer := time.After(timeout)
			select {
			case <-recvChan:
				break Listen
			case <-timer:
				tr.conn.WriteTo(payload, tr.serverAddr)
				if n < 8 {
					n++
				}
				totalAttempts++
			}
		}
	}()

	// read until some data is received
	var err error
	length := 0
	for length == 0 {
		length, _, err = tr.conn.ReadFrom(dest)

		// not sure what to do here?
		if err != nil {
			log.Println("Tracker : udp packet read error")
		}
	}

	recvChan <- true
	return length
}
