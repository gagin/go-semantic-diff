package main

import (
	"flag" // Import the flag package
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

// Global variable to hold the debug state
var DebugMode bool

func main() {
	// Define a boolean flag -debug, with a default value of false, and a short description.
	// The address of DebugMode is passed so flag.Parse can set it.
	flag.BoolVar(&DebugMode, "debug", false, "Enable debug printing")
	flag.Parse() // Parse the command-line flags

	// Check if the correct number of non-flag arguments (file paths) is provided
	if flag.NArg() != 2 {
		fmt.Fprintln(os.Stderr, "Usage: go-semantic-diff [--debug] <fileA> <fileB>")
		os.Exit(1)
	}

	fileAPath := flag.Arg(0) // Get the first non-flag argument
	fileBPath := flag.Arg(1) // Get the second non-flag argument

	contentABytes, err := ioutil.ReadFile(fileAPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", fileAPath, err)
		os.Exit(1)
	}
	contentBBytes, err := ioutil.ReadFile(fileBPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", fileBPath, err)
		os.Exit(1)
	}

	contentA := string(contentABytes)
	contentB := string(contentBBytes)

	blocksA := SegmentFile(contentA, "A")
	blocksB := SegmentFile(contentB, "B")

	if DebugMode { // Print initial block counts only if debug is on
		fmt.Printf("File A (%s) has %d blocks.\n", fileAPath, len(blocksA))
		fmt.Printf("File B (%s) has %d blocks.\n", fileBPath, len(blocksB))
		fmt.Println("--- Performing Diff (Debug Mode) ---")
	} else {
		// Optional: could print a more concise starting message or nothing
		// fmt.Println("--- Performing Diff ---")
	}

	diffResults := PerformDiff(blocksA, blocksB) // PerformDiff will now use the global DebugMode

	if len(diffResults) == 0 {
		fmt.Println("Files are semantically identical at the block level (based on current logic).")
		return
	}

	for _, entry := range diffResults {
		fmt.Printf("\n[%s]\n", entry.Type)
		switch entry.Type {
		case Added:
			fmt.Printf("  Block ID %d from File B (Lines ~%d-%d):\n", entry.BlockB.ID, entry.BlockB.LineStart, entry.BlockB.LineEnd)
			fmt.Printf("  \"%s\"\n", summarizedText(entry.BlockB.OriginalText))
		case Deleted:
			fmt.Printf("  Block ID %d from File A (Lines ~%d-%d):\n", entry.BlockA.ID, entry.BlockA.LineStart, entry.BlockA.LineEnd)
			fmt.Printf("  \"%s\"\n", summarizedText(entry.BlockA.OriginalText))
		case Modified:
			fmt.Printf("  File A Block ID %d (Lines ~%d-%d): \"%s\"\n", entry.BlockA.ID, entry.BlockA.LineStart, entry.BlockA.LineEnd, summarizedText(entry.BlockA.OriginalText))
			fmt.Printf("  File B Block ID %d (Lines ~%d-%d): \"%s\"\n", entry.BlockB.ID, entry.BlockB.LineStart, entry.BlockB.LineEnd, summarizedText(entry.BlockB.OriginalText))
			fmt.Printf("  Similarity: %.2f\n", entry.Similarity)
		case Unchanged:
			fmt.Printf("  Matched Content (Exact):\n")
			fmt.Printf("  File A Block ID %d (Lines ~%d-%d) & File B Block ID %d (Lines ~%d-%d)\n", entry.BlockA.ID, entry.BlockA.LineStart, entry.BlockA.LineEnd, entry.BlockB.ID, entry.BlockB.LineStart, entry.BlockB.LineEnd)
			fmt.Printf("  \"%s\"\n", summarizedText(entry.BlockA.OriginalText))
			if entry.BlockA.LineStart != entry.BlockB.LineStart || entry.BlockA.LineEnd != entry.BlockB.LineEnd {
				fmt.Printf("  (Note: Positional difference detected for this matched block based on line numbers)\n")
			}
		}
	}
}

func summarizedText(text string) string {
	text = strings.ReplaceAll(text, "\n", "↵ ")
	maxLength := 80
	runes := []rune(text)
	if len(runes) > maxLength {
		return string(runes[:maxLength-3]) + "..."
	}
	return text
}
