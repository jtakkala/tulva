package main

import (
	"testing"
	"strconv"
	"time"
)

func assertOrder(t *testing.T, sortedPieceSlice []int, index int, expectedPieceNum int) {
	if sortedPieceSlice[index] != expectedPieceNum { 
		t.Errorf("Expected sorted[%d] to be piece number %d, but it was %d", index, expectedPieceNum, sortedPieceSlice[index]) 
	}
}

// Create two PiecePriority objects with the same activeRequestsTotal but 
// different rarity index. Confirm that they're sorted by rarity index in
// ascending order. 
func TestPiecePrioritySliceSortingOne(t *testing.T) {

	pps := make(PiecePrioritySlice, 0)
	
	ppOne := new(PiecePriority)
	ppOne.pieceNum = 1
	ppOne.activeRequestsTotal = 0
	ppOne.rarityIndex = 3
	pps = append(pps, *ppOne)

	ppTwo := new(PiecePriority)
	ppTwo.pieceNum = 2
	ppTwo.activeRequestsTotal = 0
	ppTwo.rarityIndex = 1
	pps = append(pps, *ppTwo)

	sorted := pps.toSortedPieceSlice()

	assertOrder(t, sorted, 0, 2)
	assertOrder(t, sorted, 1, 1)

}

// Create two PiecePriority objects with different activeRequestTotal values
// and different rarityIndex values. Confirm that the sorting is based on 
// activeRequestTotal and NOT rarityIndex
func TestPiecePrioritySliceSortingTwo(t *testing.T) {

	pps := make(PiecePrioritySlice, 0)
	
	ppOne := new(PiecePriority)
	ppOne.pieceNum = 1
	ppOne.activeRequestsTotal = 0
	ppOne.rarityIndex = 3
	pps = append(pps, *ppOne)

	ppTwo := new(PiecePriority)
	ppTwo.pieceNum = 2
	ppTwo.activeRequestsTotal = 1
	ppTwo.rarityIndex = 1
	pps = append(pps, *ppTwo)

	sorted := pps.toSortedPieceSlice()

	assertOrder(t, sorted, 0, 1)
	assertOrder(t, sorted, 1, 2)

}

// An additional test using eight PiecePriority objects with varying 
// activeRequestsTotal and rarityIndex values
func TestPiecePrioritySliceSortingThree(t *testing.T) {

	pps := make(PiecePrioritySlice, 0)

	// Based on an example in "Go BitTorrent Planning"
	pps = append(pps, PiecePriority{2, 2, 5})
	pps = append(pps, PiecePriority{3, 1, 2})
	pps = append(pps, PiecePriority{4, 1, 6})
	pps = append(pps, PiecePriority{5, 1, 7})
	pps = append(pps, PiecePriority{6, 0, 0})
	pps = append(pps, PiecePriority{7, 1, 3})
	pps = append(pps, PiecePriority{8, 0, 1})
	pps = append(pps, PiecePriority{9, 1, 4})

	sorted := pps.toSortedPieceSlice()

	assertOrder(t, sorted, 0, 6)
	assertOrder(t, sorted, 1, 8)
	assertOrder(t, sorted, 2, 3)
	assertOrder(t, sorted, 3, 7)
	assertOrder(t, sorted, 4, 9)
	assertOrder(t, sorted, 5, 4)
	assertOrder(t, sorted, 6, 5)
	assertOrder(t, sorted, 7, 2)

}

func AssertRaritySliceValue(t *testing.T, raritySlice []int, index int, expectedValue int) {
	if raritySlice[index] != expectedValue {
		t.Errorf("Expected rs[%d] to be %d but it was %d", index, expectedValue, raritySlice[index])
	}
}

// Add a single pair and convert it to a rarity slice
func TestRarityMapOneValue(t *testing.T) {

	rm := NewRarityMap()
	rm.put(5, 3)
	rs := rm.getPiecesByRarity()

	if len(rs) != 1 {
		t.Errorf("Expected slice len to be %d but it was %d",1, len(rs))
	}
	if rs[0] != 3 {
		t.Errorf("Expected rs[%d] to be %d but it was %d", 0, 3, rs[0])
	}
}

// Put two pairs with the same rarity and convert it to a rarity slice
func TestRarityMapTwoValuesSameRarity(t *testing.T) {
	
	rm := NewRarityMap()
	rm.put(5, 3)
	rm.put(5, 2)
	rs := rm.getPiecesByRarity()

	if len(rs) != 2 {
		t.Errorf("Expected slice len to be %d but it was %d", 2, len(rs))
	}

	AssertRaritySliceValue(t, rs, 0, 3)
	AssertRaritySliceValue(t, rs, 1, 2)

}

// Put two pairs with different rarity and convert it to a rarity slice
func TestRarityMapTwoValuesDiffRarity(t *testing.T) {

	rm := NewRarityMap()
	rm.put(5, 3)
	rm.put(4, 2)
	rs := rm.getPiecesByRarity()

	if len(rs) != 2 {
		t.Errorf("Expected slice len to be %d but it was %d", 2, len(rs))
	}

	AssertRaritySliceValue(t, rs, 0, 2)
	AssertRaritySliceValue(t, rs, 1, 3)

}

