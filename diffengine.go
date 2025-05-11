// diffengine.go
package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"
)

type DiffType int

const (
	Added DiffType = iota
	Deleted
	Modified
	Moved
	Unchanged
)

type LineDiffOp struct {
	Operation diffmatchpatch.Operation
	Text      string
}
type DiffEntry struct {
	Type       DiffType
	BlockA     *ContentBlock
	BlockB     *ContentBlock
	Similarity float32
	LineDiffs  []LineDiffOp
}

// String representation for DiffType (Stable)
func (dt DiffType) String() string {
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

// findLISIndices finds the Longest Increasing Subsequence of B-block start lines (Stable)
func findLISIndices(pairedMatches []DiffEntry) []int {
	if len(pairedMatches) == 0 {
		return []int{}
	}
	bLineStarts := make([]int, len(pairedMatches))
	for i, match := range pairedMatches {
		if match.BlockB == nil {
			// This case should ideally not happen if pairedMatches are valid
			// Or, handle by returning error or filtering out such matches upstream
			return []int{} // Or some other error indication
		}
		bLineStarts[i] = match.BlockB.LineStart
	}

	// Standard LIS algorithm
	tails := make([]int, 0, len(bLineStarts))
	tailOriginalIndices := make([]int, 0, len(bLineStarts)) // Stores original indices in pairedMatches for LIS construction
	predecessorMatchIndices := make([]int, len(pairedMatches))
	for i := range predecessorMatchIndices {
		predecessorMatchIndices[i] = -1 // -1 indicates no predecessor
	}

	for i, bLine := range bLineStarts {
		// Find insertion point `j` for `bLine` in `tails`
		j := sort.Search(len(tails), func(k int) bool { return tails[k] >= bLine })

		if j == len(tails) {
			// `bLine` is greater than all current tails, extend LIS
			tails = append(tails, bLine)
			tailOriginalIndices = append(tailOriginalIndices, i) // Store index from pairedMatches
		} else {
			// `bLine` replaces an existing tail
			tails[j] = bLine
			tailOriginalIndices[j] = i // Store index from pairedMatches
		}

		// Link predecessor for LIS reconstruction
		if j > 0 {
			predecessorMatchIndices[i] = tailOriginalIndices[j-1]
		}
	}

	if len(tailOriginalIndices) == 0 {
		return []int{}
	}

	// Reconstruct LIS from `tailOriginalIndices` and `predecessorMatchIndices`
	lisResultIndices := make([]int, len(tails))
	currentIndexInPairedMatches := tailOriginalIndices[len(tails)-1]
	for i := len(tails) - 1; i >= 0; i-- {
		lisResultIndices[i] = currentIndexInPairedMatches
		currentIndexInPairedMatches = predecessorMatchIndices[currentIndexInPairedMatches]
		if currentIndexInPairedMatches == -1 && i > 0 {
			// Should not happen in a correctly formed LIS if len(tails) > 0
			break
		}
	}
	return lisResultIndices
}

// getLinesWithInfo processes raw content into LineInfo objects (Stable)
func getLinesWithInfo(content string, fileOrigin string) []LineInfo {
	rawLines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	lineInfos := make([]LineInfo, len(rawLines))
	for i, lineText := range rawLines {
		lineInfos[i] = LineInfo{
			OriginalText:    lineText,
			TrimmedText:     strings.TrimSpace(lineText),
			Checksum:        CalculateLineChecksum(lineText), // Assuming CalculateLineChecksum exists
			OriginalLineNum: i + 1,
			FileOrigin:      fileOrigin,
			IsPartOfMega:    false,
		}
	}
	return lineInfos
}

const MinMegaBlockLength = 3
const MinParagraphLinesForSemanticMatch = 3 // Minimum lines for a gap paragraph to be considered for semantic matching

// findNextGreedyMegaMatch finds the next longest contiguous block of identical lines.
// Removed an empty 'if DebugMode {}' block.
func findNextGreedyMegaMatch(linesA, linesB []LineInfo) (aStart, bStart, length int, found bool) {
	bestLen := 0
	foundAStart, foundBStart := -1, -1

	for i := 0; i < len(linesA); i++ {
		if linesA[i].IsPartOfMega {
			continue
		}
		for j := 0; j < len(linesB); j++ {
			if linesB[j].IsPartOfMega {
				continue
			}
			if linesA[i].Checksum == linesB[j].Checksum {
				// Potential start of a match
				currentLen := 0
				for k := 0; i+k < len(linesA) && j+k < len(linesB); k++ {
					if linesA[i+k].IsPartOfMega || linesB[j+k].IsPartOfMega {
						break // One of the lines is already part of a megablock
					}
					if linesA[i+k].Checksum == linesB[j+k].Checksum {
						currentLen++
					} else {
						break // Mismatch
					}
				}
				if currentLen > bestLen {
					bestLen = currentLen
					foundAStart = i
					foundBStart = j
				}
			}
		}
	}

	if bestLen >= MinMegaBlockLength {
		return foundAStart, foundBStart, bestLen, true
	}
	return -1, -1, 0, false
}

// PerformDiff is the main diffing logic.
// Removed several empty 'if DebugMode {}' blocks for clarity.
// The 'NO SEMANTIC MATCH' debug prints remain correctly guarded by 'else if DebugMode'.
func PerformDiff(rawContentA string, rawContentB string) []DiffEntry {
	// Stage 1: Preprocessing - Get LineInfo for both files
	allLinesA := getLinesWithInfo(rawContentA, "A")
	allLinesB := getLinesWithInfo(rawContentB, "B")

	var megablockDiffs []DiffEntry
	blockGlobalIDCounter := 0 // Used to assign unique IDs to blocks as they are created

	// Stage 2: Greedy Megablock Matching
	for {
		aStart, bStart, length, found := findNextGreedyMegaMatch(allLinesA, allLinesB)
		if !found {
			break
		}

		// Create ContentBlocks for the megamatch
		var textAblockLines []string
		for k := 0; k < length; k++ {
			textAblockLines = append(textAblockLines, allLinesA[aStart+k].OriginalText)
		}
		blockAText := strings.Join(textAblockLines, "\n")
		cbA := ContentBlock{
			ID:             blockGlobalIDCounter,
			OriginalText:   blockAText,
			NormalizedText: NormalizeTextBlock(blockAText),
			Checksum:       CalculateBlockChecksum(blockAText),
			Embedding:      StubbedGetEmbedding(NormalizeTextBlock(blockAText)),
			LineStart:      allLinesA[aStart].OriginalLineNum,
			LineEnd:        allLinesA[aStart+length-1].OriginalLineNum,
			FileOrigin:     "A",
			SourceLineRefs: allLinesA[aStart : aStart+length], // Store involved lines
		}
		blockGlobalIDCounter++

		var textBblockLines []string
		for k := 0; k < length; k++ {
			textBblockLines = append(textBblockLines, allLinesB[bStart+k].OriginalText)
		}
		blockBText := strings.Join(textBblockLines, "\n")
		cbB := ContentBlock{
			ID:             blockGlobalIDCounter,
			OriginalText:   blockBText,
			NormalizedText: NormalizeTextBlock(blockBText),
			Checksum:       CalculateBlockChecksum(blockBText),
			Embedding:      StubbedGetEmbedding(NormalizeTextBlock(blockBText)),
			LineStart:      allLinesB[bStart].OriginalLineNum,
			LineEnd:        allLinesB[bStart+length-1].OriginalLineNum,
			FileOrigin:     "B",
			SourceLineRefs: allLinesB[bStart : bStart+length], // Store involved lines
		}
		blockGlobalIDCounter++

		megablockDiffs = append(megablockDiffs, DiffEntry{Type: Unchanged, BlockA: &cbA, BlockB: &cbB})

		// Mark lines as consumed by megablocks
		for k := 0; k < length; k++ {
			allLinesA[aStart+k].IsPartOfMega = true
			allLinesB[bStart+k].IsPartOfMega = true
		}
	}
	if DebugMode {
		fmt.Printf("Megablocks found: %d\n", len(megablockDiffs))
	}

	// Stage 3: Segment Gaps into Paragraphs
	var gapBlocksA, gapBlocksB []ContentBlock
	currentGapA := []LineInfo{}
	for i := 0; i < len(allLinesA); i++ {
		if !allLinesA[i].IsPartOfMega {
			currentGapA = append(currentGapA, allLinesA[i])
		} else {
			if len(currentGapA) > 0 {
				var segmented []ContentBlock
				segmented, blockGlobalIDCounter = SegmentGapText(currentGapA, "A", blockGlobalIDCounter)
				gapBlocksA = append(gapBlocksA, segmented...)
				currentGapA = []LineInfo{}
			}
		}
	}
	if len(currentGapA) > 0 { // Process any trailing gap
		var segmented []ContentBlock
		segmented, blockGlobalIDCounter = SegmentGapText(currentGapA, "A", blockGlobalIDCounter)
		gapBlocksA = append(gapBlocksA, segmented...)
	}

	currentGapB := []LineInfo{}
	for i := 0; i < len(allLinesB); i++ {
		if !allLinesB[i].IsPartOfMega {
			currentGapB = append(currentGapB, allLinesB[i])
		} else {
			if len(currentGapB) > 0 {
				var segmented []ContentBlock
				segmented, blockGlobalIDCounter = SegmentGapText(currentGapB, "B", blockGlobalIDCounter)
				gapBlocksB = append(gapBlocksB, segmented...)
				currentGapB = []LineInfo{}
			}
		}
	}
	if len(currentGapB) > 0 { // Process any trailing gap
		var segmented []ContentBlock
		segmented, blockGlobalIDCounter = SegmentGapText(currentGapB, "B", blockGlobalIDCounter)
		gapBlocksB = append(gapBlocksB, segmented...)
	}

	if DebugMode {
		fmt.Printf("Gap blocks in A: %d, Gap blocks in B: %d\n", len(gapBlocksA), len(gapBlocksB))
	}

	// Stage 4: Semantic Matching of Gap Paragraphs
	var semanticGapMatches []DiffEntry
	processedGapA_byID := make(map[int]bool) // Tracks Gap A blocks already matched
	processedGapB_byID := make(map[int]bool) // Tracks Gap B blocks already matched
	dmp := diffmatchpatch.New()

	// Sort gapBlocksA by ID to ensure deterministic processing if needed, though order of finding best match doesn't strictly require it.
	sort.Slice(gapBlocksA, func(i, j int) bool { return gapBlocksA[i].ID < gapBlocksA[j].ID })

	for i := range gapBlocksA {
		gapA_ptr := &gapBlocksA[i]
		// Skip very short paragraphs for semantic matching to reduce noise (already part of logic)
		numLinesInGapA := strings.Count(gapA_ptr.OriginalText, "\n") + 1
		if numLinesInGapA < MinParagraphLinesForSemanticMatch {
			continue
		}

		bestMatchGapB_ptr := (*ContentBlock)(nil)
		highestSimilarity := float32(-1.0)

		for j := range gapBlocksB {
			gapB_ptr := &gapBlocksB[j]
			if processedGapB_byID[gapB_ptr.ID] { // Already matched this B block
				continue
			}
			numLinesInGapB := strings.Count(gapB_ptr.OriginalText, "\n") + 1
			if numLinesInGapB < MinParagraphLinesForSemanticMatch {
				continue // Skip very short B paragraphs
			}

			similarity := TextSimilarityNormalized(gapA_ptr.NormalizedText, gapB_ptr.NormalizedText)
			if similarity > highestSimilarity {
				highestSimilarity = similarity
				bestMatchGapB_ptr = gapB_ptr
			}
		}

		if bestMatchGapB_ptr != nil && highestSimilarity >= float32(SimilarityThreshold) {
			entry := DiffEntry{Type: Modified, BlockA: gapA_ptr, BlockB: bestMatchGapB_ptr, Similarity: highestSimilarity}
			// Perform line-level diff for MODIFIED blocks
			diffsFromDMP := dmp.DiffMain(gapA_ptr.OriginalText, bestMatchGapB_ptr.OriginalText, true) // true for line mode
			dmp.DiffCleanupSemantic(diffsFromDMP)                                                     // Optional: clean up semantic noise
			var lineDiffs []LineDiffOp
			for _, d := range diffsFromDMP {
				lineDiffs = append(lineDiffs, LineDiffOp{Operation: d.Type, Text: d.Text})
			}
			entry.LineDiffs = lineDiffs
			semanticGapMatches = append(semanticGapMatches, entry)
			processedGapA_byID[gapA_ptr.ID] = true
			processedGapB_byID[bestMatchGapB_ptr.ID] = true
		} else if DebugMode {
			// This is the specific debug message block from user's output.
			// It is correctly guarded by 'DebugMode'.
			// If these messages appear without --debug, it implies the running code
			// differs from this version in this specific 'else if' condition.
			if bestMatchGapB_ptr != nil {
				fmt.Printf("  NO SEMANTIC MATCH for Gap A ID %d (Highest sim: %.4f with B ID %d, Thresh: %.2f)\n", gapA_ptr.ID, highestSimilarity, bestMatchGapB_ptr.ID, float32(SimilarityThreshold))
			} else {
				fmt.Printf("  NO SEMANTIC MATCH for Gap A ID %d (Highest sim: %.4f, No B candidate found, Thresh: %.2f)\n", gapA_ptr.ID, highestSimilarity, float32(SimilarityThreshold))
			}
		}
	}
	if DebugMode {
		fmt.Printf("Semantic matches between gap blocks: %d\n", len(semanticGapMatches))
	}

	// Stage 5: LIS for Positional Analysis (Moved vs. Unchanged/Modified-in-place)
	allPairedMatches := append([]DiffEntry{}, megablockDiffs...)
	allPairedMatches = append(allPairedMatches, semanticGapMatches...)

	// Sort allPairedMatches by BlockA's start line to prepare for LIS
	sort.Slice(allPairedMatches, func(i, j int) bool {
		if allPairedMatches[i].BlockA.LineStart != allPairedMatches[j].BlockA.LineStart {
			return allPairedMatches[i].BlockA.LineStart < allPairedMatches[j].BlockA.LineStart
		}
		return allPairedMatches[i].BlockA.ID < allPairedMatches[j].BlockA.ID // Tie-break by ID for stability
	})

	var finalDiffs []DiffEntry
	if len(allPairedMatches) > 0 {
		lisIndices := findLISIndices(allPairedMatches) // Get indices of matches that form LIS based on BlockB start lines
		isLisMember := make(map[int]bool)              // To quickly check if a match (by its index in allPairedMatches) is part of LIS
		for _, idx := range lisIndices {
			isLisMember[idx] = true
		}

		for i, matchEntry := range allPairedMatches {
			if isLisMember[i] {
				// Type remains Unchanged (for megablocks) or Modified (for semantic matches)
				finalDiffs = append(finalDiffs, matchEntry)
			} else {
				// Not in LIS, so it's MOVED
				movedEntry := matchEntry
				movedEntry.Type = Moved
				finalDiffs = append(finalDiffs, movedEntry)
			}
		}
	}

	// Stage 6: Identify Added/Deleted Gap Paragraphs
	for i := range gapBlocksA {
		if !processedGapA_byID[gapBlocksA[i].ID] { // If not part of megablock and not semantically matched
			finalDiffs = append(finalDiffs, DiffEntry{Type: Deleted, BlockA: &gapBlocksA[i]})
		}
	}
	for i := range gapBlocksB {
		if !processedGapB_byID[gapBlocksB[i].ID] { // If not part of megablock and not semantically matched
			finalDiffs = append(finalDiffs, DiffEntry{Type: Added, BlockB: &gapBlocksB[i]})
		}
	}

	// Stage 7: Sort finalDiffs for consistent output
	sort.Slice(finalDiffs, func(i, j int) bool {
		// Primary sort by DiffType
		if finalDiffs[i].Type != finalDiffs[j].Type {
			return finalDiffs[i].Type < finalDiffs[j].Type
		}
		// Secondary sort: by File A line (if available), then File B line
		if finalDiffs[i].BlockA != nil && finalDiffs[j].BlockA != nil {
			if finalDiffs[i].BlockA.LineStart != finalDiffs[j].BlockA.LineStart {
				return finalDiffs[i].BlockA.LineStart < finalDiffs[j].BlockA.LineStart
			}
			return finalDiffs[i].BlockA.ID < finalDiffs[j].BlockA.ID // Fallback to ID
		} else if finalDiffs[i].BlockA != nil { // A entries first
			return true
		} else if finalDiffs[j].BlockA != nil {
			return false
		}
		// For Added blocks (only BlockB exists)
		if finalDiffs[i].BlockB != nil && finalDiffs[j].BlockB != nil {
			if finalDiffs[i].BlockB.LineStart != finalDiffs[j].BlockB.LineStart {
				return finalDiffs[i].BlockB.LineStart < finalDiffs[j].BlockB.LineStart
			}
			return finalDiffs[i].BlockB.ID < finalDiffs[j].BlockB.ID // Fallback to ID
		}
		return false // Should not happen if blocks are well-formed
	})

	return finalDiffs
}
