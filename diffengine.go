package main

import (
	"fmt"
	"sort"
)

// DiffType, DiffEntry, String() method remain the same

type DiffType int

const (
	Added DiffType = iota
	Deleted
	Modified
	Unchanged
)

type DiffEntry struct {
	Type       DiffType
	BlockA     *ContentBlock
	BlockB     *ContentBlock
	Similarity float32
}

func (dt DiffType) String() string {
	switch dt {
	case Added:
		return "ADDED"
	case Deleted:
		return "DELETED"
	case Modified:
		return "MODIFIED"
	case Unchanged:
		return "UNCHANGED"
	default:
		return "UNKNOWN"
	}
}

func PerformDiff(blocksA, blocksB []ContentBlock) []DiffEntry {
	// ... (map initializations, Stage 1: Exact Matches - no debug prints there yet) ...
	var diffs []DiffEntry

	mapA_checksumToBlock := make(map[string]*ContentBlock)
	mapB_checksumToBlock := make(map[string]*ContentBlock)
	mapA_idToBlock := make(map[int]*ContentBlock)
	mapB_idToBlock := make(map[int]*ContentBlock)

	for i := range blocksA {
		mapA_checksumToBlock[blocksA[i].Checksum] = &blocksA[i]
		mapA_idToBlock[blocksA[i].ID] = &blocksA[i]
	}
	for i := range blocksB {
		mapB_checksumToBlock[blocksB[i].Checksum] = &blocksB[i]
		mapB_idToBlock[blocksB[i].ID] = &blocksB[i]
	}

	processedA_byID := make(map[int]bool)
	processedB_byID := make(map[int]bool)

	// Stage 1: Exact Matches (Unchanged)
	for i := range blocksA {
		blockA_ptr := &blocksA[i]
		if blockB_ptr, ok := mapB_checksumToBlock[blockA_ptr.Checksum]; ok {
			if !processedB_byID[blockB_ptr.ID] {
				diffs = append(diffs, DiffEntry{Type: Unchanged, BlockA: blockA_ptr, BlockB: blockB_ptr})
				processedA_byID[blockA_ptr.ID] = true
				processedB_byID[blockB_ptr.ID] = true
			}
		}
	}

	// Stage 2: Semantic Matches (Modified)
	var unmatchedA_ptrs []*ContentBlock
	for i := range blocksA {
		if !processedA_byID[blocksA[i].ID] {
			unmatchedA_ptrs = append(unmatchedA_ptrs, &blocksA[i])
		}
	}
	sort.Slice(unmatchedA_ptrs, func(i, j int) bool {
		return unmatchedA_ptrs[i].ID < unmatchedA_ptrs[j].ID
	})

	for _, blockA_ptr := range unmatchedA_ptrs {
		bestMatchB_ptr := (*ContentBlock)(nil)
		highestSimilarity := float32(-1.0)

		if DebugMode { // Check the global DebugMode flag
			fmt.Printf("\n--- Checking A Block ID %d (L%d): \"%.60s...\"\n", blockA_ptr.ID, blockA_ptr.LineStart, blockA_ptr.NormalizedText)
		}

		for j := range blocksB {
			currentB_ptr := &blocksB[j]
			if processedB_byID[currentB_ptr.ID] {
				continue
			}
			similarity := TextSimilarityNormalized(blockA_ptr.NormalizedText, currentB_ptr.NormalizedText)

			if DebugMode { // Check the global DebugMode flag
				fmt.Printf("  Comparing with B Block ID %d (L%d): \"%.60s...\" -> Similarity: %.4f (Threshold: %.2f)\n",
					currentB_ptr.ID, currentB_ptr.LineStart, currentB_ptr.NormalizedText, similarity, SemanticSimilarityThreshold)
			}

			if similarity > highestSimilarity {
				highestSimilarity = similarity
				bestMatchB_ptr = currentB_ptr
			}
		}

		if bestMatchB_ptr != nil && highestSimilarity >= SemanticSimilarityThreshold {
			if DebugMode { // Check the global DebugMode flag
				fmt.Printf("  MATCHED A ID %d with B ID %d (Similarity: %.4f)\n", blockA_ptr.ID, bestMatchB_ptr.ID, highestSimilarity)
			}
			diffs = append(diffs, DiffEntry{
				Type:       Modified,
				BlockA:     blockA_ptr,
				BlockB:     bestMatchB_ptr,
				Similarity: highestSimilarity,
			})
			processedA_byID[blockA_ptr.ID] = true
			processedB_byID[bestMatchB_ptr.ID] = true
		} else {
			if DebugMode { // Check the global DebugMode flag
				bestBBlockID := -1
				if bestMatchB_ptr != nil {
					bestBBlockID = bestMatchB_ptr.ID
				}
				fmt.Printf("  NO MATCH for A ID %d (Highest sim: %.4f with B ID %d, Threshold: %.2f)\n", blockA_ptr.ID, highestSimilarity, bestBBlockID, SemanticSimilarityThreshold)
			}
		}
	}

	// ... (Stage 3: Added and Deleted Blocks, Sorting - no debug prints there) ...
	// Stage 3: Added and Deleted Blocks
	for i := range blocksA {
		if !processedA_byID[blocksA[i].ID] {
			diffs = append(diffs, DiffEntry{Type: Deleted, BlockA: &blocksA[i]})
		}
	}
	for i := range blocksB {
		if !processedB_byID[blocksB[i].ID] {
			diffs = append(diffs, DiffEntry{Type: Added, BlockB: &blocksB[i]})
		}
	}

	sort.Slice(diffs, func(i, j int) bool {
		if diffs[i].Type != diffs[j].Type {
			return diffs[i].Type < diffs[j].Type
		}
		if diffs[i].BlockA != nil && diffs[j].BlockA != nil {
			if diffs[i].BlockA.LineStart != diffs[j].BlockA.LineStart {
				return diffs[i].BlockA.LineStart < diffs[j].BlockA.LineStart
			}
			if diffs[i].BlockA.ID != diffs[j].BlockA.ID {
				return diffs[i].BlockA.ID < diffs[j].BlockA.ID
			}
		} else if diffs[i].BlockA != nil {
			return true
		} else if diffs[j].BlockA != nil {
			return false
		}

		if diffs[i].BlockB != nil && diffs[j].BlockB != nil {
			if diffs[i].BlockB.LineStart != diffs[j].BlockB.LineStart {
				return diffs[i].BlockB.LineStart < diffs[j].BlockB.LineStart
			}
			if diffs[i].BlockB.ID != diffs[j].BlockB.ID {
				return diffs[i].BlockB.ID < diffs[j].BlockB.ID
			}
		}
		return false
	})
	return diffs
}
