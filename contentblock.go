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
}

var spaceNormalizer = regexp.MustCompile(`\s+`)
var paragraphSeparator = regexp.MustCompile(`\n\s*\n`) // Key: uses blank lines as separators

func NormalizeText(text string) string {
	text = strings.ToLower(text)
	text = spaceNormalizer.ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

func CalculateChecksum(text string) string {
	hasher := sha256.New()
	hasher.Write([]byte(text))
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

// SegmentFile uses paragraph-based segmentation
func SegmentFile(content string, fileOrigin string) []ContentBlock {
	var finalBlocks []ContentBlock
	content = strings.ReplaceAll(content, "\r\n", "\n")
	rawParagraphs := paragraphSeparator.Split(content, -1)

	blockIDCounter := 0
	tempBlocks := []ContentBlock{}
	currentLineNumberInOriginalFile := 1

	for _, para := range rawParagraphs {
		startLineForPara := currentLineNumberInOriginalFile
		numInternalNewlinesInPara := strings.Count(para, "\n")
		trimmedPara := strings.TrimSpace(para)

		if trimmedPara == "" {
			currentLineNumberInOriginalFile += numInternalNewlinesInPara
			if para != "" {
				currentLineNumberInOriginalFile++
			}
			continue
		}

		normalized := NormalizeText(trimmedPara)
		linesInThisTrimmedBlock := strings.Count(trimmedPara, "\n")

		tempBlock := ContentBlock{
			ID:             blockIDCounter,
			OriginalText:   trimmedPara,
			NormalizedText: normalized,
			Checksum:       CalculateChecksum(normalized),
			Embedding:      StubbedGetEmbedding(normalized),
			LineStart:      startLineForPara,
			LineEnd:        startLineForPara + linesInThisTrimmedBlock,
			FileOrigin:     fileOrigin,
		}
		tempBlocks = append(tempBlocks, tempBlock)
		blockIDCounter++

		currentLineNumberInOriginalFile += numInternalNewlinesInPara
		if para != "" {
			currentLineNumberInOriginalFile++
		}
	}

	currentAdjustedLine := 1
	for i := range tempBlocks {
		block := tempBlocks[i]
		block.LineStart = currentAdjustedLine
		block.LineEnd = currentAdjustedLine + strings.Count(block.OriginalText, "\n")
		finalBlocks = append(finalBlocks, block)
		currentAdjustedLine = block.LineEnd + 2
	}
	return finalBlocks
}
