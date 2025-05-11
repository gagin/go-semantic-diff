package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"
)

var DebugMode bool
var SimilarityThreshold float64
var DetailsSections map[DiffType]bool
var FocusRangeStr string

const MaxMovedSummariesCompact = 5
const MaxModifiedSummariesCompact = 3

type FocusRange struct {
	StartLine, EndLine int
	IsSet              bool
}

var CurrentFocusRange FocusRange

// parseDetailsFlag is stable
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

// parseFocusRange is stable
func parseFocusRange(focusStr string) FocusRange {
	if focusStr == "" {
		return FocusRange{IsSet: false}
	}
	parts := strings.Split(focusStr, ",")
	if len(parts) != 2 {
		fmt.Fprintf(os.Stderr, "Error: --focus flag expects n,m (e.g., --focus 10,20). Got: %s\n", focusStr)
		return FocusRange{IsSet: false, StartLine: -1}
	}
	start, errS := strconv.Atoi(strings.TrimSpace(parts[0]))
	end, errE := strconv.Atoi(strings.TrimSpace(parts[1]))
	if errS != nil || errE != nil || start <= 0 || end < start {
		fmt.Fprintf(os.Stderr, "Error: Invalid line numbers for --focus. Expects positive integers n,m with n <= m. Got: %s\n", focusStr)
		return FocusRange{IsSet: false, StartLine: -1}
	}
	return FocusRange{StartLine: start, EndLine: end, IsSet: true}
}

