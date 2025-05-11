package main

import (
	"fmt"
	"sort"
)

// DiffType, DiffEntry, String() method, findLISIndices remain the same
// ... (ensure these are present as in the previous full version) ...
type DiffType int

const (
	Added DiffType = iota
	Deleted
	Modified
	Moved
	Unchanged
)

type DiffEntry struct {
	Type       DiffType
	BlockA     *ContentBlock
	BlockB     *ContentBlock
	Similarity float32
}

func (dt DiffType) String() string { /* ... */
	switch dt {
	case Added:
		return "NEW"
	case Deleted:
		return "DELETED"
	case Modified:
		return "CHANGED"
	case Moved:
		return "MOVED"
	case Unchanged:
		return "UNCHANGED_IN_PLACE"
	default:
		return "UNKNOWN"
	}
}
func findLISIndices(initialMatches []DiffEntry) []int {
	if len(initialMatches) == 0 {
		return []int{}
	}
	bLineStarts := make([]int, len(initialMatches))
	for i, match := range initialMatches {
		bLineStarts[i] = match.BlockB.LineStart
	}
	tails := make([]int, 0, len(bLineStarts))
	tailOriginalIndices := make([]int, 0, len(bLineStarts))
	predecessorMatchIndices := make([]int, len(bLineStarts))
	for i := range predecessorMatchIndices {
		predecessorMatchIndices[i] = -1
	}
	for i, bLine := range bLineStarts {
		j := sort.Search(len(tails), func(k int) bool { return tails[k] >= bLine })
		if j == len(tails) {
			tails = append(tails, bLine)
			tailOriginalIndices = append(tailOriginalIndices, i)
		} else {
			tails[j] = bLine
			tailOriginalIndices[j] = i
		}
		if j > 0 {
			predecessorMatchIndices[i] = tailOriginalIndices[j-1]
		}
	}
	if len(tailOriginalIndices) == 0 {
		return []int{}
	}
	lisResultIndices := make([]int, len(tails))
	currentIndexInInitialMatches := tailOriginalIndices[len(tails)-1]
	for i := len(tails) - 1; i >= 0; i-- {
		lisResultIndices[i] = currentIndexInInitialMatches
		currentIndexInInitialMatches = predecessorMatchIndices[currentIndexInInitialMatches]
		if currentIndexInInitialMatches == -1 && i > 0 {
			break
		}
	}
	return lisResultIndices
}

