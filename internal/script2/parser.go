package script2

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// parseDuration parses a duration string like "5s" or "30" (seconds implied)
func parseDuration(s string) (time.Duration, error) {
	// If the string already has a time unit, parse it directly
	if strings.HasSuffix(s, "s") || strings.HasSuffix(s, "m") || strings.HasSuffix(s, "h") {
		return time.ParseDuration(s)
	}

	// Otherwise, assume seconds
	seconds, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid duration: %s", s)
	}

	return time.Duration(seconds) * time.Second, nil
}

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

	// Switch-case directives
	switchRegex = regexp.MustCompile(`^switch(?:\s+timeout=(\d+(?:\.\d+)?s?))?(?:\s+poll=(\d+(?:\.\d+)?s?))?$`)
	caseRegex = regexp.MustCompile(`^case\s+"([^"]+)"$`)
	defaultRegex = regexp.MustCompile(`^default$`)
	endCaseRegex = regexp.MustCompile(`^end-case$`)
	endSwitchRegex = regexp.MustCompile(`^end-switch$`)
	endIfRegex = regexp.MustCompile(`^end-if$`)
	returnRegex = regexp.MustCompile(`^return$`)
	includeRegex = regexp.MustCompile(`^include\s+"([^"]+)"$`)
	screenshotRegex = regexp.MustCompile(`^screenshot\s+"([^"]+)"(?:\s+(png|ppm|jpg))?$`)

	// Function directives
	functionDefRegex = regexp.MustCompile(`^function\s+([a-zA-Z_][a-zA-Z0-9_]*)$`)
	endFunctionRegex = regexp.MustCompile(`^end-function$`)
	functionCallRegex = regexp.MustCompile(`^call\s+([a-zA-Z_][a-zA-Z0-9_]*)(?:\s+(.+))?$`)

	// Set directive
	setRegex = regexp.MustCompile(`^set\s+([a-zA-Z_][a-zA-Z0-9_]*)="([^"]*)"$`)
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
			nextIndex, err := p.parseDirectiveBlocks(lines, i, &parsedLine)
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", i+1, err)
			}
			if nextIndex > i {
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

	// End-if directive
	if matches := endIfRegex.FindStringSubmatch(content); len(matches) > 0 {
		directive.Type = EndIf
		return nil
	}

	// Return directive
	if matches := returnRegex.FindStringSubmatch(content); len(matches) > 0 {
		directive.Type = Return
		return nil
	}

	// Switch directive
	if matches := switchRegex.FindStringSubmatch(content); len(matches) > 0 {
		directive.Type = Switch

		// Parse timeout if provided
		if matches[1] != "" {
			timeout, err := parseDuration(matches[1])
			if err != nil {
				return fmt.Errorf("invalid timeout in switch directive: %s", matches[1])
			}
			directive.Timeout = timeout
		} else {
			// Default timeout: 30s
			directive.Timeout = 30 * time.Second
		}

		// Parse poll interval if provided
		if matches[2] != "" {
			pollInterval, err := parseDuration(matches[2])
			if err != nil {
				return fmt.Errorf("invalid poll interval in switch directive: %s", matches[2])
			}
			directive.PollInterval = pollInterval
		} else {
			// Default poll interval: 1s
			directive.PollInterval = 1 * time.Second
		}

		return nil
	}

	// Case directive
	if matches := caseRegex.FindStringSubmatch(content); len(matches) > 0 {
		directive.Type = Case
		directive.SearchText = matches[1]
		return nil
	}

	// Default directive
	if matches := defaultRegex.FindStringSubmatch(content); len(matches) > 0 {
		directive.Type = Default
		return nil
	}

	// End-case directive
	if matches := endCaseRegex.FindStringSubmatch(content); len(matches) > 0 {
		directive.Type = EndCase
		return nil
	}

	// End-switch directive
	if matches := endSwitchRegex.FindStringSubmatch(content); len(matches) > 0 {
		directive.Type = EndSwitch
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

	// Break directive: break
	if strings.TrimSpace(strings.ToLower(content)) == "break" {
		directive.Type = Break
		return nil
	}

	// Return directive: return
	if returnRegex.MatchString(content) {
		directive.Type = Return
		return nil
	}

	// Set directive: set variable="value"
	if matches := setRegex.FindStringSubmatch(content); len(matches) >= 3 {
		directive.Type = Set
		directive.VariableName = matches[1]
		directive.VariableValue = matches[2]
		return nil
	}

	return fmt.Errorf("unknown directive type: %s", content)
}

// parseConditionalDirective parses conditional directives
func (p *Parser) parseConditionalDirective(directive *Directive, content string) error {
	// Extract the quoted search text first
	quoteStart := strings.Index(content, "\"")
	if quoteStart == -1 {
		return fmt.Errorf("search text must be quoted in conditional directive: %s", content)
	}

	quoteEnd := strings.Index(content[quoteStart+1:], "\"")
	if quoteEnd == -1 {
		return fmt.Errorf("missing closing quote for search text in conditional directive: %s", content)
	}
	quoteEnd += quoteStart + 1 // Adjust for the offset in the substring

	// Extract the search text
	directive.SearchText = content[quoteStart+1 : quoteEnd]

	// Get the remaining content after the quoted text
	remaining := strings.TrimSpace(content[quoteEnd+1:])

	// Parse timeout if present
	if remaining != "" {
		timeoutStr := remaining
		if !strings.HasSuffix(timeoutStr, "s") {
			timeoutStr += "s"
		}

		timeout, err := time.ParseDuration(timeoutStr)
		if err != nil {
			return fmt.Errorf("invalid timeout in conditional directive: %s", remaining)
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
	foundEndIf := false

	// Parse lines until we hit end-if directive, empty line, or end of script
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])

		// Check for end-if directive
		if line == "<end-if>" {
			// Parse this line to get the directive
			_, err := p.ParseLine(lines[i], i+1)
			if err != nil {
				return nil, i, fmt.Errorf("error parsing end-if directive at line %d: %w", i+1, err)
			}
			foundEndIf = true
			i++ // Skip the end-if line
			break // End of conditional block
		}

		// Stop at empty lines or comments only if we don't have explicit end-if markers
		if !foundEndIf && (line == "" || strings.HasPrefix(line, "#")) {
			break
		}

		// Stop at other directives only if we don't have explicit end-if markers
		if !foundEndIf && strings.HasPrefix(line, "<") && strings.HasSuffix(line, ">") && !strings.Contains(line, "<end-if>") {
			break
		}

		// Parse this line as part of the conditional block
		parsedLine, err := p.ParseLine(lines[i], i+1)
		if err != nil {
			return nil, i, fmt.Errorf("error parsing conditional block line %d: %w", i+1, err)
		}

		// Include all line types in blocks except comments and empty lines
		if parsedLine.Type != CommentLine && parsedLine.Type != EmptyLine {
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
	foundEndIf := false

	// Parse lines until we hit end-if directive, empty line, or end of script
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])

		// Check for end-if directive
		if line == "<end-if>" {
			// Parse this line to get the directive
			_, err := p.ParseLine(lines[i], i+1)
			if err != nil {
				return nil, nil, i, fmt.Errorf("error parsing end-if directive at line %d: %w", i+1, err)
			}
			foundEndIf = true
			i++ // Skip the end-if line
			break // End of conditional block
		}

		// Stop at empty lines or comments only if we don't have explicit end-if markers
		if !foundEndIf && (line == "" || strings.HasPrefix(line, "#")) {
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

		// Stop at other directives only if we don't have explicit end-if markers
		if !foundEndIf && strings.HasPrefix(line, "<") && strings.HasSuffix(line, ">") && !strings.Contains(line, "<end-if>") {
			break
		}

		// Parse this line as part of the block
		parsedLine, err := p.ParseLine(lines[i], i+1)
		if err != nil {
			return nil, nil, i, fmt.Errorf("error parsing block line %d: %w", i+1, err)
		}

		// Include all line types in blocks except comments and empty lines
		if parsedLine.Type != CommentLine && parsedLine.Type != EmptyLine {
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

// parseSwitchBlock parses a switch statement and its cases
func (p *Parser) parseSwitchBlock(lines []string, startIndex int) ([]SwitchCase, []ParsedLine, int, error) {
	cases := make([]SwitchCase, 0)
	var defaultCase []ParsedLine

	i := startIndex + 1 // Skip the switch line

	var currentCase *SwitchCase
	inDefaultCase := false

	// Parse switch body until end-switch
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])

		// Check for end-switch
		if line == "<end-switch>" {
			// Parse this line to get the directive
			_, err := p.ParseLine(lines[i], i+1)
			if err != nil {
				return nil, nil, i, fmt.Errorf("error parsing end-switch directive at line %d: %w", i+1, err)
			}
			i++ // Skip the end-switch line
			break // End of switch block
		}

		// Check for case directive
		if strings.HasPrefix(line, "<case ") && strings.HasSuffix(line, ">") {
			// If we were in a case, finalize it
			if currentCase != nil {
				cases = append(cases, *currentCase)
				currentCase = nil
			}

			// Reset default case flag
			inDefaultCase = false

			// Parse the case directive
			parsedLine, err := p.ParseLine(lines[i], i+1)
			if err != nil {
				return nil, nil, i, fmt.Errorf("error parsing case directive at line %d: %w", i+1, err)
			}

			// Create a new case
			currentCase = &SwitchCase{
				SearchText: parsedLine.Directive.SearchText,
				Lines:      make([]ParsedLine, 0),
				LineStart:  i + 1,
			}

			i++ // Skip the case line
			continue
		}

		// Check for default directive
		if line == "<default>" {
			// If we were in a case, finalize it
			if currentCase != nil {
				cases = append(cases, *currentCase)
				currentCase = nil
			}

			// Set default case flag
			inDefaultCase = true

			// Parse the default directive
			_, err := p.ParseLine(lines[i], i+1)
			if err != nil {
				return nil, nil, i, fmt.Errorf("error parsing default directive at line %d: %w", i+1, err)
			}

			i++ // Skip the default line
			continue
		}

		// Check for end-case directive
		if line == "<end-case>" {
			// Parse this line to get the directive
			_, err := p.ParseLine(lines[i], i+1)
			if err != nil {
				return nil, nil, i, fmt.Errorf("error parsing end-case directive at line %d: %w", i+1, err)
			}

			// If we were in a case, finalize it and set the end line
			if currentCase != nil {
				currentCase.LineEnd = i + 1
				cases = append(cases, *currentCase)
				currentCase = nil
			}

			// Reset default case flag
			inDefaultCase = false

			i++ // Skip the end-case line
			continue
		}

		// Parse this line as part of the current case or default
		parsedLine, err := p.ParseLine(lines[i], i+1)
		if err != nil {
			return nil, nil, i, fmt.Errorf("error parsing switch block line %d: %w", i+1, err)
		}

		// Include all line types except comments and empty lines
		if parsedLine.Type != CommentLine && parsedLine.Type != EmptyLine {
			if inDefaultCase {
				defaultCase = append(defaultCase, parsedLine)
			} else if currentCase != nil {
				currentCase.Lines = append(currentCase.Lines, parsedLine)
			} else {
				return nil, nil, i, fmt.Errorf("line %d outside of any case or default block", i+1)
			}
		}

		i++
	}

	// If we were still in a case at the end, finalize it
	if currentCase != nil {
		cases = append(cases, *currentCase)
	}

	return cases, defaultCase, i, nil
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

		// Handle directives that require block parsing
		if parsedLine.Type == DirectiveLine && parsedLine.Directive != nil {
			nextIndex, err := p.parseDirectiveBlocks(lines, i, &parsedLine)
			if err != nil {
				return nil, i, fmt.Errorf("line %d: %w", i+1, err)
			}
			if nextIndex > i {
				i = nextIndex - 1 // -1 because the for loop will increment
			}
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

// parseDirectiveBlocks handles block parsing for directives that contain nested content
// This centralizes the logic for parsing conditional blocks, switch statements, etc.
func (p *Parser) parseDirectiveBlocks(lines []string, currentIndex int, parsedLine *ParsedLine) (int, error) {
	directive := parsedLine.Directive
	directiveType := directive.Type

	if directiveType == ConditionalIfFound || directiveType == ConditionalIfNotFound ||
	   directiveType == Retry || directiveType == Repeat ||
	   directiveType == WhileFound || directiveType == WhileNotFound {
		// Parse block starting from next line
		blockLines, elseBlock, nextIndex, err := p.parseDirectiveBlockWithElse(lines, currentIndex+1, directiveType)
		if err != nil {
			return currentIndex, err
		}

		// Set the block and optional else block in the directive
		directive.Block = blockLines
		if len(elseBlock) > 0 {
			directive.ElseBlock = elseBlock
		}

		return nextIndex, nil

	} else if directiveType == Switch {
		// Parse switch block starting from current line
		cases, defaultCase, nextIndex, err := p.parseSwitchBlock(lines, currentIndex)
		if err != nil {
			return currentIndex, err
		}

		// Set the cases and default case in the directive
		directive.Cases = cases
		if len(defaultCase) > 0 {
			directive.DefaultCase = defaultCase
		}

		return nextIndex, nil
	}

	// No block parsing needed for this directive type
	return currentIndex, nil
}
