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

	// Directive patterns: <something>
	directiveRegex = regexp.MustCompile(`^<(.+)>$`)

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
)

// ParseScript parses a complete script2 file
func (p *Parser) ParseScript(content string) (*Script, error) {
	lines := strings.Split(content, "\n")
	script := &Script{
		Variables:   make(map[string]string),
		Environment: make(map[string]string),
		Lines:       make([]ParsedLine, 0, len(lines)),
	}

	// Initialize metadata
	script.Metadata.TotalLines = len(lines)
	script.Metadata.ParsedAt = time.Now()

	p.currentLine = 0

	for i, line := range lines {
		p.currentLine = i + 1

		parsedLine, err := p.ParseLine(line, i+1)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", i+1, err)
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
		// Expand variables in text lines
		expanded, err := p.variables.Expand(trimmedLine)
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
