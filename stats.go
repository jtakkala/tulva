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
	Left       int
	Uploaded   int
	Downloaded int
	Errors     int
	peerCh     chan PeerStats
	ticker     <-chan time.Time
}

func NewStats() *Stats {
	return &Stats{
		peerCh: make(chan PeerStats),
		ticker: make(chan time.Time),
	}
}

func (s *Stats) Run() {
	log.Println("Stats : Run : Started")
	defer log.Println("Stats : Run : Stopped")

	s.ticker = time.Tick(time.Second * 1)

	for {
		select {
		case stat := <-s.peerCh:
			s.Downloaded += stat.read
			s.Uploaded += stat.write
			s.Errors += stat.errors
		case <-s.ticker:
			fmt.Printf("\033[31mDownloaded: %d, Uploaded: %d, Errors: %d\033[0m\n", s.Downloaded, s.Uploaded, s.Errors)
		}
	}
}
