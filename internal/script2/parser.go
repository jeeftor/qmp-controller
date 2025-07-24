package script2

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Line classification patterns
var (
	// Variable assignment: USER=value or USER=${USER:-default}
	variableAssignmentRegex = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)

	// Directive patterns: <something> but not \<something>
	directiveRegex = regexp.MustCompile(`^<(.+)>$`)

	// Escaped directive patterns: \<something>
	escapedDirectiveRegex = regexp.MustCompile(`^\\<(.+)>$`)

	// Comment lines starting with #
	commentRegex = regexp.MustCompile(`^\s*#`)

	// Empty or whitespace-only lines
	emptyLineRegex = regexp.MustCompile(`^\s*$`)

	// Key sequence patterns
	keySequenceRegex = regexp.MustCompile(`^(enter|tab|space|escape|backspace|delete|up|down|left|right|home|end|page_up|page_down|ctrl\+[a-z]|alt\+[a-z]|shift\+[a-z]|f[1-9][0-9]?)$`)

	// Watch patterns
	watchRegex = regexp.MustCompile(`^watch\s+"([^"]+)"\s+(\d+s?)\s*(\|\||&&|$)`)

	// Console switching: console 2
	consoleRegex = regexp.MustCompile(`^console\s+([1-6])$`)

	// Wait delay: wait 5s
	waitRegex = regexp.MustCompile(`^wait\s+(\d+)s?$`)

	// Exit command: exit 1
	exitRegex = regexp.MustCompile(`^exit\s+(\d+)$`)

	// Loop directives
	retryRegex = regexp.MustCompile(`^retry\s+(\d+)$`)
	repeatRegex = regexp.MustCompile(`^repeat\s+(\d+)$`)
	whileFoundRegex = regexp.MustCompile(`^while-found\s+"([^"]+)"\s+(\d+s?)\s*(?:poll\s+(\d+(?:\.\d+)?s?))?$`)
	whileNotFoundRegex = regexp.MustCompile(`^while-not-found\s+"([^"]+)"\s+(\d+s?)\s*(?:poll\s+(\d+(?:\.\d+)?s?))?$`)

	// Additional directives
	elseRegex = regexp.MustCompile(`^else$`)
	includeRegex = regexp.MustCompile(`^include\s+"([^"]+)"$`)
	screenshotRegex = regexp.MustCompile(`^screenshot\s+"([^"]+)"(?:\s+(png|ppm|jpg))?$`)

	// Function directives
	functionDefRegex = regexp.MustCompile(`^function\s+([a-zA-Z_][a-zA-Z0-9_]*)$`)
	endFunctionRegex = regexp.MustCompile(`^end-function$`)
	functionCallRegex = regexp.MustCompile(`^call\s+([a-zA-Z_][a-zA-Z0-9_]*)(?:\s+(.+))?$`)
)

