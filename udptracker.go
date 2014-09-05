// Copyright 2014 Rob Bassi. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"log"
	"math"
	"math/rand"
	"net"
	"net/url"
	"strconv"
	"time"
)

const (
	initialConnectionId = 0x41727101980
	connectMinResponseLength = 16
	announceMinResponseLength = 20
	connectBufferSize = 150
	announceBufferSize = 20000
)

type UdpTracker struct {
	*tracker
	ConnectionId  uint64
	TransactionId uint32
	ServerAddr    *net.UDPAddr
	Conn          *net.UDPConn
}

type connectRequest struct {
	ConnectionId  uint64
	Action        uint32
	TransactionId uint32
}

type connectResponse struct {
	Action        uint32
	TransactionId uint32
	ConnectionId  uint64
}

type errorResponse struct {
	Action        uint32
	TransactionId uint32
	Message       string
}

type announceRequest struct {
	ConnectionId  uint64
	Action        uint32
	TransactionId uint32
	InfoHash      [20]byte
	PeerId        [20]byte
	Downloaded    uint64
	Left          uint64
	Uploaded      uint64
	Event         uint32
	IpAddr        uint32
	Key           uint32
	NumWant       int32
	Port          uint16
}

type announceResponse struct {
	Action        uint32
	TransactionId uint32
	Interval      uint32
	Leechers      uint32
	Seeders       uint32
	Peers         []PeerTuple
}

func (r *connectRequest) MarshalBinary() ([]byte, error) {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.BigEndian, r)
	return buf.Bytes(), err
}

func (r *connectResponse) UnmarshalBinary(data []byte) error {
	buf := bytes.NewReader(data)
	err := binary.Read(buf, binary.BigEndian, r)
	return err
}

func (r *errorResponse) UnmarshalBinary(data []byte) error {
	buf := bytes.NewReader(append([]byte{0,0,0,data[0]}, data[4:]...))
	err := binary.Read(buf, binary.BigEndian, &r.Action)
	if err != nil {
		return err
	}

	err = binary.Read(buf, binary.BigEndian, &r.TransactionId)
	if err != nil {
		return err
	}

	r.Message = string(data[8:])
	return err
}

func (r *announceRequest) MarshalBinary() ([]byte, error) {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.BigEndian, r)
	return buf.Bytes(), err
}

func (r *announceResponse) UnmarshalBinary(data []byte) error {
	buf := bytes.NewReader(data)

	err := binary.Read(buf, binary.BigEndian, &r.Action)
	if err != nil {
		return err
	}

	err = binary.Read(buf, binary.BigEndian, &r.TransactionId)
	if err != nil {
		return err
	}
	
	err = binary.Read(buf, binary.BigEndian, &r.Interval)
	if err != nil {
		return err
	}

	err = binary.Read(buf, binary.BigEndian, &r.Leechers)
	if err != nil {
		return err
	}

	err = binary.Read(buf, binary.BigEndian, &r.Seeders)
	if err != nil {
		return err
	}

	if len(data) > announceMinResponseLength {
		peerBytes := bytes.NewReader(data[announceMinResponseLength:])
		peers := (len(data) - announceMinResponseLength) / 6
		
		for i := 0; i < peers; i++ {
			var peer PeerTuple
			var ipBuf [4]byte

			err = binary.Read(peerBytes, binary.BigEndian, &ipBuf)
			if err != nil {
				return err
			}

			err = binary.Read(peerBytes, binary.BigEndian, &peer.Port)
			if err != nil {
				return err
			}
			
			peer.IP = net.IPv4(ipBuf[0], ipBuf[1], ipBuf[2], ipBuf[3])
			r.Peers = append(r.Peers, peer)
		}
	}
	return nil
}

func NewUdpTracker(key string, chans trackerPeerChans, port uint16, infoHash []byte, announce *url.URL) *UdpTracker {
	return &UdpTracker{&tracker{key: key, peerChans: chans, port: port, infoHash: infoHash, announceURL: announce}, 0, 0, &net.UDPAddr{}, &net.UDPConn{}}
}

func (tr *UdpTracker) Announce(event int) {
	err := tr.connect()
	if err != nil {
		log.Printf("Tracker : Could not connect to tracker %s: \n", tr.announceURL.String(), err)
		return
	}

	key, _ := strconv.ParseUint(tr.key, 16, 4)
	announce := &announceRequest{
		ConnectionId:  tr.ConnectionId,
		Action:        1,
		TransactionId: tr.TransactionId,
		PeerId: PeerID,
		Downloaded:    uint64(tr.stats.Downloaded),
		Left:          uint64(tr.stats.Left),
		Uploaded:      uint64(tr.stats.Uploaded),
		Event:         uint32(event),
		IpAddr:        0,
		Key:           uint32(key),
		NumWant:       -1,
		Port:          6881,
	}
	copy(announce.InfoHash[:20], tr.infoHash)
	announceBytes, err := announce.MarshalBinary()
	if err != nil {
		log.Println("Tracker : Announce : Invalid response")
		return
	}

	buff := make([]byte, announceBufferSize)
	length := tr.request(announceBytes, buff)

	if length >= announceMinResponseLength {
		var response announceResponse
		response.UnmarshalBinary(buff[:length])

		if event != Stopped {
			if response.Interval != 0 {
				nextAnnounce := time.Second * time.Duration(response.Interval)
				log.Println("Tracker : Announce : Scheduling next announce in", nextAnnounce)
				tr.timer = time.After(nextAnnounce)
			}

			for _, peer := range response.Peers {
				tr.peerChans.peers <- peer
			}
		}
	}
}

func (tr *UdpTracker) connect() error {
	log.Printf("Tracker : Connect (%v)", tr.announceURL)

	connectReq := connectRequest{ConnectionId: initialConnectionId, Action: 0, TransactionId: tr.TransactionId}
	connectBytes, _ := connectReq.MarshalBinary()

	buff := make([]byte, connectBufferSize)
	length := tr.request(connectBytes, buff)

	if length >= connectMinResponseLength {
		var response connectResponse
		err := response.UnmarshalBinary(buff[:length])
		if err != nil {
			log.Println("Tracker : Connect : Invalid response")
			return err
		}

		if response.Action == 3 || tr.TransactionId != response.TransactionId {
			error := errorResponse{}
			err := error.UnmarshalBinary(buff)
			if err != nil {
				return errors.New(error.Message)
			} else {
				return err
			}
		}

		tr.ConnectionId = response.ConnectionId
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

	conn, err := net.ListenUDP("udp", &net.UDPAddr{Port: 0})
	if err != nil {
		log.Println("Tracker : Could not bind to any port")
		return
	}

	tr.TransactionId = rand.Uint32()
	tr.ServerAddr = serverAddr
	tr.Conn = conn
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
		tr.Conn.WriteTo(payload, tr.ServerAddr)
	Listen:
		for {
			// timeout: 15 * 2 ^ n (0-8)
			timeout := time.Second * time.Duration(15 * int(math.Pow(2.0, float64(n))))
			timer := time.After(timeout)
			select {
			case <-recvChan:
				break Listen
			case <-timer:
				tr.Conn.WriteTo(payload, tr.ServerAddr)
				totalAttempts++
				if n < 8 { n++ }
			}
		}
	}()

	// read until some data is received
	var err error
	length := 0
	for length == 0 {
		length, _, err = tr.Conn.ReadFrom(dest)

		// not sure what to do here?
		if err != nil {
			log.Println("Tracker : udp packet read error")
		}
	}

	recvChan <- true
	return length
}
