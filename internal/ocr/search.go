package ocr

import (
	"fmt"
	"regexp"
	"strings"
)

// SearchResult represents a single match found in OCR text
type SearchResult struct {
	LineNumber int    `json:"lineNumber"` // 0-based line number
	Line       string `json:"line"`       // Full line content
	StartCol   int    `json:"startCol"`   // Start column of match (0-based)
	EndCol     int    `json:"endCol"`     // End column of match (0-based)
	Match      string `json:"match"`      // Matched text
	Groups     []string `json:"groups,omitempty"` // Regex capture groups (for regex searches)
}

// SearchConfig holds configuration for search operations
type SearchConfig struct {
	IgnoreCase  bool // Case-insensitive search
	FirstOnly   bool // Stop at first match (bottom-up)
	Quiet       bool // No text output, only exit codes
	LineNumbers bool // Show line numbers in output
}

// SearchResults holds all matches and metadata
type SearchResults struct {
	Query      string         `json:"query"`      // Original search query
	Matches    []SearchResult `json:"matches"`    // All matches found
	TotalLines int           `json:"totalLines"` // Total lines scanned
	Found      bool          `json:"found"`      // Whether any matches were found
}

// FindString searches for a literal string in OCR results (bottom-up scanning)
func FindString(result *OCRResult, query string, config SearchConfig) *SearchResults {
	searchResults := &SearchResults{
		Query:      query,
		Matches:    []SearchResult{},
		TotalLines: result.Height,
		Found:      false,
	}

	// Prepare search query based on case sensitivity
	searchQuery := query
	if config.IgnoreCase {
		searchQuery = strings.ToLower(query)
	}

	// Scan from bottom to top (newest output first)
	for i := result.Height - 1; i >= 0; i-- {
		line := result.Text[i]
		searchLine := line

		if config.IgnoreCase {
			searchLine = strings.ToLower(line)
		}

		// Find all occurrences in this line
		startPos := 0
		for {
			pos := strings.Index(searchLine[startPos:], searchQuery)
			if pos == -1 {
				break
			}

			actualPos := startPos + pos
			match := SearchResult{
				LineNumber: i,
				Line:       line,
				StartCol:   actualPos,
				EndCol:     actualPos + len(query),
				Match:      line[actualPos:actualPos + len(query)], // Use original case
			}

			searchResults.Matches = append(searchResults.Matches, match)
			searchResults.Found = true

			// If we only want the first match, return immediately
			if config.FirstOnly {
				return searchResults
			}

			// Move past this match to find additional occurrences in same line
			startPos = actualPos + len(query)
		}
	}

	return searchResults
}

// FindRegex searches for a regex pattern in OCR results (bottom-up scanning)
func FindRegex(result *OCRResult, pattern string, config SearchConfig) (*SearchResults, error) {
	// Compile the regex pattern
	var re *regexp.Regexp
	var err error

	if config.IgnoreCase {
		re, err = regexp.Compile("(?i)" + pattern)
	} else {
		re, err = regexp.Compile(pattern)
	}

	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %v", err)
	}

	searchResults := &SearchResults{
		Query:      pattern,
		Matches:    []SearchResult{},
		TotalLines: result.Height,
		Found:      false,
	}

	// Scan from bottom to top (newest output first)
	for i := result.Height - 1; i >= 0; i-- {
		line := result.Text[i]

		// Find all regex matches in this line
		matches := re.FindAllStringSubmatch(line, -1)
		indices := re.FindAllStringIndex(line, -1)

		for j, match := range matches {
			if j < len(indices) {
				startCol := indices[j][0]
				endCol := indices[j][1]

				searchResult := SearchResult{
					LineNumber: i,
					Line:       line,
					StartCol:   startCol,
					EndCol:     endCol,
					Match:      match[0], // Full match
					Groups:     match[1:], // Capture groups (if any)
				}

				searchResults.Matches = append(searchResults.Matches, searchResult)
				searchResults.Found = true

				// If we only want the first match, return immediately
				if config.FirstOnly {
					return searchResults, nil
				}
			}
		}
	}

	return searchResults, nil
}

// FormatResults formats search results for display based on configuration
func FormatResults(results *SearchResults, config SearchConfig) string {
	if config.Quiet || !results.Found {
		return "" // No output in quiet mode or when nothing found
	}

	var output strings.Builder

	if config.LineNumbers {
		// Detailed output with position information when line numbers are requested
		if len(results.Matches) == 1 {
			output.WriteString(fmt.Sprintf("Found 1 match for \"%s\":\n", results.Query))
		} else {
			output.WriteString(fmt.Sprintf("Found %d matches for \"%s\":\n", len(results.Matches), results.Query))
		}

		for _, match := range results.Matches {
			if len(match.Groups) > 0 {
				// Regex match with capture groups
				output.WriteString(fmt.Sprintf("Line %d (col %d-%d): %s [Groups: %v]\n",
					match.LineNumber, match.StartCol, match.EndCol-1, match.Line, match.Groups))
			} else {
				// Simple string match
				output.WriteString(fmt.Sprintf("Line %d (col %d-%d): %s\n",
					match.LineNumber, match.StartCol, match.EndCol-1, match.Line))
			}
		}
	} else {
		// Default output - just content
		for _, match := range results.Matches {
			output.WriteString(fmt.Sprintf("%s\n", match.Line))
		}
	}

	return output.String()
}

// GetExitCode returns the appropriate exit code for the search results
func GetExitCode(results *SearchResults, err error) int {
	if err != nil {
		// Check if it's a regex compilation error
		if strings.Contains(err.Error(), "invalid regex pattern") {
			return 3 // Invalid regex pattern
		}
		return 2 // OCR processing error
	}

	if results.Found {
		return 0 // Found one or more matches
	}

	return 1 // No matches found
}
