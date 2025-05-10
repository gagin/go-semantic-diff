package main

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

// ContentBlock represents a segment of text from a file.
type ContentBlock struct {
	ID             int       // Unique ID within its original file (e.g., paragraph number)
	OriginalText   string    // The raw text
	NormalizedText string    // Text after normalization (lowercase, remove extra spaces)
	Checksum       string    // SHA256 of NormalizedText
	Embedding      []float32 // STUBBED: Would be the Nomic embedding vector
	LineStart      int       // Original starting line number
	LineEnd        int       // Original ending line number
	FileOrigin     string    // "A" or "B"
}

var (
	paragraphSeparator = regexp.MustCompile(`\n\s*\n`)
	spaceNormalizer    = regexp.MustCompile(`\s+`)
)

// NormalizeText converts text to lowercase and collapses multiple spaces.
func NormalizeText(text string) string {
	text = strings.ToLower(text)
	text = spaceNormalizer.ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

// CalculateChecksum generates a SHA256 hash of the text.
func CalculateChecksum(text string) string {
	hasher := sha256.New()
	hasher.Write([]byte(text))
	return hex.EncodeToString(hasher.Sum(nil))
}

// StubbedGetEmbedding simulates embedding generation.
func StubbedGetEmbedding(text string) []float32 {
	emb := make([]float32, 5)
	for _, char := range strings.ToLower(text) {
		switch char {
		case 'a':
			emb[0]++
		case 'e':
			emb[1]++
		case 'i':
			emb[2]++
		case 'o':
			emb[3]++
		case 'u':
			emb[4]++
		}
	}
	var sum float32
	for _, v := range emb {
		sum += v * v
	}
	if sum > 0 {
		normFactor := float32(1.0 / (float32(len(emb)) * (sum + 1e-9)))
		for i := range emb {
			emb[i] *= normFactor
		}
	}
	return emb
}

// SegmentFile breaks a file content string into ContentBlocks (paragraphs).
// Simplified line counting.
func SegmentFile(content string, fileOrigin string) []ContentBlock {
	var blocks []ContentBlock
	content = strings.ReplaceAll(content, "\r\n", "\n") // Normalize line endings
	rawParagraphs := paragraphSeparator.Split(content, -1)

	currentLineNumberInOriginalFile := 1 // Start from line 1

	for blockID, para := range rawParagraphs {
		trimmedPara := strings.TrimSpace(para)

		// Calculate original line numbers for this paragraph segment before trimming
		// This counts lines in the 'para' (which includes leading/trailing whitespace for the segment)
		startLineForThisPara := currentLineNumberInOriginalFile

		if trimmedPara == "" {
			// If the trimmed paragraph is empty, it means this segment was only whitespace/newlines.
			// Advance the line counter by the number of newlines in this segment.
			// The +1 for paragraphSeparator.Split means it splits *by* the separator,
			// so an empty paragraph implies the separator itself, which usually takes 1 or 2 lines.
			// Count lines in the raw 'para' part.
			linesInEmptySegment := strings.Count(para, "\n")
			currentLineNumberInOriginalFile += linesInEmptySegment
			if para != "" && !strings.HasSuffix(para, "\n") { // If 'para' was not empty but trimmed to empty, and didn't end with \n
				currentLineNumberInOriginalFile++ // Account for the line it was on
			} else if linesInEmptySegment == 0 && para != "" { // e.g. "  \n  " -> split gives "  ", Count \n is 0
				currentLineNumberInOriginalFile++
			} else if linesInEmptySegment > 0 { // If there were newlines, they are counted.
				// The +1 for the *next* paragraph starting line is handled at end of loop.
			}

			// If the segment was just the separator (e.g. "\n\n"), then para might be "\n".
			// The line number should advance past these separator lines.
			if len(rawParagraphs) > 1 && blockID < len(rawParagraphs)-1 { // If not the last potential segment
				currentLineNumberInOriginalFile++ // Account for one line of the separator itself
			}
			continue
		}

		normalized := NormalizeText(trimmedPara)
		linesInThisTrimmedBlock := strings.Count(trimmedPara, "\n") // Lines within the content itself

		block := ContentBlock{
			ID:             blockID,
			OriginalText:   trimmedPara,
			NormalizedText: normalized,
			Checksum:       CalculateChecksum(normalized),
			Embedding:      StubbedGetEmbedding(normalized),
			LineStart:      startLineForThisPara, // Line where this paragraph content starts
			LineEnd:        startLineForThisPara + linesInThisTrimmedBlock,
			FileOrigin:     fileOrigin,
		}
		blocks = append(blocks, block)

		// Advance currentLineNumberInOriginalFile by lines in the original 'para' segment,
		// which includes the content lines and the separator lines if any.
		currentLineNumberInOriginalFile += strings.Count(para, "\n")
		// If split by "\n\n", each `para` ends before the next separator,
		// so we need to account for the separator lines to correctly position the *next* paragraph.
		if len(rawParagraphs) > 1 && blockID < len(rawParagraphs)-1 {
			currentLineNumberInOriginalFile++ // At least one line for the separator
			if strings.HasPrefix(rawParagraphs[blockID+1], "\n") && paragraphSeparator.String() == `\n\s*\n` {
				// If the separator implies two newlines (like \n\n), and next para starts with one of them.
				// This logic is getting tricky. A simpler way is to recount from original for each block,
				// or use character offsets.
				// For now, this +1 is a heuristic for the blank line separator.
			}
		}
	}
	// A more robust line counting would involve re-parsing original content or tracking char offsets.
	// This version tries to approximate based on paragraph structure.
	// Let's refine LineStart for subsequent blocks if the previous block was multi-line
	if len(blocks) > 0 {
		currentLine := 1
		for i := range blocks {
			blocks[i].LineStart = currentLine
			blocks[i].LineEnd = currentLine + strings.Count(blocks[i].OriginalText, "\n")
			currentLine = blocks[i].LineEnd + 2 // +1 for next line, +1 for assumed blank separator
		}
	}

	return blocks
}
