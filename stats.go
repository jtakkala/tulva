// Copyright 2013 Jari Takkala and Brian Dignan. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"log"
	"time"
)

type Stats struct {
	peerCh   chan PeerStats   // receive stats counters from peers
	diskIOCh chan int         // receive bytes written from diskIO
	ticker   <-chan time.Time // print updates every tick

	Left       int // bytes left to download
	Uploaded   int // total bytes uploaded
	Downloaded int // total bytes downloaded
	Errors     int // total errors
}

func NewStats(bytesLeft int, diskIOCh chan int) *Stats {
	return &Stats{
		Left:     bytesLeft,
		peerCh:   make(chan PeerStats),
		ticker:   make(chan time.Time),
		diskIOCh: diskIOCh,
	}
}

func (s *Stats) Run() {
	log.Println("Stats : Run : Started")
	defer log.Println("Stats : Run : Stopped")

	s.ticker = time.Tick(time.Second)

	for {
		select {
		case stat := <-s.peerCh:
			s.Downloaded += stat.read
			s.Uploaded += stat.write
			s.Errors += stat.errors
		case bytesWritten := <-s.diskIOCh:
			s.Left -= bytesWritten
		case <-s.ticker:
			fmt.Printf("\033[31mDownloaded: %d, Left: %d, Uploaded: %d, Errors: %d\033[0m\n", s.Downloaded, s.Left, s.Uploaded, s.Errors)
		}
	}
}