// ParseScript parses a complete script2 file
func (p *Parser) ParseScript(content string) (*Script, error) {
	lines := strings.Split(content, "\n")
	script := &Script{
		Variables:   make(map[string]string),
		Environment: make(map[string]string),
		Functions:   make(map[string]*Function),
		Lines:       make([]ParsedLine, 0, len(lines)),
	}

	// Initialize metadata
	script.Metadata.TotalLines = len(lines)
	script.Metadata.ParsedAt = time.Now()

	p.currentLine = 0

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		p.currentLine = i + 1

		parsedLine, err := p.ParseLine(line, i+1)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", i+1, err)
		}

		// Handle function definitions specially
		if parsedLine.Type == DirectiveLine && parsedLine.Directive != nil && parsedLine.Directive.Type == FunctionDef {
			// Parse function definition starting from next line
			function, nextIndex, err := p.parseFunctionDefinition(lines, i, parsedLine.Directive.FunctionName)
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", i+1, err)
			}

			// Add function to script
			script.Functions[function.Name] = function
			script.Metadata.FunctionLines += len(function.Lines)

			// Skip the lines we've already processed
			i = nextIndex - 1 // -1 because the for loop will increment
			continue // Don't add function definition lines to main script
		}

		// Handle directives that require block parsing
		if parsedLine.Type == DirectiveLine && parsedLine.Directive != nil {
			directiveType := parsedLine.Directive.Type
			if directiveType == ConditionalIfFound || directiveType == ConditionalIfNotFound ||
			   directiveType == Retry || directiveType == Repeat ||
			   directiveType == WhileFound || directiveType == WhileNotFound {
				// Parse block starting from next line
				blockLines, elseBlock, nextIndex, err := p.parseDirectiveBlockWithElse(lines, i+1, directiveType)
				if err != nil {
					return nil, fmt.Errorf("line %d: %w", i+1, err)
				}

				// Set the block and optional else block in the directive
				parsedLine.Directive.Block = blockLines
				if len(elseBlock) > 0 {
					parsedLine.Directive.ElseBlock = elseBlock
				}

				// Skip the lines we've already processed
				i = nextIndex - 1 // -1 because the for loop will increment
			}
		}

		// Update metadata counters
		switch parsedLine.Type {
		case TextLine:
			script.Metadata.TextLines++
		case DirectiveLine, ConditionalLine:
			script.Metadata.DirectiveLines++
		case VariableLine:
			script.Metadata.VariableLines++
		}

		// Handle variable assignments
		if parsedLine.Type == VariableLine && parsedLine.Variables != nil {
			for name, value := range parsedLine.Variables {
				script.Variables[name] = value
				p.variables.Set(name, value)
			}
		}

		script.Lines = append(script.Lines, parsedLine)
	}

	// Extract all variable names used in the script
	script.Metadata.Variables = p.extractAllVariables(script)

	// Extract function names
	script.Metadata.Functions = make([]string, 0, len(script.Functions))
	for funcName := range script.Functions {
		script.Metadata.Functions = append(script.Metadata.Functions, funcName)
	}

	return script, nil
}

// ParseLine parses a single line and classifies it
func (p *Parser) ParseLine(line string, lineNumber int) (ParsedLine, error) {
	originalLine := line
	trimmedLine := strings.TrimSpace(line)

	// Calculate indentation
	indent := len(line) - len(strings.TrimLeft(line, " \t"))

	// Create base parsed line
	parsedLine := ParsedLine{
		LineNumber:   lineNumber,
		OriginalText: originalLine,
		Content:      trimmedLine,
		Indent:       indent,
	}

	// Classify the line
	lineType := p.classifyLine(trimmedLine)
	parsedLine.Type = lineType

	// Parse based on type
	switch lineType {
	case VariableLine:
		err := p.parseVariableLine(&parsedLine)
		if err != nil {
			return parsedLine, err
		}

	case DirectiveLine, ConditionalLine:
		err := p.parseDirectiveLine(&parsedLine)
		if err != nil {
			return parsedLine, err
		}

	case TextLine:
		// Handle escaped directives by removing the backslash
		text := trimmedLine
		if escapedDirectiveRegex.MatchString(trimmedLine) {
			text = strings.TrimPrefix(trimmedLine, "\\")
		}

		// Expand variables in text lines
		expanded, err := p.variables.Expand(text)
		if err != nil {
			return parsedLine, fmt.Errorf("variable expansion failed: %w", err)
		}
		parsedLine.ExpandedText = expanded

	case CommentLine, EmptyLine:
		// No special processing needed
		parsedLine.ExpandedText = trimmedLine
	}

	return parsedLine, nil
}

// classifyLine determines the type of a script line
func (p *Parser) classifyLine(line string) LineType {
	if emptyLineRegex.MatchString(line) {
		return EmptyLine
	}

	if commentRegex.MatchString(line) {
		return CommentLine
	}

	if variableAssignmentRegex.MatchString(line) {
		return VariableLine
	}

	// Check for escaped directives first (they become text)
	if escapedDirectiveRegex.MatchString(line) {
		return TextLine
	}

	if directiveRegex.MatchString(line) {
		// Check if it's a conditional directive
		if strings.Contains(line, "&&") || strings.Contains(line, "||") ||
		   strings.Contains(line, "if-found") || strings.Contains(line, "if-not-found") {
			return ConditionalLine
		}
		return DirectiveLine
	}

	return TextLine
}

// parseVariableLine parses a variable assignment line
func (p *Parser) parseVariableLine(parsedLine *ParsedLine) error {
	name, value, isAssignment := p.variables.ParseAssignment(parsedLine.Content)
	if !isAssignment {
		return fmt.Errorf("invalid variable assignment: %s", parsedLine.Content)
	}

	parsedLine.Variables = map[string]string{name: value}
	parsedLine.ExpandedText = fmt.Sprintf("%s=%s", name, value)

	return nil
}

