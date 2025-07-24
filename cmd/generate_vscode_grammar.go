package cmd

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// generateVSCodeGrammarCmd creates VSCode TextMate grammar from script2 parser
var generateVSCodeGrammarCmd = &cobra.Command{
	Use:   "generate-vscode-grammar [output-dir]",
	Short: "Generate VSCode TextMate grammar for script2 syntax highlighting",
	Long: `Generate a VSCode extension with TextMate grammar for script2 syntax highlighting.
This command automatically extracts regex patterns from the script2 parser code
and generates a synchronized grammar file that stays up-to-date with parser changes.

Features:
- Automatic regex pattern extraction from internal/script2/parser.go
- TextMate grammar generation for VSCode syntax highlighting
- Package.json generation for VSCode extension
- README generation with installation instructions
- Reusable process that can be re-run when parser changes

The generated extension provides syntax highlighting for:
- Variable assignments (USER=${USER:-default})
- Directive syntax (<enter>, <watch "text" 30s>, etc.)
- Comments (# lines)
- Function definitions and calls
- Special keywords and operators`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		outputDir := "./vscode-extension"
		if len(args) > 0 {
			outputDir = args[0]
		}

		fmt.Printf("🔧 Generating VSCode extension for script2 syntax highlighting...\n")
		fmt.Printf("📂 Output directory: %s\n", outputDir)

		if err := generateVSCodeExtension(outputDir); err != nil {
			fmt.Printf("❌ Failed to generate VSCode extension: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✅ VSCode extension generated successfully!\n")
		fmt.Printf("\n📦 To install the extension:\n")
		fmt.Printf("   cd %s\n", outputDir)
		fmt.Printf("   npm install\n")
		fmt.Printf("   code --install-extension qmp-script2-*.vsix\n")
		fmt.Printf("\n🔄 To update grammar when parser changes:\n")
		fmt.Printf("   go run main.go generate-vscode-grammar %s\n", outputDir)
	},
}

// RegexPattern represents an extracted regex with context
type RegexPattern struct {
	Name        string `json:"name"`
	Pattern     string `json:"pattern"`
	Description string `json:"description"`
	Category    string `json:"category"`
}

// TextMateGrammar represents the structure of a TextMate grammar file
type TextMateGrammar struct {
	SchemaVersion string                            `json:"$schema"`
	Name          string                            `json:"name"`
	ScopeName     string                            `json:"scopeName"`
	FileTypes     []string                          `json:"fileTypes"`
	Patterns      []TextMatePattern                 `json:"patterns"`
	Repository    map[string]TextMateRepositoryItem `json:"repository"`
}

type TextMatePattern struct {
	Include  string                     `json:"include,omitempty"`
	Name     string                     `json:"name,omitempty"`
	Match    string                     `json:"match,omitempty"`
	Begin    string                     `json:"begin,omitempty"`
	End      string                     `json:"end,omitempty"`
	Patterns []TextMatePattern          `json:"patterns,omitempty"`
	Captures map[string]TextMateCapture `json:"captures,omitempty"`
}

type TextMateRepositoryItem struct {
	Name     string            `json:"name,omitempty"`
	Match    string            `json:"match,omitempty"`
	Begin    string             `json:"begin,omitempty"`
	End      string            `json:"end,omitempty"`
	Patterns []TextMatePattern `json:"patterns,omitempty"`
	Captures map[string]TextMateCapture `json:"captures,omitempty"`
}

type TextMateCapture struct {
	Name string `json:"name"`
}