// Put several values with some overlapping rarity and convert it to a rarity slice
func TestRaritySeveralValues(t *testing.T) {

	rm := NewRarityMap()
	rm.put(4, 1)
	rm.put(3, 2)
	rm.put(2, 3)
	rm.put(3, 4)
	rm.put(4, 5)
	rm.put(1, 6)
	rm.put(2, 7)
	rm.put(1, 8)
	rm.put(2, 9)
	rm.put(1, 10)

	rs := rm.getPiecesByRarity()

	if len(rs) != 10 {
		t.Errorf("Expected slice len to be %d but it was %d", 10, len(rs))
	}

	AssertRaritySliceValue(t, rs, 0, 6)
	AssertRaritySliceValue(t, rs, 1, 8)
	AssertRaritySliceValue(t, rs, 2, 10)
	AssertRaritySliceValue(t, rs, 3, 3)
	AssertRaritySliceValue(t, rs, 4, 7)
	AssertRaritySliceValue(t, rs, 5, 9)
	AssertRaritySliceValue(t, rs, 6, 2)
	AssertRaritySliceValue(t, rs, 7, 4)
	AssertRaritySliceValue(t, rs, 8, 1)
	AssertRaritySliceValue(t, rs, 9, 5)
		
}

func createDummyPieceHashSlice(sliceLength int) []string {
	pieceHashes := make([]string, sliceLength)
	for index := 0; index < sliceLength; index++ {
		pieceHashes[index] = strconv.FormatInt(int64(index), 10)
	} 
	return pieceHashes
}

func createTestController() *Controller {
	finishedPieces := []bool{true, false, false, false, false, false, false, false, false, true}
	pieceHashes := createDummyPieceHashSlice(len(finishedPieces))
	controllerRxChans := NewControllerRxChans(
		NewControllerDiskIOChans(),
		NewControllerPeerManagerChans(),
		NewPeerControllerChans())
	return NewController(finishedPieces, pieceHashes, controllerRxChans)
}

func TestControllerRunStop(t *testing.T) {

	cont := createTestController()
	go cont.Run()
	cont.Stop()
}

func TestControllerNewPeerReceiveFinishedBitfield(t *testing.T) {

	cont := createTestController()
	go cont.Run()
	defer cont.Stop()

	peer1Comms := NewPeerComms("1.2.3.4:1234", *NewControllerPeerChans())

	cont.rxChans.peerManager.newPeer <- *peer1Comms

	// Since this is a new peer, we expect the controller to send the entire bitfield over the HavePiece
	// channel. 
	// Emulate the peer by receiving the entire bitfield over the HavePiece chan from the controller
	innerChan := <- peer1Comms.chans.havePiece

	receivedBitField := make([]bool, len(cont.finishedPieces))
	for havePiece := range innerChan {
		receivedBitField[havePiece.pieceNum] = true
	}

	for pieceNum, havePiece := range receivedBitField {
		if cont.finishedPieces[pieceNum] != havePiece {
			t.Errorf("After receiving bitfield from controller, expected pieceNum %d to be %t but it was %t", pieceNum, cont.finishedPieces[pieceNum], havePiece)
		}
	}
	
}

func TestControllerAskNewPeerToGetOnePiece(t *testing.T) {

	cont := createTestController()
	go cont.Run()
	defer cont.Stop()

	peer1Name := "1.2.3.4:1234"
	peer1Comms := NewPeerComms(peer1Name, *NewControllerPeerChans())

	cont.rxChans.peerManager.newPeer <- *peer1Comms

	// Ignore the bitfield that the controller is sending to the peer

	// The peer is expected to send its entire bitfield to the controller. Emulate the peer by sending
	// the controller a fake bitfield. 

	// peer1 has pieces 0, 1
	peer1Bitfield := []bool{true, true, false, false, false, false, false, false, false, false}
	sendBitfieldOverChannel(cont.rxChans.peer.havePiece, peer1Name, peer1Bitfield)

	// Sleep briefly to give the controller a chance to process the bitfield
	time.Sleep(50 * time.Millisecond)

	// The peer is still choked (by default it starts choked). Confirm that the controller doesn't ask the
	// peer to request any pieces. 
	select {
	case <- peer1Comms.chans.requestPiece:
		t.Errorf("The controller sent a peer a request before the peer was unchoked")

	default:
		// Pass. The peer didn't receive any piece request
	}

	// Tell the controller that the peer is now unchoked
	cont.rxChans.peer.chokeStatus <- PeerChokeStatus{ peer1Name, false }

	// Sleep briefly to give the controller a chance to process the message
	time.Sleep(50 * time.Millisecond)

	// The controller should now tell the peer to retrieve only piece 1, since the controller 
	// already has piece 0
	select {
	case request := <- peer1Comms.chans.requestPiece:
		if request.pieceNum != 1 {
			t.Errorf("Expected the controller to request piece number %d but received request for piece number %d", 1, request.pieceNum)
		} 
	default:
		t.Errorf("We expected the controller to request piece number %d, but it wasn't received", 1)
	}


	// Sleep briefly to give the controller a chance to queue another message if it intends to
	time.Sleep(50 * time.Millisecond)

	select {
	case request := <- peer1Comms.chans.requestPiece:
		t.Errorf("The controller sent the peer a second requests for piece number %d, but we didn't expect any more requests", request.pieceNum)

	default:
		// Pass. We didn't receive any additional request over the channel 
	}


}


/*
func TestControllerNewPeerSendControllerPeerBitfieldTwo(t *testing.T) {

	cont := createTestController()
	go cont.Run()
	defer cont.Stop()

	peer1Name := "1.2.3.4:1234"
	peer1Comms := NewPeerComms(peer1Name, *NewControllerPeerChans())

	cont.rxChans.peerManager.newPeer <- *peer1Comms

	// Ignore the bitfield that the controller is sending to the peer

	// The peer is expected to send its entire bitfield to the controller. Emulate the peer by sending
	// the controller a fake bitfield. 

	// peer1 has pieces 0, 1, 3, 4 and 8
	peer1Bitfield := []bool{true, true, false, true, true, false, false, false, true, false}
	sendBitfieldOverChannel(cont.rxChans.peer.havePiece, peer1Name, peer1Bitfield)

	// The controller should now tell the peer to retrieve pieces 1, 3, 4 and 8


}*/