// parseDirectiveLine parses a directive line
func (p *Parser) parseDirectiveLine(parsedLine *ParsedLine) error {
	matches := directiveRegex.FindStringSubmatch(parsedLine.Content)
	if len(matches) < 2 {
		return fmt.Errorf("invalid directive format: %s", parsedLine.Content)
	}

	directiveContent := strings.TrimSpace(matches[1])
	directive := &Directive{
		Command: directiveContent,
	}

	// Parse different directive types
	if err := p.parseDirectiveContent(directive, directiveContent); err != nil {
		return fmt.Errorf("failed to parse directive: %w", err)
	}

	parsedLine.Directive = directive

	// Expand variables in the directive if applicable
	if directive.SearchText != "" {
		expanded, err := p.variables.Expand(directive.SearchText)
		if err != nil {
			return fmt.Errorf("variable expansion in directive failed: %w", err)
		}
		directive.SearchText = expanded
	}

	parsedLine.ExpandedText = fmt.Sprintf("<%s>", directive.Command)

	return nil
}

// parseDirectiveContent parses the content of a directive
func (p *Parser) parseDirectiveContent(directive *Directive, content string) error {
	// Key sequences: enter, tab, ctrl+c, etc.
	if keySequenceRegex.MatchString(content) {
		directive.Type = KeySequence
		directive.KeyName = content
		return nil
	}

	// Watch commands: watch "text" 30s
	if matches := watchRegex.FindStringSubmatch(content); len(matches) >= 3 {
		directive.Type = Watch
		directive.SearchText = matches[1]

		// Parse timeout
		timeoutStr := matches[2]
		if !strings.HasSuffix(timeoutStr, "s") {
			timeoutStr += "s"
		}

		timeout, err := time.ParseDuration(timeoutStr)
		if err != nil {
			return fmt.Errorf("invalid timeout format: %s", matches[2])
		}
		directive.Timeout = timeout

		// Check for conditional operators
		if len(matches) > 3 && matches[3] != "" {
			if matches[3] == "||" || matches[3] == "&&" {
				directive.Type = ConditionalWatch
				directive.Condition = matches[3]
			}
		}

		return nil
	}

	// Console switching: console 2
	if matches := consoleRegex.FindStringSubmatch(content); len(matches) >= 2 {
		directive.Type = Console
		consoleNum, err := strconv.Atoi(matches[1])
		if err != nil {
			return fmt.Errorf("invalid console number: %s", matches[1])
		}
		directive.ConsoleNum = consoleNum
		return nil
	}

	// Wait delay: wait 5s
	if matches := waitRegex.FindStringSubmatch(content); len(matches) >= 2 {
		directive.Type = WaitDelay
		seconds, err := strconv.Atoi(matches[1])
		if err != nil {
			return fmt.Errorf("invalid wait duration: %s", matches[1])
		}
		directive.Timeout = time.Duration(seconds) * time.Second
		return nil
	}

	// Exit command: exit 1
	if matches := exitRegex.FindStringSubmatch(content); len(matches) >= 2 {
		directive.Type = Exit
		exitCode, err := strconv.Atoi(matches[1])
		if err != nil {
			return fmt.Errorf("invalid exit code: %s", matches[1])
		}
		directive.ExitCode = exitCode
		return nil
	}

	// Conditional directives
	if strings.HasPrefix(content, "if-found") {
		directive.Type = ConditionalIfFound
		return p.parseConditionalDirective(directive, content)
	}

	if strings.HasPrefix(content, "if-not-found") {
		directive.Type = ConditionalIfNotFound
		return p.parseConditionalDirective(directive, content)
	}

	// Retry directive: retry 3
	if matches := retryRegex.FindStringSubmatch(content); len(matches) >= 2 {
		directive.Type = Retry
		retryCount, err := strconv.Atoi(matches[1])
		if err != nil {
			return fmt.Errorf("invalid retry count: %s", matches[1])
		}
		directive.RetryCount = retryCount
		return nil
	}

	// Repeat directive: repeat 5
	if matches := repeatRegex.FindStringSubmatch(content); len(matches) >= 2 {
		directive.Type = Repeat
		repeatCount, err := strconv.Atoi(matches[1])
		if err != nil {
			return fmt.Errorf("invalid repeat count: %s", matches[1])
		}
		directive.RepeatCount = repeatCount
		return nil
	}

	// While-found directive: while-found "text" 30s poll 1s
	if matches := whileFoundRegex.FindStringSubmatch(content); len(matches) >= 3 {
		directive.Type = WhileFound
		directive.SearchText = matches[1]

		// Parse timeout
		timeoutStr := matches[2]
		if !strings.HasSuffix(timeoutStr, "s") {
			timeoutStr += "s"
		}
		timeout, err := time.ParseDuration(timeoutStr)
		if err != nil {
			return fmt.Errorf("invalid timeout in while-found: %s", matches[2])
		}
		directive.Timeout = timeout

		// Parse optional poll interval
		if len(matches) > 3 && matches[3] != "" {
			pollStr := matches[3]
			if !strings.HasSuffix(pollStr, "s") {
				pollStr += "s"
			}
			poll, err := time.ParseDuration(pollStr)
			if err != nil {
				return fmt.Errorf("invalid poll interval in while-found: %s", matches[3])
			}
			directive.PollInterval = poll
		} else {
			directive.PollInterval = 1 * time.Second // Default poll interval
		}
		return nil
	}

	// While-not-found directive: while-not-found "text" 30s poll 1s
	if matches := whileNotFoundRegex.FindStringSubmatch(content); len(matches) >= 3 {
		directive.Type = WhileNotFound
		directive.SearchText = matches[1]

		// Parse timeout
		timeoutStr := matches[2]
		if !strings.HasSuffix(timeoutStr, "s") {
			timeoutStr += "s"
		}
		timeout, err := time.ParseDuration(timeoutStr)
		if err != nil {
			return fmt.Errorf("invalid timeout in while-not-found: %s", matches[2])
		}
		directive.Timeout = timeout

		// Parse optional poll interval
		if len(matches) > 3 && matches[3] != "" {
			pollStr := matches[3]
			if !strings.HasSuffix(pollStr, "s") {
				pollStr += "s"
			}
			poll, err := time.ParseDuration(pollStr)
			if err != nil {
				return fmt.Errorf("invalid poll interval in while-not-found: %s", matches[3])
			}
			directive.PollInterval = poll
		} else {
			directive.PollInterval = 1 * time.Second // Default poll interval
		}
		return nil
	}

	// Else directive: else
	if elseRegex.MatchString(content) {
		directive.Type = Else
		return nil
	}

	// Include directive: include "script.txt"
	if matches := includeRegex.FindStringSubmatch(content); len(matches) >= 2 {
		directive.Type = Include
		directive.IncludePath = matches[1]
		return nil
	}

	// Screenshot directive: screenshot "filename.png" [format]
	if matches := screenshotRegex.FindStringSubmatch(content); len(matches) >= 2 {
		directive.Type = Screenshot
		directive.ScreenshotPath = matches[1]

		// Set format based on extension or explicit format
		if len(matches) > 2 && matches[2] != "" {
			directive.ScreenshotFormat = matches[2]
		} else {
			// Auto-detect format from file extension
			if strings.HasSuffix(strings.ToLower(directive.ScreenshotPath), ".png") {
				directive.ScreenshotFormat = "png"
			} else if strings.HasSuffix(strings.ToLower(directive.ScreenshotPath), ".jpg") ||
					  strings.HasSuffix(strings.ToLower(directive.ScreenshotPath), ".jpeg") {
				directive.ScreenshotFormat = "jpg"
			} else {
				directive.ScreenshotFormat = "ppm" // Default format
			}
		}
		return nil
	}

	// Function definition: function name
	if matches := functionDefRegex.FindStringSubmatch(content); len(matches) >= 2 {
		directive.Type = FunctionDef
		directive.FunctionName = matches[1]
		return nil
	}

	// End function: end-function
	if endFunctionRegex.MatchString(content) {
		directive.Type = EndFunction
		return nil
	}

	// Function call: call function_name args...
	if matches := functionCallRegex.FindStringSubmatch(content); len(matches) >= 2 {
		directive.Type = FunctionCall
		directive.FunctionName = matches[1]

		// Parse arguments if present
		if len(matches) > 2 && matches[2] != "" {
			// Split arguments on whitespace, respecting quotes
			directive.FunctionArgs = parseArguments(matches[2])
		} else {
			directive.FunctionArgs = []string{}
		}
		return nil
	}

	return fmt.Errorf("unknown directive type: %s", content)
}

