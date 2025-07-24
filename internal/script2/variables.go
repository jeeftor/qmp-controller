package script2

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Variable expansion patterns (bash-compatible)
var (
	// $VAR or ${VAR}
	simpleVarPattern = regexp.MustCompile(`\$([A-Za-z_][A-Za-z0-9_]*)|\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

	// ${VAR:-default} - use default if VAR is unset or empty
	defaultPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*):-([^}]*)\}`)

	// ${VAR:=default} - set VAR to default if unset or empty, then use value
	assignPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*):=([^}]*)\}`)

	// ${VAR:+value} - use value if VAR is set and non-empty
	conditionalPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*):+([^}]*)\}`)

	// Variable assignment pattern: VAR=value or VAR=${...}
	assignmentPattern = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)=(.*)$`)
)

// Get retrieves a variable value with precedence: overrides > variables > environment
func (ve *VariableExpander) Get(name string) (string, bool) {
	// Check command-line overrides first
	if value, exists := ve.overrides[name]; exists {
		return value, true
	}

	// Check script-defined variables
	if value, exists := ve.variables[name]; exists {
		return value, true
	}

	// Check environment variables
	if value, exists := ve.environment[name]; exists {
		return value, true
	}

	return "", false
}

// Set sets a variable in the script variables map
func (ve *VariableExpander) Set(name, value string) {
	if ve.variables == nil {
		ve.variables = make(map[string]string)
	}
	ve.variables[name] = value
}

// Expand performs bash-style variable expansion on the given text
func (ve *VariableExpander) Expand(text string) (string, error) {
	result := text

	// Process ${VAR:+value} first (conditional expansion)
	result = conditionalPattern.ReplaceAllStringFunc(result, func(match string) string {
		matches := conditionalPattern.FindStringSubmatch(match)
		if len(matches) < 3 {
			return match // Should not happen, but be safe
		}

		varName := matches[1]
		value := matches[2]

		if varValue, exists := ve.Get(varName); exists && varValue != "" {
			return ve.expandValue(value)
		}

		return "" // Variable is unset or empty, return empty string
	})

	// Process ${VAR:=default} (assign default)
	result = assignPattern.ReplaceAllStringFunc(result, func(match string) string {
		matches := assignPattern.FindStringSubmatch(match)
		if len(matches) < 3 {
			return match
		}

		varName := matches[1]
		defaultValue := matches[2]

		if varValue, exists := ve.Get(varName); exists && varValue != "" {
			return varValue
		}

		// Variable is unset or empty, set it to default and return default
		expandedDefault := ve.expandValue(defaultValue)
		ve.Set(varName, expandedDefault)
		return expandedDefault
	})

	// Process ${VAR:-default} (use default)
	result = defaultPattern.ReplaceAllStringFunc(result, func(match string) string {
		matches := defaultPattern.FindStringSubmatch(match)
		if len(matches) < 3 {
			return match
		}

		varName := matches[1]
		defaultValue := matches[2]

		if varValue, exists := ve.Get(varName); exists && varValue != "" {
			return varValue
		}

		return ve.expandValue(defaultValue)
	})

	// Process simple variables: $VAR and ${VAR}
	result = simpleVarPattern.ReplaceAllStringFunc(result, func(match string) string {
		matches := simpleVarPattern.FindStringSubmatch(match)
		var varName string

		// Check which capture group matched
		if matches[1] != "" {
			varName = matches[1] // $VAR format
		} else if matches[2] != "" {
			varName = matches[2] // ${VAR} format
		}

		if varValue, exists := ve.Get(varName); exists {
			return varValue
		}

		// Variable not found, return empty string (bash behavior)
		return ""
	})

	return result, nil
}

// expandValue recursively expands variables in a value (for nested expansion)
func (ve *VariableExpander) expandValue(value string) string {
	// Simple recursive expansion - in practice, bash has limits to prevent infinite recursion
	expanded, err := ve.Expand(value)
	if err != nil {
		return value // Return original on error
	}
	return expanded
}

// ParseAssignment parses a variable assignment line
func (ve *VariableExpander) ParseAssignment(line string) (name, value string, isAssignment bool) {
	matches := assignmentPattern.FindStringSubmatch(strings.TrimSpace(line))
	if len(matches) < 3 {
		return "", "", false
	}

	name = matches[1]
	value = matches[2]

	// Expand the value using current variable context
	expandedValue, err := ve.Expand(value)
	if err != nil {
		// On error, use the original value
		expandedValue = value
	}

	return name, expandedValue, true
}

// LoadFromEnvironment loads environment variables into the expander
func (ve *VariableExpander) LoadFromEnvironment() {
	if ve.environment == nil {
		ve.environment = make(map[string]string)
	}

	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			ve.environment[parts[0]] = parts[1]
		}
	}
}

// LoadFromFile loads variables from a file (simple KEY=VALUE format)
func (ve *VariableExpander) LoadFromFile(filename string) error {
	content, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read environment file %s: %w", filename, err)
	}

	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		name, value, isAssignment := ve.ParseAssignment(line)
		if !isAssignment {
			return fmt.Errorf("invalid assignment on line %d: %s", i+1, line)
		}

		ve.Set(name, value)
	}

	return nil
}

// SetOverrides sets command-line variable overrides
func (ve *VariableExpander) SetOverrides(overrides []string) error {
	if ve.overrides == nil {
		ve.overrides = make(map[string]string)
	}

	for _, override := range overrides {
		parts := strings.SplitN(override, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid variable override format: %s (expected key=value)", override)
		}

		name := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Validate variable name
		if !regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`).MatchString(name) {
			return fmt.Errorf("invalid variable name: %s", name)
		}

		ve.overrides[name] = value
	}

	return nil
}

// GetAllVariables returns all variables from all sources
func (ve *VariableExpander) GetAllVariables() map[string]string {
	result := make(map[string]string)

	// Add environment variables
	for k, v := range ve.environment {
		result[k] = v
	}

	// Add script variables (override environment)
	for k, v := range ve.variables {
		result[k] = v
	}

	// Add overrides (override everything)
	for k, v := range ve.overrides {
		result[k] = v
	}

	return result
}

// GetUsedVariables extracts all variable references from text
func GetUsedVariables(text string) []string {
	var variables []string
	seen := make(map[string]bool)

	// Find all variable patterns
	patterns := []*regexp.Regexp{
		simpleVarPattern,
		defaultPattern,
		assignPattern,
		conditionalPattern,
	}

	for _, pattern := range patterns {
		matches := pattern.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			for i := 1; i < len(match); i++ {
				if match[i] != "" && !seen[match[i]] {
					variables = append(variables, match[i])
					seen[match[i]] = true
				}
			}
		}
	}

	return variables
}

// ValidateVariableName checks if a variable name is valid
func ValidateVariableName(name string) bool {
	return regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`).MatchString(name)
}
