package main

import (
	"log"
)

type UdpTracker tracker


func (tr *UdpTracker) Announce(event int) {
	log.Println("udp announce")
}

func (tr *UdpTracker) Run() {
	log.Println("udp tracking running")
}