// parseConditionalDirective parses conditional directives
func (p *Parser) parseConditionalDirective(directive *Directive, content string) error {
	// Parse if-found "text" 5s or if-not-found "text" 5s
	parts := strings.Fields(content)
	if len(parts) < 3 {
		return fmt.Errorf("invalid conditional directive format: %s", content)
	}

	// Extract search text (should be quoted)
	if !strings.HasPrefix(parts[1], "\"") || !strings.HasSuffix(parts[1], "\"") {
		return fmt.Errorf("search text must be quoted in conditional directive: %s", content)
	}

	directive.SearchText = strings.Trim(parts[1], "\"")

	// Parse timeout
	if len(parts) >= 3 {
		timeoutStr := parts[2]
		if !strings.HasSuffix(timeoutStr, "s") {
			timeoutStr += "s"
		}

		timeout, err := time.ParseDuration(timeoutStr)
		if err != nil {
			return fmt.Errorf("invalid timeout in conditional directive: %s", parts[2])
		}
		directive.Timeout = timeout
	}

	return nil
}

// extractAllVariables extracts all variable names used in the script
func (p *Parser) extractAllVariables(script *Script) []string {
	variableSet := make(map[string]bool)

	for _, line := range script.Lines {
		// Get variables from original text
		vars := GetUsedVariables(line.OriginalText)
		for _, v := range vars {
			variableSet[v] = true
		}

		// Get variables from directive search text
		if line.Directive != nil && line.Directive.SearchText != "" {
			vars := GetUsedVariables(line.Directive.SearchText)
			for _, v := range vars {
				variableSet[v] = true
			}
		}
	}

	// Convert to sorted slice
	variables := make([]string, 0, len(variableSet))
	for v := range variableSet {
		variables = append(variables, v)
	}

	return variables
}

