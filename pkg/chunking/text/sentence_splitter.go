package text

import (
	"strings"
	"unicode"
)

// SentenceSplitter splits text into sentences
type SentenceSplitter interface {
	Split(text string) []string
}

// DefaultSentenceSplitter implements a rule-based sentence splitter
type DefaultSentenceSplitter struct {
	abbreviations map[string]bool
}

// NewSentenceSplitter creates a new sentence splitter
func NewSentenceSplitter() SentenceSplitter {
	return &DefaultSentenceSplitter{
		abbreviations: getCommonAbbreviations(),
	}
}

// Split splits text into sentences
func (s *DefaultSentenceSplitter) Split(text string) []string {
	if strings.TrimSpace(text) == "" {
		return []string{}
	}

	var sentences []string
	var currentSentence strings.Builder
	runes := []rune(text)

	for i := 0; i < len(runes); i++ {
		currentSentence.WriteRune(runes[i])

		// Check for sentence endings
		if s.isSentenceEnd(runes, i) {
			// Look ahead for continuation
			if !s.isContinuation(runes, i) {
				sentence := strings.TrimSpace(currentSentence.String())
				if sentence != "" {
					sentences = append(sentences, sentence)
				}
				currentSentence.Reset()
			}
		}
	}

	// Add any remaining text as a sentence
	if currentSentence.Len() > 0 {
		sentence := strings.TrimSpace(currentSentence.String())
		if sentence != "" {
			sentences = append(sentences, sentence)
		}
	}

	return sentences
}

// isSentenceEnd checks if the current position is a sentence ending
func (s *DefaultSentenceSplitter) isSentenceEnd(runes []rune, pos int) bool {
	if pos >= len(runes) {
		return false
	}

	r := runes[pos]

	// Check for sentence-ending punctuation
	if r != '.' && r != '!' && r != '?' {
		return false
	}

	// For periods, check if it's part of an abbreviation
	if r == '.' {
		// Look back to find the word
		wordStart := pos
		for wordStart > 0 && !unicode.IsSpace(runes[wordStart-1]) {
			wordStart--
		}

		word := string(runes[wordStart:pos])
		if s.isAbbreviation(word) {
			return false
		}

		// Check for decimal numbers (e.g., 3.14)
		if pos > 0 && unicode.IsDigit(runes[pos-1]) &&
			pos+1 < len(runes) && unicode.IsDigit(runes[pos+1]) {
			return false
		}

		// Check for ellipsis
		if pos+2 < len(runes) && runes[pos+1] == '.' && runes[pos+2] == '.' {
			return false
		}
		if pos > 0 && pos+1 < len(runes) &&
			runes[pos-1] == '.' && runes[pos+1] == '.' {
			return false
		}
	}

	// Look ahead to see if followed by space and capital letter or end of text
	if pos+1 >= len(runes) {
		return true
	}

	// Check for closing quotes or parentheses after punctuation
	nextPos := pos + 1
	for nextPos < len(runes) && (runes[nextPos] == '"' || runes[nextPos] == '\'' ||
		runes[nextPos] == ')' || runes[nextPos] == ']' ||
		runes[nextPos] == '}') {
		nextPos++
	}

	// Must be followed by whitespace
	if nextPos < len(runes) && !unicode.IsSpace(runes[nextPos]) {
		return false
	}

	// Skip whitespace
	for nextPos < len(runes) && unicode.IsSpace(runes[nextPos]) {
		nextPos++
	}

	// If at end of text, it's a sentence end
	if nextPos >= len(runes) {
		return true
	}

	// Check if next word starts with capital letter
	return unicode.IsUpper(runes[nextPos])
}

// isContinuation checks if the sentence continues after punctuation
func (s *DefaultSentenceSplitter) isContinuation(runes []rune, pos int) bool {
	if pos+1 >= len(runes) {
		return false
	}

	// Skip any quotes or brackets after punctuation
	nextPos := pos + 1
	for nextPos < len(runes) && (runes[nextPos] == '"' || runes[nextPos] == '\'' ||
		runes[nextPos] == ')' || runes[nextPos] == ']') {
		nextPos++
	}

	// Check for immediate continuation without space (e.g., "U.S.A.")
	if nextPos < len(runes) && !unicode.IsSpace(runes[nextPos]) {
		return true
	}

	return false
}

// isAbbreviation checks if a word is a common abbreviation
func (s *DefaultSentenceSplitter) isAbbreviation(word string) bool {
	word = strings.ToLower(strings.TrimSpace(word))
	return s.abbreviations[word]
}

// getCommonAbbreviations returns a map of common abbreviations
func getCommonAbbreviations() map[string]bool {
	return map[string]bool{
		// Titles
		"mr": true, "mrs": true, "ms": true, "dr": true, "prof": true,
		"sr": true, "jr": true, "ph.d": true, "m.d": true, "b.a": true,
		"m.a": true, "b.s": true, "m.s": true,

		// Common abbreviations
		"inc": true, "corp": true, "co": true, "ltd": true, "llc": true,
		"vs": true, "etc": true, "i.e": true, "e.g": true, "cf": true,
		"al": true, "et": true,

		// Time
		"jan": true, "feb": true, "mar": true, "apr": true, "jun": true,
		"jul": true, "aug": true, "sep": true, "sept": true, "oct": true,
		"nov": true, "dec": true,
		"mon": true, "tue": true, "wed": true, "thu": true, "fri": true,
		"sat": true, "sun": true,

		// Measurements
		"ft": true, "in": true, "cm": true, "mm": true, "km": true,
		"kg": true, "lb": true, "oz": true, "pt": true, "qt": true,

		// Geography
		"st": true, "ave": true, "blvd": true, "rd": true, "u.s": true,
		"u.k": true, "u.s.a": true,

		// Tech abbreviations
		"api": true, "sdk": true, "ui": true, "ux": true, "db": true,
		"os": true, "cpu": true, "gpu": true, "ram": true, "ssd": true,
		"http": true, "https": true, "ftp": true, "ssh": true,
	}
}
