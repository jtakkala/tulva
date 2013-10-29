// Copyright 2013 Jari Takkala. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
//	"code.google.com/p/bencode-go"
/*
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
*/
)

// Peers dictionary model
type Peers struct {
	PeerId string "peer id"
	Ip     string
	Port   int
}

// Tracker Response
type TrackerResponse struct {
	FailureReason  string "failure reason"
	WarningMessage string "warning message"
	Interval       int
	MinInterval    int    "min interval"
	TrackerId      string "tracker id"
	Complete       int
	Incomplete     int
	Peers          string "peers"
	//TODO: Figure out how to handle dict of peers
//	Peers          []Peers "peers"
}

type Tracker struct {
	url string
	trackerResponse TrackerResponse
}