// ValidateScript performs basic validation on a parsed script
func (p *Parser) ValidateScript(script *Script) ValidationResult {
	result := ValidationResult{
		Valid:      true,
		Variables:  script.Metadata.Variables,
		Directives: make([]string, 0),
	}

	directiveSet := make(map[string]bool)

	for _, line := range script.Lines {
		// Collect directive types
		if line.Directive != nil {
			directiveType := line.Directive.Type.String()
			if !directiveSet[directiveType] {
				result.Directives = append(result.Directives, directiveType)
				directiveSet[directiveType] = true
			}
		}

		// Validate directive syntax
		if line.Type == DirectiveLine || line.Type == ConditionalLine {
			if line.Directive == nil {
				result.Valid = false
				result.Errors = append(result.Errors, ValidationError{
					LineNumber: line.LineNumber,
					Type:       "error",
					Message:    "Directive line missing directive data",
					Suggestion: "Check directive syntax",
				})
			}
		}

		// Check for undefined variables (warnings)
		vars := GetUsedVariables(line.OriginalText)
		for _, varName := range vars {
			if _, exists := p.variables.Get(varName); !exists {
				result.Warnings = append(result.Warnings, ValidationError{
					LineNumber: line.LineNumber,
					Type:       "warning",
					Message:    fmt.Sprintf("Variable '%s' is not defined", varName),
					Suggestion: fmt.Sprintf("Define '%s' or set it via --var %s=value", varName, varName),
				})
			}
		}
	}

	return result
}