func main() {
	var detailsFlagStr string
	flag.BoolVar(&DebugMode, "debug", false, "Enable debug printing")
	flag.StringVar(&detailsFlagStr, "details", "new,deleted", "Comma-separated list of sections to show in detail (new,deleted,changed,moved,unchanged,all)")
	flag.Float64Var(&SimilarityThreshold, "threshold", 0.55, "Semantic similarity threshold (0.0 to 1.0)")
	flag.StringVar(&FocusRangeStr, "focus", "", "Report on lines n,m from File A (e.g., --focus 10,20)")
	flag.Parse()
	DetailsSections = parseDetailsFlag(detailsFlagStr)
	CurrentFocusRange = parseFocusRange(FocusRangeStr)
	if CurrentFocusRange.IsSet && CurrentFocusRange.StartLine == -1 {
		os.Exit(1)
	}

	if flag.NArg() != 2 {
		fmt.Fprintln(os.Stderr, "Usage: go-semantic-diff [--debug] [--details <sections>] [--threshold <value>] [--focus n,m] <fileA> <fileB>")
		os.Exit(1)
	}
	fileAPath := flag.Arg(0)
	fileBPath := flag.Arg(1)
	if SimilarityThreshold < 0.0 || SimilarityThreshold > 1.0 {
		fmt.Fprintln(os.Stderr, "Error: threshold value must be between 0.0 and 1.0")
		os.Exit(1)
	}

	contentABytes, errA := ioutil.ReadFile(fileAPath)
	if errA != nil {
		fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", fileAPath, errA)
		os.Exit(1)
	}
	contentBBytes, errB := ioutil.ReadFile(fileBPath)
	if errB != nil {
		fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", fileBPath, errB)
		os.Exit(1)
	}
	rawContentA := string(contentABytes)
	rawContentB := string(contentBBytes)

	// Debug prints and initial setup are stable
	if DebugMode {
		fmt.Printf("File A ('%s') has %d lines.\n", fileAPath, strings.Count(rawContentA, "\n")+1)
		fmt.Printf("File B ('%s') has %d lines.\n", fileBPath, strings.Count(rawContentB, "\n")+1)
		fmt.Printf("Using Similarity Threshold: %.2f\n", SimilarityThreshold)
		fmt.Printf("Details sections: %s\n", detailsFlagStr)
		if CurrentFocusRange.IsSet {
			fmt.Printf("Focus range for File A: Lines %d-%d\n", CurrentFocusRange.StartLine, CurrentFocusRange.EndLine)
		}
		fmt.Println("--- Performing Diff (Debug Mode) ---")
	}

	diffResults := PerformDiff(rawContentA, rawContentB)
	if CurrentFocusRange.IsSet {
		printFocusResults(rawContentA, diffResults, CurrentFocusRange)
		return
	}
	if len(diffResults) == 0 {
		fmt.Println("Files are semantically identical at the block level.")
		return
	}

	groupedDiffs := make(map[DiffType][]DiffEntry)
	for _, entry := range diffResults {
		groupedDiffs[entry.Type] = append(groupedDiffs[entry.Type], entry)
	}
	outputOrder := []DiffType{Added, Deleted, Moved, Modified, Unchanged}

	// This loop processes and prints each diff type section.
	// The main change is within the `showDetailsForThisSection` block.
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

		if !showDetailsForThisSection { // Compact Output Logic (Stable)
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
					if e.BlockB != nil {
						totalLinesAdded += (e.BlockB.LineEnd - e.BlockB.LineStart + 1)
					}
				}
				fmt.Printf("  Total: %d new blocks (approx %d lines in File B).\n", len(entries), totalLinesAdded)
			case Deleted:
				totalLinesDeleted := 0
				for _, e := range entries {
					if e.BlockA != nil {
						totalLinesDeleted += (e.BlockA.LineEnd - e.BlockA.LineStart + 1)
					}
				}
				fmt.Printf("  Total: %d deleted blocks (approx %d lines from File A).\n", len(entries), totalLinesDeleted)
			case Unchanged:
				fmt.Printf("  Total: %d blocks found to be unchanged and in the same relative order.\n", len(entries))
			case Moved:
				totalLinesMovedFileA := 0
				numMovedEntries := len(entries)
				for _, e := range entries {
					if e.BlockA != nil {
						totalLinesMovedFileA += (e.BlockA.LineEnd - e.BlockA.LineStart + 1)
					}
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
		// Detailed Output Logic (with Coalescing & LineDiffs for Modified)
		// This section is modified to use line adjacency for coalescing.
		i := 0
		for i < len(entries) {
			startEntry := entries[i]

			currentCoalescedStartA, currentCoalescedEndA := 0, 0
			if startEntry.BlockA != nil {
				currentCoalescedStartA = startEntry.BlockA.LineStart
				currentCoalescedEndA = startEntry.BlockA.LineEnd
			}
			currentCoalescedStartB, currentCoalescedEndB := 0, 0
			if startEntry.BlockB != nil {
				currentCoalescedStartB = startEntry.BlockB.LineStart
				currentCoalescedEndB = startEntry.BlockB.LineEnd
			}

			var combinedTextA, combinedTextB strings.Builder
			if startEntry.BlockA != nil {
				combinedTextA.WriteString(startEntry.BlockA.OriginalText)
			}
			if startEntry.BlockB != nil {
				combinedTextB.WriteString(startEntry.BlockB.OriginalText)
			}

			firstBlockInCoalescedGroup := startEntry

			j := i + 1
			for j < len(entries) {
				nextEntry := entries[j]
				canCoalesce := false
				const maxGapForCoalesce = 1

				switch diffType {
				case Added:
					if startEntry.BlockB != nil && nextEntry.BlockB != nil &&
						nextEntry.BlockB.LineStart <= currentCoalescedEndB+1+maxGapForCoalesce {
						canCoalesce = true
					}
				case Deleted:
					if startEntry.BlockA != nil && nextEntry.BlockA != nil &&
						nextEntry.BlockA.LineStart <= currentCoalescedEndA+1+maxGapForCoalesce {
						canCoalesce = true
					}
				case Modified, Moved, Unchanged:
					if startEntry.BlockA != nil && nextEntry.BlockA != nil &&
						(nextEntry.BlockA.LineStart <= currentCoalescedEndA+1+maxGapForCoalesce) {
						if startEntry.BlockB != nil && nextEntry.BlockB != nil &&
							(nextEntry.BlockB.LineStart <= currentCoalescedEndB+1+maxGapForCoalesce) {
							canCoalesce = true
						}
					}
				}

				if canCoalesce {
					if nextEntry.BlockA != nil {
						if combinedTextA.Len() > 0 {
							if nextEntry.BlockA.LineStart > currentCoalescedEndA+1 {
								for k := 0; k < (nextEntry.BlockA.LineStart - (currentCoalescedEndA + 1)); k++ {
									combinedTextA.WriteString("\n")
								}
							} else {
								combinedTextA.WriteString("\n\n")
							}
						}
						combinedTextA.WriteString(nextEntry.BlockA.OriginalText)
						currentCoalescedEndA = max(currentCoalescedEndA, nextEntry.BlockA.LineEnd)
					}
					if nextEntry.BlockB != nil {
						if combinedTextB.Len() > 0 {
							if nextEntry.BlockB.LineStart > currentCoalescedEndB+1 {
								for k := 0; k < (nextEntry.BlockB.LineStart - (currentCoalescedEndB + 1)); k++ {
									combinedTextB.WriteString("\n")
								}
							} else {
								combinedTextB.WriteString("\n\n")
							}
						}
						combinedTextB.WriteString(nextEntry.BlockB.OriginalText)
						currentCoalescedEndB = max(currentCoalescedEndB, nextEntry.BlockB.LineEnd)
					}
					j++
				} else {
					break
				}
			}

			switch diffType {
			case Added:
				fmt.Printf("  + File B Lines ~%d-%d:\n", currentCoalescedStartB, currentCoalescedEndB)
				fmt.Printf("    \"%s\"\n", summarizedText(combinedTextB.String(), true))
			case Deleted:
				fmt.Printf("  - File A Lines ~%d-%d:\n", currentCoalescedStartA, currentCoalescedEndA)
				fmt.Printf("    \"%s\"\n", summarizedText(combinedTextA.String(), true))
			case Modified:
				fmt.Printf("  ~ File A Lines ~%d-%d vs File B Lines ~%d-%d\n", currentCoalescedStartA, currentCoalescedEndA, currentCoalescedStartB, currentCoalescedEndB)
				fmt.Printf("    (Overall Block Similarity: %.2f)\n", firstBlockInCoalescedGroup.Similarity)
				if len(firstBlockInCoalescedGroup.LineDiffs) > 0 && (j-i == 1) {
					fmt.Println("    Line-level changes (for first block in sequence):")
					for _, op := range firstBlockInCoalescedGroup.LineDiffs {
						opTextLines := strings.Split(strings.TrimSuffix(op.Text, "\n"), "\n")
						for _, opLine := range opTextLines {
							if strings.TrimSpace(opLine) == "" && op.Operation == diffmatchpatch.DiffEqual {
								continue
							}
							prefix := "      "
							switch op.Operation {
							case diffmatchpatch.DiffInsert:
								prefix += "+ "
							case diffmatchpatch.DiffDelete:
								prefix += "- "
							case diffmatchpatch.DiffEqual:
								prefix += "  "
							}
							fmt.Printf("%s%s\n", prefix, opLine)
						}
					}
				} else {
					fmt.Printf("    Block A Content: \"%s\"\n", summarizedText(combinedTextA.String(), true))
					fmt.Printf("    Block B Content: \"%s\"\n", summarizedText(combinedTextB.String(), true))
				}
			case Moved:
				fmt.Printf("  M File A Lines ~%d-%d moved to\n", currentCoalescedStartA, currentCoalescedEndA)
				fmt.Printf("    Content (from A): \"%s\"\n", summarizedText(combinedTextA.String(), true))
				fmt.Printf("  M File B Lines ~%d-%d\n", currentCoalescedStartB, currentCoalescedEndB)
				if combinedTextA.String() != combinedTextB.String() && combinedTextB.Len() > 0 {
					fmt.Printf("    Content (from B, if different): \"%s\"\n", summarizedText(combinedTextB.String(), true))
				}
				if firstBlockInCoalescedGroup.Similarity > 0 && firstBlockInCoalescedGroup.Similarity < 0.9999 {
					fmt.Printf("    (Note: Initial pair in sequence may also be modified, Similarity to B: %.2f)\n", firstBlockInCoalescedGroup.Similarity)
				}
			case Unchanged:
				fmt.Printf("  = File A Lines ~%d-%d matches\n", currentCoalescedStartA, currentCoalescedEndA)
				fmt.Printf("  = File B Lines ~%d-%d\n", currentCoalescedStartB, currentCoalescedEndB)
				fmt.Printf("    \"%s\"\n", summarizedText(combinedTextA.String(), true))
			}
			i = j
		}
	}
}

