package main

import (
	"math"

	"github.com/agnivade/levenshtein"
)

// const SemanticSimilarityThreshold = 0.55 // No longer needed here, it's a global var in main.go

// StubbedCosineSimilarity - STUBBED.
func StubbedCosineSimilarity(vecA, vecB []float32) float32 {
	if len(vecA) != len(vecB) || len(vecA) == 0 {
		return 0.0
	}
	var dotProduct float32
	var normA float32
	var normB float32
	for i := 0; i < len(vecA); i++ {
		dotProduct += vecA[i] * vecB[i]
		normA += vecA[i] * vecA[i]
		normB += vecB[i] * vecB[i]
	}
	if normA == 0 || normB == 0 {
		return 0.0
	}
	denominator := float32(math.Sqrt(float64(normA)) * math.Sqrt(float64(normB)))
	if denominator == 0 {
		return 0.0
	}
	return dotProduct / denominator
}

// TextSimilarityNormalized uses Levenshtein distance.
func TextSimilarityNormalized(textA, textB string) float32 {
	if len(textA) == 0 && len(textB) == 0 {
		return 1.0
	}
	if len(textA) == 0 || len(textB) == 0 {
		return 0.0
	}

	dist := levenshtein.ComputeDistance(textA, textB)
	maxLen := len(textA)
	if len(textB) > maxLen {
		maxLen = len(textB)
	}
	if maxLen == 0 {
		return 1.0
	}
	similarity := 1.0 - (float32(dist) / float32(maxLen))
	return similarity
}
