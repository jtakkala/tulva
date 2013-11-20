package main

import (
	"testing"
	"strconv"
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

func createTestController(controllerRxChannels *ControllerRxChannels) *Controller {
	finishedPieces := []bool{true, false, false, false, false, false, false, false, false, true}
	pieceHashes := createDummyPieceHashSlice(len(finishedPieces))
	//controllerRxChannels := NewControllerRxChannels()
	return NewController(finishedPieces, pieceHashes, controllerRxChannels)
}

func TestControllerRunStop(t *testing.T) {

	crc := new(ControllerRxChannels)
	crc.receivedPiece = make(chan ReceivedPiece)
	crc.newPeer = make(chan PeerComms)
	crc.peerChokeStatus = make(chan PeerChokeStatus)
	crc.havePiece = make(chan chan HavePiece)

	cont := createTestController(crc)
	go cont.Run()
	cont.Stop()
}

func TestControllerNewPeerReceiveFinishedBitfield(t *testing.T) {

	crc := new(ControllerRxChannels)

	receivedPieceCh := make(chan ReceivedPiece)
	newPeerCh := make(chan PeerComms)
	peerChokeStatusCh := make(chan PeerChokeStatus)
	havePieceCh := make(chan chan HavePiece)
	crc := NewControllerRxChannels(receivedPieceCh, newPeerCh, peerChokeStatusCh, havePieceCh)


	cont := createTestController(crc)
	go cont.Run()

	//peer1Comms, peer1RequestPieceCh, peer1CancelPieceCh, peer1HavePieceCh := NewPeerComms("1.2.3.4:1234")
	peer1Comms, _, _, peer1HavePieceCh := NewPeerComms("1.2.3.4:1234")
	//peer1Comms, _, _, _ := NewPeerComms("1.2.3.4:1234")
	newPeerCh <- *peer1Comms

	innerChan := <- peer1HavePieceCh

	receivedBitField := make([]bool, len(cont.finishedPieces))
	for havePiece := range innerChan {
		receivedBitField[havePiece.pieceNum] = true
	}

	for pieceNum, havePiece := range receivedBitField {
		if cont.finishedPieces[pieceNum] != havePiece {
			t.Errorf("After receiving bitfield from controller, expected pieceNum %d to be %t but it was %t", pieceNum, cont.finishedPieces[pieceNum], havePiece)
		}
	}

	cont.Stop()
}

func TestControllerNewPeerReceiveFinishedBitfield(t *testing.T) {

}