// parseDirectiveBlock parses lines following a directive that contains a block
func (p *Parser) parseDirectiveBlock(lines []string, startIndex int, directiveType DirectiveType) ([]ParsedLine, int, error) {
	var blockLines []ParsedLine
	i := startIndex

	// Parse lines until we hit another directive, empty line, or end of script
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])

		// Stop at empty lines or comments (these end the conditional block)
		if line == "" || strings.HasPrefix(line, "#") {
			break
		}

		// Stop at directives (these end the conditional block)
		if strings.HasPrefix(line, "<") && strings.HasSuffix(line, ">") {
			break
		}

		// Parse this line as part of the conditional block
		parsedLine, err := p.ParseLine(lines[i], i+1)
		if err != nil {
			return nil, i, fmt.Errorf("error parsing conditional block line %d: %w", i+1, err)
		}

		// Only include text and variable lines in conditional blocks
		// Skip empty lines and comments within blocks
		if parsedLine.Type == TextLine || parsedLine.Type == VariableLine {
			blockLines = append(blockLines, parsedLine)
		}

		i++
	}

	return blockLines, i, nil
}

// parseDirectiveBlockWithElse parses lines following a conditional directive, including optional else block
func (p *Parser) parseDirectiveBlockWithElse(lines []string, startIndex int, directiveType DirectiveType) ([]ParsedLine, []ParsedLine, int, error) {
	var blockLines []ParsedLine
	var elseBlockLines []ParsedLine
	i := startIndex
	foundElse := false

	// Parse lines until we hit another directive, empty line, or end of script
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])

		// Stop at empty lines or comments (these end the conditional block)
		if line == "" || strings.HasPrefix(line, "#") {
			break
		}

		// Check for else directive
		if line == "<else>" {
			if directiveType != ConditionalIfFound && directiveType != ConditionalIfNotFound {
				return nil, nil, i, fmt.Errorf("else directive only allowed after if-found or if-not-found")
			}
			foundElse = true
			i++ // Skip the else line
			continue
		}

		// Stop at other directives (these end the conditional block)
		if strings.HasPrefix(line, "<") && strings.HasSuffix(line, ">") {
			break
		}

		// Parse this line as part of the block
		parsedLine, err := p.ParseLine(lines[i], i+1)
		if err != nil {
			return nil, nil, i, fmt.Errorf("error parsing block line %d: %w", i+1, err)
		}

		// Only include text and variable lines in blocks
		if parsedLine.Type == TextLine || parsedLine.Type == VariableLine {
			if foundElse {
				elseBlockLines = append(elseBlockLines, parsedLine)
			} else {
				blockLines = append(blockLines, parsedLine)
			}
		}

		i++
	}

	return blockLines, elseBlockLines, i, nil
}

// parseFunctionDefinition parses a function definition and its body
func (p *Parser) parseFunctionDefinition(lines []string, startIndex int, functionName string) (*Function, int, error) {
	function := &Function{
		Name:      functionName,
		Lines:     make([]ParsedLine, 0),
		Defined:   true,
		LineStart: startIndex + 1,
	}

	i := startIndex + 1 // Skip the function definition line

	// Parse function body until end-function
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])

		// Check for end-function
		if line == "<end-function>" {
			function.LineEnd = i + 1
			return function, i + 1, nil // Return next index after end-function
		}

		// Skip empty lines and comments in function body
		if line == "" || strings.HasPrefix(line, "#") {
			i++
			continue
		}

		// Parse the line as part of the function body
		parsedLine, err := p.ParseLine(lines[i], i+1)
		if err != nil {
			return nil, i, fmt.Errorf("error parsing function body line %d: %w", i+1, err)
		}

		// Only include meaningful lines in function body
		if parsedLine.Type != EmptyLine && parsedLine.Type != CommentLine {
			function.Lines = append(function.Lines, parsedLine)
		}

		i++
	}

	return nil, i, fmt.Errorf("function '%s' missing end-function", functionName)
}

// parseArguments parses function call arguments, respecting quotes
func parseArguments(argString string) []string {
	var args []string
	var current strings.Builder
	inQuotes := false
	escaped := false

	for _, char := range argString {
		switch char {
		case '\\':
			if escaped {
				current.WriteRune('\\')
				escaped = false
			} else {
				escaped = true
			}
		case '"':
			if escaped {
				current.WriteRune('"')
				escaped = false
			} else {
				inQuotes = !inQuotes
			}
		case ' ', '\t':
			if escaped {
				current.WriteRune(char)
				escaped = false
			} else if inQuotes {
				current.WriteRune(char)
			} else if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			if escaped {
				current.WriteRune('\\')
				escaped = false
			}
			current.WriteRune(char)
		}
	}

	if current.Len() > 0 {
		args = append(args, current.String())
	}

	return args
}