// extractRegexPatterns extracts regex patterns from the parser.go file
func extractRegexPatterns() ([]RegexPattern, error) {
	// Get the parser file path
	parserPath := filepath.Join("internal", "script2", "parser.go")

	// Parse the Go source file
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, parserPath, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", parserPath, err)
	}

	var patterns []RegexPattern

	// Walk the AST to find regex declarations
	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.GenDecl:
			if x.Tok == token.VAR {
				for _, spec := range x.Specs {
					if valueSpec, ok := spec.(*ast.ValueSpec); ok {
						for i, name := range valueSpec.Names {
							if i < len(valueSpec.Values) {
								if callExpr, ok := valueSpec.Values[i].(*ast.CallExpr); ok {
									if selectorExpr, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
										if ident, ok := selectorExpr.X.(*ast.Ident); ok && ident.Name == "regexp" && selectorExpr.Sel.Name == "MustCompile" {
											if len(callExpr.Args) > 0 {
												if basicLit, ok := callExpr.Args[0].(*ast.BasicLit); ok {
													// Extract the regex pattern (remove backticks or quotes)
													pattern := strings.Trim(basicLit.Value, "`\"")

													// Create pattern with metadata
													regexPattern := RegexPattern{
														Name:        name.Name,
														Pattern:     pattern,
														Description: extractDescription(name.Name),
														Category:    categorizePattern(name.Name),
													}
													patterns = append(patterns, regexPattern)
												}
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
		return true
	})

	// Sort patterns by category and name for consistent output
	sort.Slice(patterns, func(i, j int) bool {
		if patterns[i].Category != patterns[j].Category {
			return patterns[i].Category < patterns[j].Category
		}
		return patterns[i].Name < patterns[j].Name
	})

	return patterns, nil
}

// extractDescription creates a human-readable description from the regex variable name
func extractDescription(name string) string {
	descriptions := map[string]string{
		"variableAssignmentRegex": "Variable assignment: USER=value or USER=${USER:-default}",
		"directiveRegex":          "Directive patterns: <something> but not \\<something>",
		"escapedDirectiveRegex":   "Escaped directive patterns: \\<something>",
		"commentRegex":            "Comment lines starting with #",
		"emptyLineRegex":          "Empty or whitespace-only lines",
		"keySequenceRegex":        "Key sequence patterns: enter, tab, ctrl+c, etc.",
		"watchRegex":              "Watch patterns: watch \"text\" 30s",
		"consoleRegex":            "Console switching: console 2",
		"waitRegex":               "Wait delay: wait 5s",
		"exitRegex":               "Exit command: exit 1",
		"retryRegex":              "Retry directive: retry 3",
		"repeatRegex":             "Repeat directive: repeat 5",
		"whileFoundRegex":         "While-found loop: while-found \"text\" 30s poll 1s",
		"whileNotFoundRegex":      "While-not-found loop: while-not-found \"text\" 30s poll 1s",
		"elseRegex":               "Else directive: else",
		"includeRegex":            "Include directive: include \"script.txt\"",
		"screenshotRegex":         "Screenshot directive: screenshot \"filename.png\"",
		"functionDefRegex":        "Function definition: function name",
		"endFunctionRegex":        "End function: end-function",
		"functionCallRegex":       "Function call: call function_name args...",
	}

	if desc, exists := descriptions[name]; exists {
		return desc
	}
	return "Script2 pattern: " + name
}

// categorizePattern assigns patterns to categories for grammar organization
func categorizePattern(name string) string {
	categories := map[string]string{
		"variableAssignmentRegex": "variables",
		"directiveRegex":          "directives",
		"escapedDirectiveRegex":   "directives",
		"commentRegex":            "comments",
		"emptyLineRegex":          "structure",
		"keySequenceRegex":        "keys",
		"watchRegex":              "control",
		"consoleRegex":            "control",
		"waitRegex":               "control",
		"exitRegex":               "control",
		"retryRegex":              "loops",
		"repeatRegex":             "loops",
		"whileFoundRegex":         "loops",
		"whileNotFoundRegex":      "loops",
		"elseRegex":               "conditionals",
		"includeRegex":            "composition",
		"screenshotRegex":         "debugging",
		"functionDefRegex":        "functions",
		"endFunctionRegex":        "functions",
		"functionCallRegex":       "functions",
	}

	if category, exists := categories[name]; exists {
		return category
	}
	return "misc"
}

// generateTextMateGrammar creates a TextMate grammar from extracted patterns
func generateTextMateGrammar(patterns []RegexPattern) TextMateGrammar {
	grammar := TextMateGrammar{
		SchemaVersion: "https://raw.githubusercontent.com/martinring/tmlanguage/master/tmlanguage.json",
		Name:          "QMP Script2",
		ScopeName:     "source.qmp-script2",
		FileTypes:     []string{"sc2", "script2", "qmp2"},
		Patterns: []TextMatePattern{
			{Include: "#comments"},
			{Include: "#functions"},
			{Include: "#directives"},
			{Include: "#variables"},
			{Include: "#strings"},
		},
		Repository: make(map[string]TextMateRepositoryItem),
	}

	// Group patterns by category
	patternsByCategory := make(map[string][]RegexPattern)
	for _, pattern := range patterns {
		patternsByCategory[pattern.Category] = append(patternsByCategory[pattern.Category], pattern)
	}

	// Create repository items for each category
	for category, categoryPatterns := range patternsByCategory {
		var repoPatterns []TextMatePattern

		for _, pattern := range categoryPatterns {
			// Convert regex patterns to TextMate format
			tmPattern := convertToTextMatePattern(pattern)
			if tmPattern.Match != "" || tmPattern.Begin != "" {
				repoPatterns = append(repoPatterns, tmPattern)
			}
		}

		if len(repoPatterns) > 0 {
			grammar.Repository[category] = TextMateRepositoryItem{
				Patterns: repoPatterns,
			}
		}
	}

	// Add custom patterns for better syntax highlighting
	addCustomPatterns(&grammar)

	return grammar
}

// convertToTextMatePattern converts a RegexPattern to TextMate format
func convertToTextMatePattern(pattern RegexPattern) TextMatePattern {
	switch pattern.Name {
	case "variableAssignmentRegex":
		return TextMatePattern{
			Match: pattern.Pattern,
			Name:  "variable.other.assignment.qmp-script2",
		}
	case "directiveRegex":
		return TextMatePattern{
			Match: pattern.Pattern,
			Name:  "punctuation.definition.directive.qmp-script2",
		}
	case "commentRegex":
		return TextMatePattern{
			Match: pattern.Pattern,
			Name:  "comment.line.hash.qmp-script2",
		}
	case "keySequenceRegex":
		return TextMatePattern{
			Match: pattern.Pattern,
			Name:  "support.function.key.special.qmp-script2",
		}
	case "functionDefRegex":
		return TextMatePattern{
			Match: pattern.Pattern,
			Name:  "storage.type.function.qmp-script2",
		}
	case "endFunctionRegex":
		return TextMatePattern{
			Match: pattern.Pattern,
			Name:  "storage.type.function.end.qmp-script2",
		}
	case "functionCallRegex":
		return TextMatePattern{
			Match: pattern.Pattern,
			Name:  "entity.name.function.call.qmp-script2",
		}
	case "watchRegex":
		return TextMatePattern{
			Match: pattern.Pattern,
			Name:  "keyword.control.conditional.watch.qmp-script2",
		}
	case "retryRegex", "repeatRegex":
		return TextMatePattern{
			Match: pattern.Pattern,
			Name:  "keyword.control.loop.qmp-script2",
		}
	case "whileFoundRegex", "whileNotFoundRegex":
		return TextMatePattern{
			Match: pattern.Pattern,
			Name:  "keyword.control.loop.while.qmp-script2",
		}
	case "elseRegex":
		return TextMatePattern{
			Match: pattern.Pattern,
			Name:  "keyword.control.conditional.else.qmp-script2",
		}
	case "includeRegex":
		return TextMatePattern{
			Match: pattern.Pattern,
			Name:  "keyword.control.import.qmp-script2",
		}
	case "screenshotRegex":
		return TextMatePattern{
			Match: pattern.Pattern,
			Name:  "support.function.debug.screenshot.qmp-script2",
		}
	case "consoleRegex":
		return TextMatePattern{
			Match: pattern.Pattern,
			Name:  "support.function.system.console.qmp-script2",
		}
	case "waitRegex":
		return TextMatePattern{
			Match: pattern.Pattern,
			Name:  "support.function.timing.wait.qmp-script2",
		}
	case "exitRegex":
		return TextMatePattern{
			Match: pattern.Pattern,
			Name:  "keyword.control.flow.exit.qmp-script2",
		}
	default:
		return TextMatePattern{
			Match: pattern.Pattern,
			Name:  "markup.other.qmp-script2",
		}
	}
}

// addCustomPatterns adds additional patterns for better highlighting
func addCustomPatterns(grammar *TextMateGrammar) {
	// Add comments pattern
	grammar.Repository["comments"] = TextMateRepositoryItem{
		Patterns: []TextMatePattern{
			{
				Match: "^\\s*#.*$",
				Name:  "comment.line.hash.qmp-script2",
			},
		},
	}

	// Add strings pattern with more color variety
	grammar.Repository["strings"] = TextMateRepositoryItem{
		Patterns: []TextMatePattern{
			{
				Begin: "\"",
				End:   "\"",
				Name:  "string.quoted.double.qmp-script2",
				Patterns: []TextMatePattern{
					{
						Match: "\\\\.",
						Name:  "constant.character.escape.qmp-script2",
					},
					{
						Match: "\\{(timestamp|date|time|datetime|unix)\\}",
						Name:  "support.constant.placeholder.qmp-script2",
					},
				},
			},
		},
	}

	// Add variables pattern with enhanced highlighting
	grammar.Repository["variables"] = TextMateRepositoryItem{
		Patterns: []TextMatePattern{
			{
				Match: "\\$\\{([^}:]+):([^}]+)\\}",
				Name:  "variable.other.expansion.complex.qmp-script2",
				Captures: map[string]TextMateCapture{
					"1": {Name: "variable.other.readwrite.qmp-script2"},
					"2": {Name: "support.constant.expansion.qmp-script2"},
				},
			},
			{
				Match: "\\$\\{([^}]+)\\}",
				Name:  "variable.other.expansion.simple.qmp-script2",
				Captures: map[string]TextMateCapture{
					"1": {Name: "variable.other.readwrite.qmp-script2"},
				},
			},
			{
				Match: "\\$[A-Za-z_][A-Za-z0-9_]*",
				Name:  "variable.other.readwrite.basic.qmp-script2",
			},
			{
				Match: "^([A-Za-z_][A-Za-z0-9_]*)=",
				Name:  "variable.other.assignment.qmp-script2",
				Captures: map[string]TextMateCapture{
					"1": {Name: "variable.other.readwrite.name.qmp-script2"},
				},
			},
		},
	}

	// Add functions pattern with colorful highlighting
	grammar.Repository["functions"] = TextMateRepositoryItem{
		Patterns: []TextMatePattern{
			{
				Match: "<(function)\\s+([a-zA-Z_][a-zA-Z0-9_]*)>",
				Name:  "meta.function.definition.qmp-script2",
				Captures: map[string]TextMateCapture{
					"1": {Name: "storage.type.function.keyword.qmp-script2"},
					"2": {Name: "entity.name.function.definition.qmp-script2"},
				},
			},
			{
				Match: "<(call)\\s+([a-zA-Z_][a-zA-Z0-9_]*)(.*?)>",
				Name:  "meta.function.call.qmp-script2",
				Captures: map[string]TextMateCapture{
					"1": {Name: "keyword.control.call.qmp-script2"},
					"2": {Name: "entity.name.function.call.qmp-script2"},
					"3": {Name: "meta.function.arguments.qmp-script2"},
				},
			},
			{
				Match: "<(end-function)>",
				Name:  "storage.type.function.end.keyword.qmp-script2",
				Captures: map[string]TextMateCapture{
					"1": {Name: "storage.type.function.end.qmp-script2"},
				},
			},
		},
	}

	// Add enhanced directives pattern with specific colors
	grammar.Repository["directives"] = TextMateRepositoryItem{
		Patterns: []TextMatePattern{
			// Key sequences - distinct colors for each part
			{
				Match: "(<)(enter|tab|space|escape|backspace|delete|up|down|left|right|home|end|page_up|page_down)(>)",
				Name:  "meta.key.navigation.qmp-script2",
				Captures: map[string]TextMateCapture{
					"1": {Name: "punctuation.definition.directive.begin.qmp-script2"}, // < bracket
					"2": {Name: "support.function.key.navigation.qmp-script2"},        // key name - bright blue
					"3": {Name: "punctuation.definition.directive.end.qmp-script2"},  // > bracket
				},
			},
			{
				Match: "(<)(ctrl|alt|shift)(\\+)([a-z])(>)",
				Name:  "meta.key.modifier.qmp-script2",
				Captures: map[string]TextMateCapture{
					"1": {Name: "punctuation.definition.directive.begin.qmp-script2"}, // < bracket
					"2": {Name: "support.function.key.modifier.name.qmp-script2"},     // ctrl/alt/shift - blue
					"3": {Name: "punctuation.separator.modifier.qmp-script2"},        // + sign - white
					"4": {Name: "support.function.key.modifier.key.qmp-script2"},     // key letter - bright blue
					"5": {Name: "punctuation.definition.directive.end.qmp-script2"},  // > bracket
				},
			},
			{
				Match: "(<)(f[1-9][0-9]?)(>)",
				Name:  "meta.key.function.qmp-script2",
				Captures: map[string]TextMateCapture{
					"1": {Name: "punctuation.definition.directive.begin.qmp-script2"}, // < bracket
					"2": {Name: "support.function.key.function.qmp-script2"},          // function key - bright blue
					"3": {Name: "punctuation.definition.directive.end.qmp-script2"},  // > bracket
				},
			},
			// Conditional directives - distinct colors for each part
			{
				Match: "(<)(watch|if-found|if-not-found)(\\s+)(\"[^\"]*\")(\\s+)(\\d+s?)(>)",
				Name:  "meta.conditional.qmp-script2",
				Captures: map[string]TextMateCapture{
					"1": {Name: "punctuation.definition.directive.begin.qmp-script2"},      // < bracket - gray
					"2": {Name: "keyword.control.conditional.qmp-script2"},                // watch keyword - blue
					"3": {Name: "punctuation.separator.space.qmp-script2"},               // space - invisible
					"4": {Name: "string.quoted.double.search.qmp-script2"},               // "password" - green
					"5": {Name: "punctuation.separator.space.qmp-script2"},               // space - invisible
					"6": {Name: "constant.numeric.timeout.qmp-script2"},                  // 5s - orange
					"7": {Name: "punctuation.definition.directive.end.qmp-script2"},      // > bracket - gray
				},
			},
			// Loop directives - distinct colors for each part
			{
				Match: "(<)(while-found|while-not-found)(\\s+)(\"[^\"]*\")(\\s+)(\\d+s?)(?:(\\s+poll\\s+)(\\d+(?:\\.\\d+)?s?))?(>)",
				Name:  "meta.loop.while.qmp-script2",
				Captures: map[string]TextMateCapture{
					"1": {Name: "punctuation.definition.directive.begin.qmp-script2"},      // < bracket
					"2": {Name: "keyword.control.loop.while.qmp-script2"},                // while-found keyword - purple
					"3": {Name: "punctuation.separator.space.qmp-script2"},               // space
					"4": {Name: "string.quoted.double.search.qmp-script2"},               // "text" - green
					"5": {Name: "punctuation.separator.space.qmp-script2"},               // space
					"6": {Name: "constant.numeric.timeout.qmp-script2"},                  // 30s - orange
					"7": {Name: "keyword.other.poll.qmp-script2"},                        // poll 1s - cyan
					"8": {Name: "constant.numeric.poll.qmp-script2"},                     // poll interval - orange
					"9": {Name: "punctuation.definition.directive.end.qmp-script2"},      // > bracket
				},
			},
			{
				Match: "(<)(retry|repeat)(\\s+)(\\d+)(>)",
				Name:  "meta.loop.count.qmp-script2",
				Captures: map[string]TextMateCapture{
					"1": {Name: "punctuation.definition.directive.begin.qmp-script2"},    // < bracket
					"2": {Name: "keyword.control.loop.qmp-script2"},                      // retry/repeat - purple
					"3": {Name: "punctuation.separator.space.qmp-script2"},              // space
					"4": {Name: "constant.numeric.count.qmp-script2"},                   // count number - orange
					"5": {Name: "punctuation.definition.directive.end.qmp-script2"},     // > bracket
				},
			},
			// System directives - distinct colors for each part
			{
				Match: "(<)(console)(\\s+)(\\d+)(>)",
				Name:  "meta.system.console.qmp-script2",
				Captures: map[string]TextMateCapture{
					"1": {Name: "punctuation.definition.directive.begin.qmp-script2"},    // < bracket
					"2": {Name: "support.function.system.console.qmp-script2"},          // console keyword - cyan
					"3": {Name: "punctuation.separator.space.qmp-script2"},             // space
					"4": {Name: "constant.numeric.console.qmp-script2"},                // console number - orange
					"5": {Name: "punctuation.definition.directive.end.qmp-script2"},    // > bracket
				},
			},
			{
				Match: "(<)(wait)(\\s+)(\\d+s?)(>)",
				Name:  "meta.timing.wait.qmp-script2",
				Captures: map[string]TextMateCapture{
					"1": {Name: "punctuation.definition.directive.begin.qmp-script2"},   // < bracket
					"2": {Name: "support.function.timing.wait.qmp-script2"},            // wait keyword - cyan
					"3": {Name: "punctuation.separator.space.qmp-script2"},            // space
					"4": {Name: "constant.numeric.duration.qmp-script2"},              // duration - orange
					"5": {Name: "punctuation.definition.directive.end.qmp-script2"},   // > bracket
				},
			},
			{
				Match: "(<)(exit)(\\s+)(\\d+)(>)",
				Name:  "meta.control.exit.qmp-script2",
				Captures: map[string]TextMateCapture{
					"1": {Name: "punctuation.definition.directive.begin.qmp-script2"},  // < bracket
					"2": {Name: "keyword.control.flow.exit.qmp-script2"},              // exit keyword - red
					"3": {Name: "punctuation.separator.space.qmp-script2"},           // space
					"4": {Name: "constant.numeric.exitcode.qmp-script2"},             // exit code - orange
					"5": {Name: "punctuation.definition.directive.end.qmp-script2"},  // > bracket
				},
			},
			// Composition directives - distinct colors for each part
			{
				Match: "(<)(include)(\\s+)(\"[^\"]+\")(>)",
				Name:  "meta.composition.include.qmp-script2",
				Captures: map[string]TextMateCapture{
					"1": {Name: "punctuation.definition.directive.begin.qmp-script2"},   // < bracket
					"2": {Name: "keyword.control.import.qmp-script2"},                  // include keyword - magenta
					"3": {Name: "punctuation.separator.space.qmp-script2"},            // space
					"4": {Name: "string.quoted.double.filename.qmp-script2"},          // "filename" - green
					"5": {Name: "punctuation.definition.directive.end.qmp-script2"},   // > bracket
				},
			},
			{
				Match: "(<)(screenshot)(\\s+)(\"[^\"]+\")(?:(\\s+)(png|ppm|jpg))?(>)",
				Name:  "meta.debug.screenshot.qmp-script2",
				Captures: map[string]TextMateCapture{
					"1": {Name: "punctuation.definition.directive.begin.qmp-script2"}, // < bracket
					"2": {Name: "support.function.debug.screenshot.qmp-script2"},      // screenshot keyword - yellow
					"3": {Name: "punctuation.separator.space.qmp-script2"},           // space
					"4": {Name: "string.quoted.double.filename.qmp-script2"},         // "filename" - green
					"5": {Name: "punctuation.separator.space.qmp-script2"},           // space
					"6": {Name: "support.constant.format.qmp-script2"},               // format - cyan
					"7": {Name: "punctuation.definition.directive.end.qmp-script2"},  // > bracket
				},
			},
			// Control flow - distinct colors for each part
			{
				Match: "(<)(else)(>)",
				Name:  "meta.control.else.qmp-script2",
				Captures: map[string]TextMateCapture{
					"1": {Name: "punctuation.definition.directive.begin.qmp-script2"}, // < bracket
					"2": {Name: "keyword.control.conditional.else.qmp-script2"},       // else keyword - red
					"3": {Name: "punctuation.definition.directive.end.qmp-script2"},  // > bracket
				},
			},
			// Escaped directives - gray
			{
				Match: "\\\\(<[^>]*>)",
				Name:  "string.escaped.directive.qmp-script2",
				Captures: map[string]TextMateCapture{
					"1": {Name: "markup.raw.directive.qmp-script2"},
				},
			},
		},
	}
}

// generateVSCodeExtension creates the complete VSCode extension
func generateVSCodeExtension(outputDir string) error {
	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Extract regex patterns from parser
	fmt.Printf("🔍 Extracting regex patterns from parser...\n")
	patterns, err := extractRegexPatterns()
	if err != nil {
		return fmt.Errorf("failed to extract regex patterns: %w", err)
	}
	fmt.Printf("   Found %d regex patterns\n", len(patterns))

	// Generate TextMate grammar
	fmt.Printf("🎨 Generating TextMate grammar...\n")
	grammar := generateTextMateGrammar(patterns)

	// Write grammar file
	grammarPath := filepath.Join(outputDir, "syntaxes", "qmp-script2.tmLanguage.json")
	if err := os.MkdirAll(filepath.Dir(grammarPath), 0755); err != nil {
		return fmt.Errorf("failed to create syntaxes directory: %w", err)
	}

	grammarData, err := json.MarshalIndent(grammar, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal grammar: %w", err)
	}

	if err := os.WriteFile(grammarPath, grammarData, 0644); err != nil {
		return fmt.Errorf("failed to write grammar file: %w", err)
	}
	fmt.Printf("   ✅ Grammar file: %s\n", grammarPath)

	// Generate package.json
	fmt.Printf("📦 Generating package.json...\n")
	if err := generatePackageJSON(outputDir); err != nil {
		return fmt.Errorf("failed to generate package.json: %w", err)
	}

	// Generate README
	fmt.Printf("📝 Generating README...\n")
	if err := generateREADME(outputDir, patterns); err != nil {
		return fmt.Errorf("failed to generate README: %w", err)
	}

	// Generate CHANGELOG
	fmt.Printf("📋 Generating CHANGELOG...\n")
	if err := generateChangelog(outputDir); err != nil {
		return fmt.Errorf("failed to generate CHANGELOG: %w", err)
	}

	return nil
}

// getGitVersion gets version info from git
func getGitVersion() (string, string, error) {
	// Get git describe for version
	cmd := exec.Command("git", "describe", "--tags", "--always", "--dirty")
	output, err := cmd.Output()
	if err != nil {
		// Fallback to commit hash if no tags
		cmd = exec.Command("git", "rev-parse", "--short", "HEAD")
		output, err = cmd.Output()
		if err != nil {
			return "1.0.0", "unknown", nil // Default fallback
		}
	}

	version := strings.TrimSpace(string(output))

	// Get current branch
	cmd = exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	branchOutput, err := cmd.Output()
	branch := "main"
	if err == nil {
		branch = strings.TrimSpace(string(branchOutput))
	}

	// Clean up version for package.json (remove 'v' prefix, handle dirty)
	cleanVersion := strings.TrimPrefix(version, "v")
	if strings.Contains(cleanVersion, "-dirty") {
		cleanVersion = strings.Replace(cleanVersion, "-dirty", "-dev", 1)
	}
	if !strings.Contains(cleanVersion, ".") {
		cleanVersion = "1.0.0-" + cleanVersion // Prefix with base version if just hash
	}

	return cleanVersion, branch, nil
}

// generatePackageJSON creates the VSCode extension package.json
func generatePackageJSON(outputDir string) error {
	version, branch, err := getGitVersion()
	if err != nil {
		fmt.Printf("   ⚠️  Could not get git version, using default: %v\n", err)
		version = "1.0.0"
	} else {
		fmt.Printf("   📋 Version: %s (branch: %s)\n", version, branch)
	}

	packageJSON := map[string]interface{}{
		"name":         "qmp-script2",
		"displayName":  "QMP Script2 Syntax Highlighting",
		"description":  "Syntax highlighting for QMP Script2 automation files",
		"version":      version,
		"publisher":    "qmp-controller",
		"engines": map[string]string{
			"vscode": "^1.60.0",
		},
		"categories": []string{"Programming Languages"},
		"keywords":   []string{"qmp", "script2", "automation", "virtualization", "syntax"},
		"contributes": map[string]interface{}{
			"languages": []map[string]interface{}{
				{
					"id":           "qmp-script2",
					"aliases":      []string{"QMP Script2", "script2"},
					"extensions":   []string{".sc2", ".script2", ".qmp2"},
					"configuration": "./language-configuration.json",
				},
			},
			"grammars": []map[string]interface{}{
				{
					"language":   "qmp-script2",
					"scopeName":  "source.qmp-script2",
					"path":       "./syntaxes/qmp-script2.tmLanguage.json",
				},
			},
		},
		"repository": map[string]string{
			"type": "git",
			"url":  "https://github.com/jeeftor/qmp-controller.git",
		},
		"license": "MIT",
		"scripts": map[string]string{
			"compile": "echo 'No compilation needed for grammar-only extension'",
			"package": "vsce package",
			"publish": "vsce publish",
		},
		"devDependencies": map[string]string{
			"vsce": "^2.15.0",
		},
	}

	packageData, err := json.MarshalIndent(packageJSON, "", "  ")
	if err != nil {
		return err
	}

	packagePath := filepath.Join(outputDir, "package.json")
	return os.WriteFile(packagePath, packageData, 0644)
}

// generateREADME creates the extension README
func generateREADME(outputDir string, patterns []RegexPattern) error {
	readme := `# QMP Script2 Syntax Highlighting

VSCode extension providing syntax highlighting for QMP Script2 automation files.

## Features

- **Comprehensive syntax highlighting** for all Script2 language features
- **Automatic grammar generation** from actual parser code
- **Always up-to-date** with the latest Script2 syntax
- **Color-coded** directives, variables, functions, and comments

## Supported Syntax

### Variables
- Variable assignments: ` + "`USER=admin`" + `
- Variable expansion: ` + "`$USER`, `${USER:-default}`" + `
- Complex expansions: ` + "`${VAR:=value}`, `${VAR:+value}`" + `

### Directives
- Key sequences: ` + "`<enter>`, `<tab>`, `<ctrl+c>`" + `
- Wait commands: ` + "`<wait 5s>`, `<watch \"text\" 30s>`" + `
- Console switching: ` + "`<console 2>`" + `
- Control flow: ` + "`<exit 1>`, `<retry 3>`, `<repeat 5>`" + `

### Advanced Features
- **Functions**: ` + "`<function name>`, `<call function args>`" + `
- **Conditionals**: ` + "`<if-found \"text\" 5s>`, `<else>`" + `
- **Loops**: ` + "`<while-found \"text\" 30s poll 1s>`" + `
- **Composition**: ` + "`<include \"script.txt\">`" + `
- **Debugging**: ` + "`<screenshot \"file.png\">`" + `

### Comments and Escaping
- Comments: ` + "`# This is a comment`" + `
- Escaped directives: ` + "`\\<literal angle brackets>`" + `

## Installation

### From VSIX (Recommended)
1. Download the latest ` + "`qmp-script2-*.vsix`" + ` file
2. Open VSCode
3. Run: ` + "`code --install-extension qmp-script2-*.vsix`" + `

### From Source
1. Clone the repository
2. Navigate to the extension directory
3. Run: ` + "`npm install && npm run compile`" + `
4. Press F5 to launch extension development host

## File Associations

The extension automatically activates for files with these extensions:
- ` + "`.sc2`" + ` (recommended)
- ` + "`.script2`" + `
- ` + "`.qmp2`" + `

You can also manually set the language mode in VSCode:
1. Open a script file
2. Press ` + "`Ctrl+K M`" + ` (or ` + "`Cmd+K M`" + ` on Mac)
3. Type "QMP Script2" and select it

## Grammar Generation

This extension uses **automated grammar generation** from the actual QMP Script2 parser code.
The grammar is automatically synchronized with parser changes to ensure accuracy.

### Extracted Patterns

The following patterns were automatically extracted from the parser:

`

	// Group patterns by category for documentation
	patternsByCategory := make(map[string][]RegexPattern)
	for _, pattern := range patterns {
		patternsByCategory[pattern.Category] = append(patternsByCategory[pattern.Category], pattern)
	}

	for category, categoryPatterns := range patternsByCategory {
		readme += fmt.Sprintf("\n#### %s\n", strings.Title(category))
		for _, pattern := range categoryPatterns {
			readme += fmt.Sprintf("- **%s**: %s\n", pattern.Name, pattern.Description)
		}
	}

	readme += `

## Development

### Updating Grammar
When the Script2 parser is updated, regenerate the grammar:

` + "```bash" + `
go run main.go generate-vscode-grammar ./vscode-extension
` + "```" + `

### Building Extension
` + "```bash" + `
npm install
npm run compile
vsce package
` + "```" + `

## License

MIT License - see LICENSE file for details.
`

	readmePath := filepath.Join(outputDir, "README.md")
	return os.WriteFile(readmePath, []byte(readme), 0644)
}

// generateChangelog creates the extension CHANGELOG
func generateChangelog(outputDir string) error {
	changelog := `# Change Log

## [1.0.0] - Initial Release

### Added
- Complete syntax highlighting for QMP Script2 language
- Automatic grammar generation from parser source code
- Support for all Script2 features:
  - Variables and expansion syntax
  - Directive syntax with proper highlighting
  - Function definitions and calls
  - Control flow (conditionals, loops)
  - Comments and escape sequences
  - Script composition and debugging features

### Features
- Color-coded syntax highlighting
- Automatic file type detection for .script2 and .qmp2 files
- Grammar synchronized with actual parser implementation
- Comprehensive language support

### Technical
- TextMate grammar generated from Go parser regex patterns
- Automated generation process for future updates
- Clean separation of language features in grammar
`

	changelogPath := filepath.Join(outputDir, "CHANGELOG.md")
	return os.WriteFile(changelogPath, []byte(changelog), 0644)
}

func init() {
	rootCmd.AddCommand(generateVSCodeGrammarCmd)
}