func PerformDiff(blocksA, blocksB []ContentBlock) []DiffEntry {
	var allPairedMatches []DiffEntry // Stores all BlockA-BlockB pairs (Unchanged or Modified)

	// --- Stage 0: Preparation ---
	// Maps for quick lookup by ID
	mapA_idToBlock := make(map[int]*ContentBlock)
	for i := range blocksA {
		mapA_idToBlock[blocksA[i].ID] = &blocksA[i]
	}
	mapB_idToBlock := make(map[int]*ContentBlock)
	for i := range blocksB {
		mapB_idToBlock[blocksB[i].ID] = &blocksB[i]
	}

	// Keep track of processed blocks to avoid re-matching
	processedA_byID := make(map[int]bool) // Tracks A blocks already paired (exact or semantic)
	processedB_byID := make(map[int]bool) // Tracks B blocks already paired

	// --- Stage 1: Global Exact Matches (Checksum based) ---
	// Build a checksum map for File B's blocks for efficient lookup
	mapB_checksumToAvailableBlockPointers := make(map[string][]*ContentBlock)
	for i := range blocksB {
		blockB_ptr := &blocksB[i]
		mapB_checksumToAvailableBlockPointers[blockB_ptr.Checksum] = append(mapB_checksumToAvailableBlockPointers[blockB_ptr.Checksum], blockB_ptr)
	}

	for i := range blocksA { // Iterate through File A blocks
		blockA_ptr := &blocksA[i]
		if potentialMatchingBs, ok := mapB_checksumToAvailableBlockPointers[blockA_ptr.Checksum]; ok {
			// Find the first available B block with this checksum
			foundB_idx := -1
			var blockB_match_ptr *ContentBlock
			for b_idx, b_ptr := range potentialMatchingBs {
				if !processedB_byID[b_ptr.ID] {
					blockB_match_ptr = b_ptr
					foundB_idx = b_idx
					break
				}
			}

			if blockB_match_ptr != nil {
				allPairedMatches = append(allPairedMatches, DiffEntry{Type: Unchanged, BlockA: blockA_ptr, BlockB: blockB_match_ptr})
				processedA_byID[blockA_ptr.ID] = true
				processedB_byID[blockB_match_ptr.ID] = true

				// Optional: To prevent reusing the exact same B block instance if multiple A blocks match it,
				// one could remove it from `potentialMatchingBs` or mark it used in a more granular way.
				// For simplicity, `processedB_byID` handles this broadly. If checksum collisions are rare for distinct blocks, this is fine.
				// If checksums are identical for truly distinct blocks (very rare for SHA256), this might "claim" a B block prematurely.
				// A slightly more robust way if checksums could collide for different blocks:
				if foundB_idx != -1 && len(potentialMatchingBs) > 1 {
					// This logic is tricky to get right without overcomplicating.
					// The current processedB_byID should suffice for most cases.
				}
			}
		}
	}
	if DebugMode {
		fmt.Printf("Global Exact Matches found: %d\n", len(allPairedMatches))
	}

	// --- Stage 2: Semantic Matches (for remaining blocks) ---
	var remainingA_ptrs []*ContentBlock
	for i := range blocksA {
		if !processedA_byID[blocksA[i].ID] {
			remainingA_ptrs = append(remainingA_ptrs, &blocksA[i])
		}
	}
	// Sort remainingA_ptrs to process consistently (though greedy matching can change order of pairing)
	sort.Slice(remainingA_ptrs, func(i, j int) bool { return remainingA_ptrs[i].ID < remainingA_ptrs[j].ID })

	// Collect remaining B blocks once
	var remainingB_ptrs []*ContentBlock
	for i := range blocksB {
		if !processedB_byID[blocksB[i].ID] {
			remainingB_ptrs = append(remainingB_ptrs, &blocksB[i])
		}
	}

	for _, blockA_ptr := range remainingA_ptrs {
		bestMatchB_ptr_for_A := (*ContentBlock)(nil)
		highestSimilarity_for_A := float32(-1.0)

		if DebugMode {
			fmt.Printf("\n--- Semantically Checking A Block ID %d (L%d): \"%.60s...\"\n", blockA_ptr.ID, blockA_ptr.LineStart, blockA_ptr.NormalizedText)
		}

		for _, currentB_ptr := range remainingB_ptrs { // Iterate only through remaining B blocks
			if processedB_byID[currentB_ptr.ID] { // Double check, though remainingB_ptrs should only have available ones
				continue
			}
			similarity := TextSimilarityNormalized(blockA_ptr.NormalizedText, currentB_ptr.NormalizedText)
			if DebugMode {
				fmt.Printf("  Comparing with B Block ID %d (L%d): \"%.60s...\" -> Similarity: %.4f (Threshold: %.2f)\n",
					currentB_ptr.ID, currentB_ptr.LineStart, currentB_ptr.NormalizedText, similarity, float32(SimilarityThreshold))
			}
			if similarity > highestSimilarity_for_A {
				highestSimilarity_for_A = similarity
				bestMatchB_ptr_for_A = currentB_ptr
			}
		}

		if bestMatchB_ptr_for_A != nil && highestSimilarity_for_A >= float32(SimilarityThreshold) {
			if DebugMode {
				fmt.Printf("  SEMANTIC MATCHED A ID %d with B ID %d (Similarity: %.4f)\n", blockA_ptr.ID, bestMatchB_ptr_for_A.ID, highestSimilarity_for_A)
			}
			allPairedMatches = append(allPairedMatches, DiffEntry{
				Type:       Modified, // Tentative type
				BlockA:     blockA_ptr,
				BlockB:     bestMatchB_ptr_for_A,
				Similarity: highestSimilarity_for_A,
			})
			processedA_byID[blockA_ptr.ID] = true
			processedB_byID[bestMatchB_ptr_for_A.ID] = true // This B block is now claimed
		} else if DebugMode {
			bestBBlockID := -1
			if bestMatchB_ptr_for_A != nil {
				bestBBlockID = bestMatchB_ptr_for_A.ID
			}
			fmt.Printf("  NO SEMANTIC MATCH for A ID %d (Highest sim: %.4f with B ID %d, Threshold: %.2f)\n", blockA_ptr.ID, highestSimilarity_for_A, bestBBlockID, float32(SimilarityThreshold))
		}
	}
	if DebugMode {
		fmt.Printf("Total Paired Matches (Exact + Semantic): %d\n", len(allPairedMatches))
	}

	// --- Stage 3: Identify MOVED blocks from allPairedMatches ---
	var finalDiffs []DiffEntry

	sort.Slice(allPairedMatches, func(i, j int) bool { // Sort by File A's block order
		if allPairedMatches[i].BlockA.LineStart != allPairedMatches[j].BlockA.LineStart {
			return allPairedMatches[i].BlockA.LineStart < allPairedMatches[j].BlockA.LineStart
		}
		return allPairedMatches[i].BlockA.ID < allPairedMatches[j].BlockA.ID
	})

	if len(allPairedMatches) > 0 {
		lisIndicesInPairedMatches := findLISIndices(allPairedMatches)
		isLisMember := make(map[int]bool)
		for _, originalIdx := range lisIndicesInPairedMatches {
			isLisMember[originalIdx] = true
		}

		for i, matchEntry := range allPairedMatches {
			if isLisMember[i] { // Part of LIS: Type is Unchanged or Modified
				finalDiffs = append(finalDiffs, matchEntry)
			} else { // Not part of LIS: Type is Moved
				movedEntry := matchEntry
				movedEntry.Type = Moved
				finalDiffs = append(finalDiffs, movedEntry)
			}
		}
	}

	// --- Stage 4: Add truly Added/Deleted blocks ---
	for i := range blocksA {
		if !processedA_byID[blocksA[i].ID] {
			finalDiffs = append(finalDiffs, DiffEntry{Type: Deleted, BlockA: &blocksA[i]})
		}
	}
	for i := range blocksB {
		if !processedB_byID[blocksB[i].ID] {
			finalDiffs = append(finalDiffs, DiffEntry{Type: Added, BlockB: &blocksB[i]})
		}
	}

	// --- Stage 5: Sort finalDiffs for output ---
	// ... (Sorting logic remains the same) ...
	sort.Slice(finalDiffs, func(i, j int) bool {
		if finalDiffs[i].Type != finalDiffs[j].Type {
			return finalDiffs[i].Type < finalDiffs[j].Type
		}
		if finalDiffs[i].BlockA != nil && finalDiffs[j].BlockA != nil {
			if finalDiffs[i].BlockA.LineStart != finalDiffs[j].BlockA.LineStart {
				return finalDiffs[i].BlockA.LineStart < finalDiffs[j].BlockA.LineStart
			}
			return finalDiffs[i].BlockA.ID < finalDiffs[j].BlockA.ID
		} else if finalDiffs[i].BlockA != nil {
			return true
		} else if finalDiffs[j].BlockA != nil {
			return false
		}
		if finalDiffs[i].BlockB != nil && finalDiffs[j].BlockB != nil {
			if finalDiffs[i].BlockB.LineStart != finalDiffs[j].BlockB.LineStart {
				return finalDiffs[i].BlockB.LineStart < finalDiffs[j].BlockB.LineStart
			}
			return finalDiffs[i].BlockB.ID < finalDiffs[j].BlockB.ID
		}
		return false
	})
	return finalDiffs
}
