package main

import (
	"testing"
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

// AssertSliceContainsValue asserts that integer i is present in slice of integers s
func AssertSliceContainsValue(t *testing.T, s []int, i int) {
	found := false
	for _, v:= range s {
		if v == i {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected to find %d in %v, but not found", i, s)
	}
}

// Add a single pair and convert it to a rarity slice
func TestRarityMapOneValue(t *testing.T) {

	rm := NewRarityMap()
	rm.put(5, 3)
	rs := rm.getPiecesByRarity()

	if len(rs) != 1 {
		t.Errorf("Expected slice len to be %d but it was %d", 1, len(rs))
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

	AssertSliceContainsValue(t, rs, 3)
	AssertSliceContainsValue(t, rs, 2)

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

	AssertSliceContainsValue(t, rs[0:1], 2)
	AssertSliceContainsValue(t, rs[1:2], 3)
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

	AssertSliceContainsValue(t, rs[0:3], 6)
	AssertSliceContainsValue(t, rs[0:3], 8)
	AssertSliceContainsValue(t, rs[0:3], 10)
	AssertSliceContainsValue(t, rs[3:6], 3)
	AssertSliceContainsValue(t, rs[3:6], 7)
	AssertSliceContainsValue(t, rs[3:6], 9)
	AssertSliceContainsValue(t, rs[6:8], 2)
	AssertSliceContainsValue(t, rs[6:8], 4)
	AssertSliceContainsValue(t, rs[8:10], 1)
	AssertSliceContainsValue(t, rs[8:10], 5)

}

func createTestController() *Controller {
	// Initialize the slice of pieces that have been supposedly downloaded
	finishedPieces := []bool{true, false, false, false, false, false, false, false, false, true}
	pieceHashes := make([][]byte, len(finishedPieces))
	/*
	// Populate the pieceHashes with some dummy values
	for i := range pieceHashes {
		pieceHashes[i] = make([]byte, 1)
		pieceHashes[i][0] = byte(i)
	}
	*/

	// Create stubs and channels for DiskIO, PeerManager, and Peer
	diskIOStub := ControllerDiskIOChans{receivedPiece: make(chan ReceivedPiece)}
	peerManagerStub := ControllerPeerManagerChans{
		newPeer: make(chan PeerComms),
		deadPeer: make(chan string),
		seeding: make(chan bool),
	}
	peerStub := PeerControllerChans{
		chokeStatus: make(chan PeerChokeStatus),
		havePiece: make(chan chan HavePiece),
	}

	// Create the controller and return it
	return NewController(finishedPieces, pieceHashes, diskIOStub, peerManagerStub, peerStub)
}

func TestControllerRunStop(t *testing.T) {
	// Create a test controller and a wait channel. Start the controller in an
	// anonymous function. Attempt to stop the controller and then check if the
	// Run() method returns with select on the wait channel which should be closed.
	cont := createTestController()
	wait := make(chan struct{})
	go func() {
		cont.Run()
		close(wait)
	}()
	close(cont.quit)
	time.Sleep(10 * time.Millisecond)
	select {
	case <-wait:
		// PASS - Controller did shutdown
	default:
		t.Errorf("Wait channel did not close. Controller did not appear to shutdown.")
	}
}

// Confirm that the controller sends the new peer the bitfield of finished pieces
func TestControllerNewPeerReceiveFinishedBitfield(t *testing.T) {
	cont := createTestController()
	go cont.Run()

	peer1Comms := NewPeerComms("1.2.3.4:1234", *NewControllerPeerChans())

	cont.rxChans.peerManager.newPeer <- *peer1Comms

	// Since this is a new peer, we expect the controller to send the entire bitfield over the HavePiece
	// channel.
	// Emulate the peer by receiving the entire bitfield over the HavePiece chan from the controller
	innerChan := <-peer1Comms.chans.havePiece

	receivedBitField := make([]bool, len(cont.finishedPieces))
	for havePiece := range innerChan {
		receivedBitField[havePiece.pieceNum] = true
	}

	for pieceNum, havePiece := range receivedBitField {
		if cont.finishedPieces[pieceNum] != havePiece {
			t.Errorf("After receiving bitfield from controller, expected pieceNum %d to be %t but it was %t", pieceNum, cont.finishedPieces[pieceNum], havePiece)
		}
	}

	close(cont.quit)
}

// When a new peer comes online (when we're connected to no other peers) and we need
// a single piece from that peer, confirm that the controller doesn't ask it to get
// pieces that we don't need.
func TestControllerAskNewPeerToGetOnePiece(t *testing.T) {
	cont := createTestController()
	go cont.Run()

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
	time.Sleep(10 * time.Millisecond)

	// The peer is still choked (by default it starts choked). Confirm that the controller doesn't ask the
	// peer to request any pieces.
	select {
	case <-peer1Comms.chans.requestPiece:
		t.Errorf("The controller sent a peer a request before the peer was unchoked")

	default:
		// Pass. The peer didn't receive any piece request
	}

	// Tell the controller that the peer is now unchoked
	cont.rxChans.peer.chokeStatus <- PeerChokeStatus{peer1Name, false}

	// Sleep briefly to give the controller a chance to process the message
	time.Sleep(10 * time.Millisecond)

	// The controller should now tell the peer to retrieve only piece 1, since the controller
	// already has piece 0
	select {
	case request := <-peer1Comms.chans.requestPiece:
		if request.pieceNum != 1 {
			t.Errorf("Expected the controller to request piece number %d but received request for piece number %d", 1, request.pieceNum)
		}
	default:
		t.Errorf("We expected the controller to request piece number %d, but it wasn't received", 1)
	}

	// Sleep briefly to give the controller a chance to queue another message if it intends to
	time.Sleep(10 * time.Millisecond)

	select {
	case request := <-peer1Comms.chans.requestPiece:
		t.Errorf("The controller sent the peer a second requests for piece number %d, but we didn't expect any more requests", request.pieceNum)

	default:
		// Pass. We didn't receive any additional request over the channel
	}

	close(cont.quit)
}

func convertZeroOrMoreRequestsToBitfield(t *testing.T, requestChan chan RequestPiece, quantityOfPieces int) []bool {
	requestBitField := make([]bool, quantityOfPieces)

	for {
		time.Sleep(10 * time.Millisecond)
		select {
		case request := <-requestChan:
			requestBitField[request.pieceNum] = true
		default:
			// That was the last request
			return requestBitField
		}
	}
}

func assertActualBitfieldMatchesExpected(t *testing.T, expectedBitfield []bool, actualBitfield []bool) {
	for pieceNum, wasRequested := range actualBitfield {
		if expectedBitfield[pieceNum] == false && wasRequested == true {
			t.Errorf("Expected the controller to NOT send a request for pieceNum %d but was received", pieceNum)
		} else if expectedBitfield[pieceNum] == true && wasRequested == false {
			t.Errorf("Expected the controller send a request for pieceNum %d but it wasn't received", pieceNum)
		}
	}
}

// After a new peer comes online, it first signals that it's unchoked, then it sends its bitfield,
// instead of the other way around. Confirm that the controller requests for the peer to get pieces.
func TestControllerANewPeerSendsUnchokeBeforeBitfield(t *testing.T) {
	cont := createTestController()
	go cont.Run()

	peer1Name := "1.2.3.4:1234"
	peer1Comms := NewPeerComms(peer1Name, *NewControllerPeerChans())

	cont.rxChans.peerManager.newPeer <- *peer1Comms

	cont.rxChans.peer.chokeStatus <- PeerChokeStatus{peer1Name, false}

	time.Sleep(10 * time.Millisecond)

	peer1Bitfield := []bool{true, true, false, false, false, false, false, false, false, false}
	sendBitfieldOverChannel(cont.rxChans.peer.havePiece, peer1Name, peer1Bitfield)

	// Sleep briefly to give the controller a chance to process the bitfield
	time.Sleep(10 * time.Millisecond)

	requestsFromController := convertZeroOrMoreRequestsToBitfield(t, peer1Comms.chans.requestPiece, len(peer1Bitfield))

	expectedRequestsBitfield := []bool{false, true, false, false, false, false, false, false, false, false, false}

	assertActualBitfieldMatchesExpected(t, expectedRequestsBitfield, requestsFromController)

	close(cont.quit)
}

// When a new peer comes online (when we're connected to no other peers) and we need
// several pieces from that peer, confirm that the controller only asks it for the
// pieces that we need, but no more than maxSimultaneousDownloads
func TestControllerNewPeerWithSeveralPiecesThatWeNeed(t *testing.T) {
	cont := createTestController()
	go cont.Run()

	peer1Name := "1.2.3.4:1234"
	peer1Comms := NewPeerComms(peer1Name, *NewControllerPeerChans())

	cont.rxChans.peerManager.newPeer <- *peer1Comms

	// peer1 has pieces 0, 1, 3, 4 and 8
	peer1Bitfield := []bool{true, true, false, true, true, false, false, false, true, false}
	sendBitfieldOverChannel(cont.rxChans.peer.havePiece, peer1Name, peer1Bitfield)

	time.Sleep(10 * time.Millisecond)

	cont.rxChans.peer.chokeStatus <- PeerChokeStatus{peer1Name, false}

	// The controller should now tell the peer to retrieve pieces 1, 3, 4, 5, 6 but not 8
	// because there are a max of 5 simultaneous downloads

	requestsFromController := convertZeroOrMoreRequestsToBitfield(t, peer1Comms.chans.requestPiece, len(peer1Bitfield))

	expectedRequestsBitfield := []bool{false, true, false, true, true, false, false, false, true, false}

	assertActualBitfieldMatchesExpected(t, expectedRequestsBitfield, requestsFromController)

	close(cont.quit)
}

// Switch to unchoked, then back to choked, then back to unchoked. When the peer switches to
// unchoked for the second time, expect that the controller will tell it again to download
// the same pieces.
func TestControllerPeerSwitchesBetweenUnchokedAndChokedRepeatedly(t *testing.T) {
	cont := createTestController()
	go cont.Run()

	peer1Name := "1.2.3.4:1234"
	peer1Comms := NewPeerComms(peer1Name, *NewControllerPeerChans())

	cont.rxChans.peerManager.newPeer <- *peer1Comms

	// peer1 has pieces 0, 1, 3, 4 and 8
	peer1Bitfield := []bool{true, true, false, true, true, false, false, false, true, false}
	sendBitfieldOverChannel(cont.rxChans.peer.havePiece, peer1Name, peer1Bitfield)

	// Signal that the peer is unchoked
	cont.rxChans.peer.chokeStatus <- PeerChokeStatus{peer1Name, false}

	requestsFromController := convertZeroOrMoreRequestsToBitfield(t, peer1Comms.chans.requestPiece, len(peer1Bitfield))
	expectedRequestsBitfield := []bool{false, true, false, true, true, false, false, false, true, false}
	assertActualBitfieldMatchesExpected(t, expectedRequestsBitfield, requestsFromController)

	// Signal that the peer is choked
	cont.rxChans.peer.chokeStatus <- PeerChokeStatus{peer1Name, true}

	// Pause briefly to give the controller time to process the choke.
	time.Sleep(10 * time.Millisecond)

	// Confirm that the peer is not told to request anything after switching to a choked state.
	select {
	case <-peer1Comms.chans.requestPiece:
		t.Errorf("The controller shouldn't have sent any requests after the peer switched to a choked state")
	default:
		// Pass. Nothing was received.
	}

	// Signal that the peer is unchoked
	cont.rxChans.peer.chokeStatus <- PeerChokeStatus{peer1Name, false}

	// The same requests should be re-sent by the controller to the peer
	requestsFromController = convertZeroOrMoreRequestsToBitfield(t, peer1Comms.chans.requestPiece, len(peer1Bitfield))
	assertActualBitfieldMatchesExpected(t, expectedRequestsBitfield, requestsFromController)

	close(cont.quit)
}

func assertCancelReceived(t *testing.T, cancelCh chan CancelPiece, expectedPieceNum int) {

	time.Sleep(10 * time.Millisecond)
	select {
	case message := <-cancelCh:
		if message.pieceNum != expectedPieceNum {
			t.Errorf("Expected to receive a cancel message or piece %d but it was %d", expectedPieceNum, message.pieceNum)
		} else {
			// Pass. We received the cancel for the piece that was expected.
		}

	default:
		t.Errorf("Expected to receive a cancel message for piece %d but it wasn't received", expectedPieceNum)
	}
}

func assertRequestOrder(t *testing.T, peerComms *PeerComms, expectedRequests []int) {

	// For every request received from the controller, tell it that we finished the piece, then wait
	// for more requests in the request channel.
	for _, expectedPieceNum := range expectedRequests {
		time.Sleep(10 * time.Millisecond)
		select {
		case request := <-peerComms.chans.requestPiece:

			// Confirm that the piece that it was just told to get it what was expected
			if request.pieceNum != expectedPieceNum {
				t.Errorf("Expected %s to be told to get pieceNum %d next, but it was told to get pieceNum %d instead", peerComms.peerName, expectedPieceNum, request.pieceNum)
			}

		default:
			t.Errorf("%s wasn't told to get any more pieces, but it was expected to be told to get pieceNum %d", peerComms.peerName, expectedPieceNum)
		}
	}
}

// Confirm that download priority works as expected with multiple peers. In this case, there are
// four peers who each have different (and somewhat overlapping) sets of pieces.
func TestControllerDownloadPriorityForFourPeers(t *testing.T) {
	cont := createTestController()

	// Override the maxSimultaneousDownloadsPerPeer value to confirm the order of downloadPriority
	// cont.maxSimultaneousDownloadsPerPeer = 1

	go cont.Run()

	peer1Name := "1.2.3.4:1234"
	peer1Comms := NewPeerComms(peer1Name, *NewControllerPeerChans())

	peer2Name := "2.3.4.5:2345"
	peer2Comms := NewPeerComms(peer2Name, *NewControllerPeerChans())

	peer3Name := "3.4.5.6:3456"
	peer3Comms := NewPeerComms(peer3Name, *NewControllerPeerChans())

	peer4Name := "4.5.6.7:4567"
	peer4Comms := NewPeerComms(peer4Name, *NewControllerPeerChans())

	cont.rxChans.peerManager.newPeer <- *peer1Comms
	cont.rxChans.peerManager.newPeer <- *peer2Comms
	cont.rxChans.peerManager.newPeer <- *peer3Comms
	cont.rxChans.peerManager.newPeer <- *peer4Comms

	// peer1 has pieces 0, 1, 3, 4, 8
	peer1Bitfield := []bool{true, true, false, true, true, false, false, false, true, false}
	sendBitfieldOverChannel(cont.rxChans.peer.havePiece, peer1Name, peer1Bitfield)

	// peer2 has pieces 0, 2, 3, 4, 6
	peer2Bitfield := []bool{true, false, true, true, true, false, true, false, false, false}
	sendBitfieldOverChannel(cont.rxChans.peer.havePiece, peer2Name, peer2Bitfield)

	// peer3 has pieces 0, 1, 4
	peer3Bitfield := []bool{true, true, false, false, true, false, false, false, false, false}
	sendBitfieldOverChannel(cont.rxChans.peer.havePiece, peer3Name, peer3Bitfield)

	// peer3 has all 10 pieces
	peer4Bitfield := []bool{true, true, true, true, true, true, true, true, true, true}
	sendBitfieldOverChannel(cont.rxChans.peer.havePiece, peer4Name, peer4Bitfield)

	// At this point no peers would have been told to retrieve and pieces, because they're all
	// still choked.

	time.Sleep(10 * time.Millisecond)

	peer1ExpectedRequests := []int{3, 8, 1, 4} // Since peer2 isn't considered yet, piece 3 has rarity of 2 (not 3)
	peer2ExpectedRequests := []int{2, 6, 3, 4}
	peer3ExpectedRequests := []int{1, 4}
	// Note that peer4 won't be asked to get more than 5 pieces because of maxSimultaneousDownloadsPerPeer
	peer4ExpectedRequests := []int{5, 7, 2, 6, 8}

	// Send the unchoke message for peer3 first because he has the smallest list of needed pieces
	cont.rxChans.peer.chokeStatus <- PeerChokeStatus{peer3Name, false}
	cont.rxChans.peer.chokeStatus <- PeerChokeStatus{peer1Name, false}
	cont.rxChans.peer.chokeStatus <- PeerChokeStatus{peer2Name, false}
	cont.rxChans.peer.chokeStatus <- PeerChokeStatus{peer4Name, false}

	assertRequestOrder(t, peer1Comms, peer1ExpectedRequests)
	assertRequestOrder(t, peer2Comms, peer2ExpectedRequests)
	assertRequestOrder(t, peer3Comms, peer3ExpectedRequests)
	assertRequestOrder(t, peer4Comms, peer4ExpectedRequests)

	close(cont.quit)
}

// Two peers. Both Are working on the same piece. One finishes, so the other should be told to CANCEL.
func TestControllerTwoPeersDownloadingSamePieceAndOneFinishes(t *testing.T) {
	cont := createTestController()
	go cont.Run()

	peer1Name := "1.2.3.4:1234"
	peer1Comms := NewPeerComms(peer1Name, *NewControllerPeerChans())

	peer2Name := "4.2.2.2:53"
	peer2Comms := NewPeerComms(peer2Name, *NewControllerPeerChans())

	cont.rxChans.peerManager.newPeer <- *peer1Comms
	cont.rxChans.peerManager.newPeer <- *peer2Comms

	// unchoke both peers
	cont.rxChans.peer.chokeStatus <- PeerChokeStatus{peer1Name, false}
	cont.rxChans.peer.chokeStatus <- PeerChokeStatus{peer2Name, false}

	time.Sleep(10 * time.Millisecond)

	// peer1 only has piece 1
	peer1Bitfield := []bool{false, true, false, false, false, false, false, false, false, false}
	sendBitfieldOverChannel(cont.rxChans.peer.havePiece, peer1Name, peer1Bitfield)

	// peer2 also only has piece 1
	peer2Bitfield := []bool{false, true, false, false, false, false, false, false, false, false}
	sendBitfieldOverChannel(cont.rxChans.peer.havePiece, peer2Name, peer2Bitfield)

	// Controller will now tell both peers to download piece 1. Sleep
	// briefly to give the controller time to issue requests to both peers
	time.Sleep(10 * time.Millisecond)

	// Inform controller that piece1 was finished by peer1
	cont.rxChans.diskIO.receivedPiece <- ReceivedPiece{1, peer1Name}

	// Confirm that peer2 is told to cancel piece number 1
	assertCancelReceived(t, peer2Comms.chans.cancelPiece, 1)

	// Confirm that peer1 is not told to cancel
	select {
	case message := <-peer1Comms.chans.cancelPiece:
		t.Errorf("Peer1 was told to cancel piece %d, but this shouldn't have happened.", message.pieceNum)
	default:
		// PASS. Peer1 was not told to cancel any piece.
	}

	close(cont.quit)
}

// sliceToSet takes a slice of integers and returns the values in the slice as a set
func sliceToSet(numbers []int) map[int]struct{} {
	set := make(map[int]struct{})
	for _, v := range numbers {
		set[v] = struct{}{}
	}
	return set
}

func TestShuffle(t *testing.T) {
	// initialize a slice of numbers and also copy them to a set
	numbers := make([]int, 100)
	for i := 0; i < len(numbers); i++ {
		numbers[i] = i
	}
	oldSet := sliceToSet(numbers)
	// shuffle the numbers in-place and return a new set
	shuffle(numbers)
	newSet := sliceToSet(numbers)
	// assert that the lengths of the sets are equal
	if len(oldSet) != len(newSet) {
		t.Errorf("Expected shuffled set length to equal %d, but length is %d", len(oldSet), len(newSet))
	}
	// assert that all numbers in the original set are present in the new set
	for k, _ := range oldSet {
		_, ok := newSet[k]
		if !ok {
			t.Errorf("Expected to find '%d' in new set, but did not", k)
		}
	}
	//
	for i := 0; ; i++ {
		if i >= len(numbers) {
			// there is a tiny but unlikely possibility that the slice was shuffled and no numbers changed position
			t.Errorf("Reached end of slice and slice does not appear to have been shuffled")
			break
		}
		if numbers[i] != i {
			// slice appears to have been shuffled
			break
		}
	}
}
