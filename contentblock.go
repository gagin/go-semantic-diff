package main

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

type ContentBlock struct {
	ID             int
	OriginalText   string
	NormalizedText string
	Checksum       string
	Embedding      []float32
	LineStart      int
	LineEnd        int
	FileOrigin     string
	SourceLineRefs []LineInfo
}

type LineInfo struct {
	OriginalText    string
	TrimmedText     string
	Checksum        string
	OriginalLineNum int
	FileOrigin      string
	IsPartOfMega    bool
	MegaBlockRefID  int
}

var spaceNormalizerContentBlock = regexp.MustCompile(`\s+`)

func NormalizeTextBlock(text string) string {
	text = strings.ToLower(text)
	text = spaceNormalizerContentBlock.ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

func CalculateLineChecksum(lineText string) string {
	normalizedLine := NormalizeTextBlock(lineText)
	hasher := sha256.New()
	hasher.Write([]byte(normalizedLine))
	return hex.EncodeToString(hasher.Sum(nil))
}

func CalculateBlockChecksum(blockText string) string {
	normalized := NormalizeTextBlock(blockText)
	hasher := sha256.New()
	hasher.Write([]byte(normalized))
	return hex.EncodeToString(hasher.Sum(nil))
}

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

var paragraphSeparatorForGapsContentBlock = regexp.MustCompile(`\n\s*\n`)

func SegmentGapText(gapLines []LineInfo, fileOrigin string, startBlockID int) ([]ContentBlock, int) {
	if len(gapLines) == 0 {
		return []ContentBlock{}, startBlockID
	}

	var gapContentBuilder strings.Builder
	for _, li := range gapLines {
		gapContentBuilder.WriteString(li.OriginalText)
		gapContentBuilder.WriteString("\n")
	}
	gapContent := strings.TrimSuffix(gapContentBuilder.String(), "\n")

	var finalBlocks []ContentBlock
	rawParagraphs := paragraphSeparatorForGapsContentBlock.Split(gapContent, -1)
	blockIDCounter := startBlockID

	// currentLineInfoIdx tracks the index in the original gapLines slice
	// that corresponds to the start of the current rawParagraph segment.
	currentLineInfoIdx := 0

	for _, paraText := range rawParagraphs {
		// Find the actual content lines corresponding to this paraText from gapLines
		// This loop consumes lines from gapLines until paraText is fully formed or gapLines end
		var currentParaLines []LineInfo
		var builtPara strings.Builder
		//startIndexInGapLinesForThisPara := currentLineInfoIdx

		for currentLineInfoIdx < len(gapLines) {
			lineInfo := gapLines[currentLineInfoIdx]
			currentParaLines = append(currentParaLines, lineInfo)
			builtPara.WriteString(lineInfo.OriginalText)

			// Check if the builtPara now matches the current rawParagraphs[i]
			// This is tricky because Split can produce segments that are just newlines
			// or that span multiple original lines.

			// A simpler heuristic: assume paraText corresponds to a sequence of lines in gapLines.
			// Count how many original lines are in paraText (including its internal newlines)
			// to advance currentLineInfoIdx.

			currentLineInfoIdx++                // Always advance at least one line from gapLines
			if builtPara.String() == paraText { // If we've reconstructed the exact segment from Split
				break
			}
			// If paraText ends with a newline and builtPara doesn't yet, add another line from gapLines
			// This logic is complex if paraText itself contains multiple original lines from gapLines.
			// For now, we'll assume paraText from Split can be reconstructed by joining
			// some number of consecutive lines from gapLines.
			// The number of lines is paraLinesCount.
			if builtPara.Len() >= len(paraText) { // if built string is already as long or longer
				break
			}
			builtPara.WriteString("\n") // add separator for next line from gapLines
		}

		trimmedPara := strings.TrimSpace(paraText) // Use paraText from Split for trimming
		if trimmedPara == "" {
			continue // Skip empty paragraphs
		}

		if len(currentParaLines) == 0 { // Should not happen if trimmedPara is not empty
			continue
		}

		normalized := NormalizeTextBlock(trimmedPara)

		block := ContentBlock{
			ID:             blockIDCounter,
			OriginalText:   trimmedPara,
			NormalizedText: normalized,
			Checksum:       CalculateBlockChecksum(trimmedPara),
			Embedding:      StubbedGetEmbedding(normalized),
			LineStart:      currentParaLines[0].OriginalLineNum,
			LineEnd:        currentParaLines[len(currentParaLines)-1].OriginalLineNum,
			FileOrigin:     fileOrigin,
			SourceLineRefs: currentParaLines, // Store the actual LineInfo objects
		}
		finalBlocks = append(finalBlocks, block)
		blockIDCounter++
	}
	return finalBlocks, blockIDCounter
}
