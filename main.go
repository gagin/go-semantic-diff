package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	// "regexp" // No longer needed here if paragraphSeparator is only in contentblock.go
)

// Global variables
var DebugMode bool
var SimilarityThreshold float64
var DetailsSections map[DiffType]bool

const MaxMovedSummariesCompact = 5
const MaxModifiedSummariesCompact = 3

func parseDetailsFlag(detailsStr string) map[DiffType]bool {
	sections := make(map[DiffType]bool)
	if detailsStr == "all" {
		sections[Added] = true
		sections[Deleted] = true
		sections[Modified] = true
		sections[Moved] = true
		sections[Unchanged] = true
		return sections
	}
	parts := strings.Split(detailsStr, ",")
	for _, part := range parts {
		trimmedPart := strings.ToLower(strings.TrimSpace(part))
		switch trimmedPart {
		case "new", "added":
			sections[Added] = true
		case "deleted":
			sections[Deleted] = true
		case "changed", "modified":
			sections[Modified] = true
		case "moved":
			sections[Moved] = true
		case "unchanged":
			sections[Unchanged] = true
		}
	}
	return sections
}

func main() {
	var detailsFlagStr string
	flag.BoolVar(&DebugMode, "debug", false, "Enable debug printing")
	flag.StringVar(&detailsFlagStr, "details", "new,deleted", "Comma-separated list of sections to show in detail (new,deleted,changed,moved,unchanged,all)")
	flag.Float64Var(&SimilarityThreshold, "threshold", 0.55, "Semantic similarity threshold (0.0 to 1.0)")
	flag.Parse()
	DetailsSections = parseDetailsFlag(detailsFlagStr)

	if flag.NArg() != 2 {
		fmt.Fprintln(os.Stderr, "Usage: go-semantic-diff [--debug] [--details <sections>] [--threshold <value>] <fileA> <fileB>")
		os.Exit(1)
	}
	fileAPath := flag.Arg(0)
	fileBPath := flag.Arg(1)
	if SimilarityThreshold < 0.0 || SimilarityThreshold > 1.0 {
		fmt.Fprintln(os.Stderr, "Error: threshold value must be between 0.0 and 1.0")
		os.Exit(1)
	}

	contentABytes, _ := ioutil.ReadFile(fileAPath)
	contentBBytes, _ := ioutil.ReadFile(fileBPath)
	contentA := string(contentABytes)
	contentB := string(contentBBytes)

	// Call SegmentFile from contentblock.go
	blocksA := SegmentFile(contentA, "A")
	blocksB := SegmentFile(contentB, "B")

	if DebugMode {
		fmt.Printf("File A ('%s') has %d blocks.\n", fileAPath, len(blocksA))
		fmt.Printf("File B ('%s') has %d blocks.\n", fileBPath, len(blocksB))
		// for i, blk := range blocksB { // DEBUG: Print IDs and first line of new blocks
		// 	if i < 25 { // Show more blocks for debugging segmentation
		// 		fmt.Printf("  B Block ID: %d, LineStart: %d, LineEnd: %d, Text: %.50s...\n", blk.ID, blk.LineStart, blk.LineEnd, strings.Split(blk.OriginalText, "\n")[0])
		// 	}
		// }
		fmt.Printf("Using Similarity Threshold: %.2f\n", SimilarityThreshold)
		fmt.Printf("Details sections: %s\n", detailsFlagStr)
		fmt.Println("--- Performing Diff (Debug Mode) ---")
	}

	diffResults := PerformDiff(blocksA, blocksB)
	if len(diffResults) == 0 {
		fmt.Println("Files are semantically identical at the block level.")
		return
	}

	groupedDiffs := make(map[DiffType][]DiffEntry)
	for _, entry := range diffResults {
		groupedDiffs[entry.Type] = append(groupedDiffs[entry.Type], entry)
	}
	outputOrder := []DiffType{Added, Deleted, Moved, Modified, Unchanged}

	for _, diffType := range outputOrder {
		entries, ok := groupedDiffs[diffType]
		if !ok || len(entries) == 0 {
			continue
		}
		sectionTitle := strings.ToUpper(entries[0].Type.String())
		if entries[0].Type == Unchanged && !DetailsSections[Unchanged] {
			sectionTitle = "UNCHANGED (IN PLACE)"
		} else if entries[0].Type == Unchanged && DetailsSections[Unchanged] {
			sectionTitle = "UNCHANGED_IN_PLACE"
		}
		fmt.Printf("\n# %s BLOCKS\n", sectionTitle)

		showDetailsForThisSection := DetailsSections[diffType]

		if !showDetailsForThisSection { /* ... Compact Output Logic ... */
			sectionKeyName := "unknown"
			switch diffType {
			case Added:
				sectionKeyName = "new"
			case Deleted:
				sectionKeyName = "deleted"
			case Modified:
				sectionKeyName = "changed"
			case Moved:
				sectionKeyName = "moved"
			case Unchanged:
				sectionKeyName = "unchanged"
			}
			switch diffType {
			case Added:
				totalLinesAdded := 0
				for _, e := range entries {
					totalLinesAdded += (e.BlockB.LineEnd - e.BlockB.LineStart + 1)
				}
				fmt.Printf("  Total: %d new blocks (approx %d lines in File B).\n", len(entries), totalLinesAdded)
			case Deleted:
				totalLinesDeleted := 0
				for _, e := range entries {
					totalLinesDeleted += (e.BlockA.LineEnd - e.BlockA.LineStart + 1)
				}
				fmt.Printf("  Total: %d deleted blocks (approx %d lines from File A).\n", len(entries), totalLinesDeleted)
			case Unchanged:
				fmt.Printf("  Total: %d blocks found to be unchanged and in the same relative order.\n", len(entries))
			case Moved:
				totalLinesMovedFileA := 0
				numMovedEntries := len(entries)
				for _, e := range entries {
					totalLinesMovedFileA += (e.BlockA.LineEnd - e.BlockA.LineStart + 1)
				}
				fmt.Printf("  Moved %d blocks (approx %d lines from File A):\n", numMovedEntries, totalLinesMovedFileA)
				limit := MaxMovedSummariesCompact
				if numMovedEntries < limit {
					limit = numMovedEntries
				}
				for i := 0; i < limit; i++ {
					e := entries[i]
					summary := fmt.Sprintf("A_ID:%d (L%d-%d) -> B_ID:%d (L%d-%d)", e.BlockA.ID, e.BlockA.LineStart, e.BlockA.LineEnd, e.BlockB.ID, e.BlockB.LineStart, e.BlockB.LineEnd)
					if e.Similarity > 0 && e.Similarity < 0.9999 {
						summary += fmt.Sprintf(" [Sim: %.2f]", e.Similarity)
					}
					fmt.Printf("    - %s\n", summary)
				}
				if numMovedEntries > MaxMovedSummariesCompact {
					fmt.Printf("    ... and %d more moved blocks.\n", numMovedEntries-MaxMovedSummariesCompact)
				}
			case Modified:
				fmt.Printf("  Total: %d blocks changed.\n", len(entries))
				limit := MaxModifiedSummariesCompact
				if len(entries) < limit {
					limit = len(entries)
				}
				for i := 0; i < limit; i++ {
					e := entries[i]
					fmt.Printf("    ~ A_ID:%d (L%d-%d) vs B_ID:%d (L%d-%d) (Sim: %.2f)\n", e.BlockA.ID, e.BlockA.LineStart, e.BlockA.LineEnd, e.BlockB.ID, e.BlockB.LineStart, e.BlockB.LineEnd, e.Similarity)
				}
				if len(entries) > limit {
					fmt.Printf("    ... and %d more changed blocks.\n", len(entries)-limit)
				}
			}
			if diffType == Modified && !showDetailsForThisSection {
				fmt.Printf("  (Use --details including '%s' to see content.)\n", sectionKeyName)
			} else if !showDetailsForThisSection {
				fmt.Printf("  (Use --details including '%s' to list them.)\n", sectionKeyName)
			}
			continue
		}

		// Detailed Output Logic (with Coalescing)
		i := 0
		for i < len(entries) {
			startEntry := entries[i]
			endEntry := entries[i]
			j := i + 1
			for j < len(entries) {
				nextEntry := entries[j]
				canCoalesce := false
				switch diffType {
				case Added:
					if endEntry.BlockB != nil && nextEntry.BlockB != nil && endEntry.BlockB.ID+1 == nextEntry.BlockB.ID {
						canCoalesce = true
					}
				case Deleted:
					if endEntry.BlockA != nil && nextEntry.BlockA != nil && endEntry.BlockA.ID+1 == nextEntry.BlockA.ID {
						canCoalesce = true
					}
				case Modified, Moved, Unchanged:
					if endEntry.BlockA != nil && nextEntry.BlockA != nil && endEntry.BlockA.ID+1 == nextEntry.BlockA.ID &&
						endEntry.BlockB != nil && nextEntry.BlockB != nil && endEntry.BlockB.ID+1 == nextEntry.BlockB.ID {
						canCoalesce = true
					}
				}
				if canCoalesce {
					endEntry = nextEntry
					j++
				} else {
					break
				}
			}
			var combinedTextA, combinedTextB strings.Builder
			for k := i; k < j; k++ {
				if entries[k].BlockA != nil {
					if combinedTextA.Len() > 0 {
						combinedTextA.WriteString("\n\n")
					}
					combinedTextA.WriteString(entries[k].BlockA.OriginalText)
				}
				if entries[k].BlockB != nil {
					if combinedTextB.Len() > 0 {
						combinedTextB.WriteString("\n\n")
					}
					combinedTextB.WriteString(entries[k].BlockB.OriginalText)
				}
			}
			switch diffType {
			case Added:
				fmt.Printf("  + File B Lines ~%d-%d (Orig IDs B:%d..%d):\n", startEntry.BlockB.LineStart, endEntry.BlockB.LineEnd, startEntry.BlockB.ID, endEntry.BlockB.ID)
				fmt.Printf("    \"%s\"\n", summarizedText(combinedTextB.String(), true))
			case Deleted:
				fmt.Printf("  - File A Lines ~%d-%d (Orig IDs A:%d..%d):\n", startEntry.BlockA.LineStart, endEntry.BlockA.LineEnd, startEntry.BlockA.ID, endEntry.BlockA.ID)
				fmt.Printf("    \"%s\"\n", summarizedText(combinedTextA.String(), true))
			case Modified:
				fmt.Printf("  ~ File A Lines ~%d-%d (Orig IDs A:%d..%d) changed to\n", startEntry.BlockA.LineStart, endEntry.BlockA.LineEnd, startEntry.BlockA.ID, endEntry.BlockA.ID)
				fmt.Printf("    \"%s\"\n", summarizedText(combinedTextA.String(), true))
				fmt.Printf("  ~ File B Lines ~%d-%d (Orig IDs B:%d..%d)\n", startEntry.BlockB.LineStart, endEntry.BlockB.LineEnd, startEntry.BlockB.ID, endEntry.BlockB.ID)
				fmt.Printf("    \"%s\"\n", summarizedText(combinedTextB.String(), true))
				fmt.Printf("    (Similarity of initial pair in sequence: %.2f)\n", startEntry.Similarity)
			case Moved:
				fmt.Printf("  M File A Lines ~%d-%d (Orig IDs A:%d..%d) moved to\n", startEntry.BlockA.LineStart, endEntry.BlockA.LineEnd, startEntry.BlockA.ID, endEntry.BlockA.ID)
				fmt.Printf("    Content (from A): \"%s\"\n", summarizedText(combinedTextA.String(), true))
				fmt.Printf("  M File B Lines ~%d-%d (Orig IDs B:%d..%d)\n", startEntry.BlockB.LineStart, endEntry.BlockB.LineEnd, startEntry.BlockB.ID, endEntry.BlockB.ID)
				if combinedTextA.String() != combinedTextB.String() && combinedTextB.Len() > 0 {
					fmt.Printf("    Content (from B): \"%s\"\n", summarizedText(combinedTextB.String(), true))
				}
				if startEntry.Similarity > 0 && startEntry.Similarity < 0.9999 {
					fmt.Printf("    (Note: Initial pair in sequence also modified, Similarity to B: %.2f)\n", startEntry.Similarity)
				}
			case Unchanged:
				fmt.Printf("  = File A Lines ~%d-%d (Orig IDs A:%d..%d) matches\n", startEntry.BlockA.LineStart, endEntry.BlockA.LineEnd, startEntry.BlockA.ID, endEntry.BlockA.ID)
				fmt.Printf("  = File B Lines ~%d-%d (Orig IDs B:%d..%d)\n", startEntry.BlockB.LineStart, endEntry.BlockB.LineEnd, startEntry.BlockB.ID, endEntry.BlockB.ID)
				fmt.Printf("    \"%s\"\n", summarizedText(combinedTextA.String(), true))
			}
			i = j
		}
	}
}

func summarizedText(text string, detailed bool) string {
	text = strings.ReplaceAll(text, "\n", "↵ ")
	maxLength := 60
	if detailed {
		maxLength = 80
	}
	runes := []rune(text)
	if len(runes) > maxLength {
		return string(runes[:maxLength-3]) + "..."
	}
	return text
}
