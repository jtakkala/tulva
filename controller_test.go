package main

import (
	"testing"
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