// summarizedText is stable
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

// printFocusResults is stable
func printFocusResults(rawFileAContent string, diffs []DiffEntry, focus FocusRange) {
	fmt.Printf("\n--- Focus on File A Lines %d-%d ---\n", focus.StartLine, focus.EndLine)
	fileALines := strings.Split(strings.ReplaceAll(rawFileAContent, "\r\n", "\n"), "\n")

	sortedDiffsA := make([]DiffEntry, 0)
	for _, d := range diffs {
		if d.BlockA != nil {
			sortedDiffsA = append(sortedDiffsA, d)
		}
	}
	sort.Slice(sortedDiffsA, func(i, j int) bool {
		if sortedDiffsA[i].BlockA == nil || sortedDiffsA[j].BlockA == nil {
			return false
		}
		return sortedDiffsA[i].BlockA.LineStart < sortedDiffsA[j].BlockA.LineStart
	})

	lastReportedBlockKey := ""

	currentFocusLineNum := focus.StartLine
	for currentFocusLineNum <= focus.EndLine {
		if currentFocusLineNum-1 >= len(fileALines) {
			fmt.Printf("\nLine A:%d: (Beyond end of File A)\n", currentFocusLineNum)
			break
		}

		var intersectingDiffEntry *DiffEntry = nil
		for i := range sortedDiffsA {
			d_ptr := &sortedDiffsA[i]
			if d_ptr.BlockA != nil && currentFocusLineNum >= d_ptr.BlockA.LineStart && currentFocusLineNum <= d_ptr.BlockA.LineEnd {
				intersectingDiffEntry = d_ptr
				break
			}
		}

		if intersectingDiffEntry != nil {
			blockA := intersectingDiffEntry.BlockA
			entryKey := fmt.Sprintf("%s-A%d", intersectingDiffEntry.Type.String(), blockA.ID)

			if entryKey != lastReportedBlockKey {
				fmt.Printf("\nLines A:%d-%d are part of a %s block (Original A Lines: %d-%d):\n",
					max(focus.StartLine, blockA.LineStart), min(focus.EndLine, blockA.LineEnd),
					intersectingDiffEntry.Type, blockA.LineStart, blockA.LineEnd)

				switch intersectingDiffEntry.Type {
				case Deleted:
					fmt.Printf("    Content (from A): \"%s\"\n", summarizedText(blockA.OriginalText, true))
				case Unchanged:
					fmt.Printf("    Matched with File B Lines: ~%d-%d\n", intersectingDiffEntry.BlockB.LineStart, intersectingDiffEntry.BlockB.LineEnd)
					fmt.Printf("    Content: \"%s\"\n", summarizedText(blockA.OriginalText, true))
				case Moved:
					fmt.Printf("    Moved to File B Lines: ~%d-%d\n", intersectingDiffEntry.BlockB.LineStart, intersectingDiffEntry.BlockB.LineEnd)
					fmt.Printf("    Content (from A): \"%s\"\n", summarizedText(blockA.OriginalText, true))
					if intersectingDiffEntry.Similarity > 0 && intersectingDiffEntry.Similarity < 0.9999 {
						fmt.Printf("    (Note: Content also modified, Block Similarity: %.2f)\n", intersectingDiffEntry.Similarity)
					}
				case Modified:
					fmt.Printf("    Changed from/to File B Lines: ~%d-%d\n", intersectingDiffEntry.BlockB.LineStart, intersectingDiffEntry.BlockB.LineEnd)
					fmt.Printf("    (Overall Block Similarity: %.2f)\n", intersectingDiffEntry.Similarity)
					if len(intersectingDiffEntry.LineDiffs) > 0 {
						fmt.Println("    Line-level changes within this block:")
						for _, op := range intersectingDiffEntry.LineDiffs {
							opTextLines := strings.Split(strings.TrimSuffix(op.Text, "\n"), "\n")
							for _, opLine := range opTextLines {
								if strings.TrimSpace(opLine) == "" && op.Operation == diffmatchpatch.DiffEqual {
									continue
								}
								prefix := "      "
								switch op.Operation {
								case diffmatchpatch.DiffInsert:
									prefix += "+ "
								case diffmatchpatch.DiffDelete:
									prefix += "- "
								case diffmatchpatch.DiffEqual:
									prefix += "  "
								}
								fmt.Printf("%s%s\n", prefix, opLine)
							}
						}
					}
				}
				lastReportedBlockKey = entryKey
			}
			currentFocusLineNum = min(focus.EndLine, blockA.LineEnd) + 1
		} else {
			fmt.Printf("\nLine A:%d: \"%s\"\n", currentFocusLineNum, fileALines[currentFocusLineNum-1])
			fmt.Printf("  Status: Line not part of any reported diff block.\n")
			currentFocusLineNum++
		}
	}
}

// min is stable
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// max is stable
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
