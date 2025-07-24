package script2

import (
	"time"

	"github.com/jeeftor/qmp-controller/internal/qmp"
)

// LineType represents the different types of lines in a script2 file
type LineType int

const (
	TextLine LineType = iota      // Regular text to be typed (most common)
	VariableLine                  // Variable assignments: USER=value, USER=${USER:-default}
	DirectiveLine                 // Special directives: <enter>, <watch "text" 30s>
	ConditionalLine              // Conditional blocks: <watch "text" && { ... }>
	CommentLine                  // Comments starting with #
	EmptyLine                    // Empty or whitespace-only lines
)

// String returns a human-readable representation of the LineType
func (lt LineType) String() string {
	switch lt {
	case TextLine:
		return "text"
	case VariableLine:
		return "variable"
	case DirectiveLine:
		return "directive"
	case ConditionalLine:
		return "conditional"
	case CommentLine:
		return "comment"
	case EmptyLine:
		return "empty"
	default:
		return "unknown"
	}
}

// DirectiveType represents different types of script directives
type DirectiveType int

const (
	KeySequence DirectiveType = iota  // <enter>, <tab>, <ctrl+c>
	Watch                            // <watch "text" 30s>
	Console                          // <console 2>
	WaitDelay                       // <wait 5s>
	Exit                            // <exit 1>
	ConditionalWatch                // <watch "text" 5s || { ... }>
	ConditionalIfFound             // <if-found "text" 5s>
	ConditionalIfNotFound          // <if-not-found "text" 5s>
	Else                           // <else>
	Retry                          // <retry 3>
	Repeat                         // <repeat 5>
	WhileFound                     // <while-found "text" 30s>
	WhileNotFound                  // <while-not-found "text" 30s>
	Include                        // <include "script.txt">
	Screenshot                     // <screenshot "filename.png">
	FunctionDef                    // <function name>
	EndFunction                    // <end-function>
	FunctionCall                   // <call function_name args...>
)

// String returns a human-readable representation of the DirectiveType
func (dt DirectiveType) String() string {
	switch dt {
	case KeySequence:
		return "key_sequence"
	case Watch:
		return "watch"
	case Console:
		return "console"
	case WaitDelay:
		return "wait"
	case Exit:
		return "exit"
	case ConditionalWatch:
		return "conditional_watch"
	case ConditionalIfFound:
		return "if_found"
	case ConditionalIfNotFound:
		return "if_not_found"
	case Else:
		return "else"
	case Retry:
		return "retry"
	case Repeat:
		return "repeat"
	case WhileFound:
		return "while_found"
	case WhileNotFound:
		return "while_not_found"
	case Include:
		return "include"
	case Screenshot:
		return "screenshot"
	case FunctionDef:
		return "function_def"
	case EndFunction:
		return "end_function"
	case FunctionCall:
		return "function_call"
	default:
		return "unknown"
	}
}

// Directive represents a parsed script directive
type Directive struct {
	Type        DirectiveType     `json:"type"`
	Command     string           `json:"command"`           // Raw command text
	Args        []string         `json:"args"`              // Parsed arguments
	SearchText  string           `json:"search_text"`       // Text to search for
	Timeout     time.Duration    `json:"timeout"`           // Operation timeout
	ExitCode    int              `json:"exit_code"`         // For exit directives
	ConsoleNum  int              `json:"console_num"`       // For console directives
	KeyName     string           `json:"key_name"`          // For key sequences
	Block       []ParsedLine     `json:"block,omitempty"`   // For conditional blocks
	ElseBlock   []ParsedLine     `json:"else_block,omitempty"` // For else conditions
	Condition   string           `json:"condition"`         // Condition expression
	RetryCount  int              `json:"retry_count"`       // For retry directives
	RepeatCount int              `json:"repeat_count"`      // For repeat directives
	PollInterval time.Duration   `json:"poll_interval"`     // For while loops
	IncludePath string           `json:"include_path"`      // For include directives
	ScreenshotPath string        `json:"screenshot_path"`   // For screenshot directives
	ScreenshotFormat string      `json:"screenshot_format"` // Screenshot format (png, ppm, jpg)
	FunctionName string          `json:"function_name"`     // For function definition and calls
	FunctionArgs []string        `json:"function_args"`     // For function call arguments
}

// ParsedLine represents a single parsed line from a script2 file
type ParsedLine struct {
	Type         LineType           `json:"type"`
	LineNumber   int               `json:"line_number"`
	OriginalText string            `json:"original_text"`     // Original line content
	Content      string            `json:"content"`           // Processed content
	Variables    map[string]string `json:"variables,omitempty"` // For variable lines
	Directive    *Directive        `json:"directive,omitempty"` // For directive lines
	ExpandedText string            `json:"expanded_text"`     // After variable substitution
	Indent       int               `json:"indent"`            // Indentation level
}

// Function represents a parsed function definition
type Function struct {
	Name      string        `json:"name"`
	Lines     []ParsedLine  `json:"lines"`
	Defined   bool          `json:"defined"`
	LineStart int           `json:"line_start"` // For error reporting
	LineEnd   int           `json:"line_end"`   // For error reporting
}

// Script represents a complete parsed script2 file
type Script struct {
	Lines         []ParsedLine      `json:"lines"`
	Variables     map[string]string `json:"variables"`         // Script-defined variables
	Environment   map[string]string `json:"environment"`       // Environment variables
	Functions     map[string]*Function `json:"functions"`      // Function definitions
	Metadata      ScriptMetadata    `json:"metadata"`
}

// ScriptMetadata contains information about the script
type ScriptMetadata struct {
	Filename      string            `json:"filename"`
	TotalLines    int               `json:"total_lines"`
	TextLines     int               `json:"text_lines"`
	DirectiveLines int              `json:"directive_lines"`
	VariableLines int               `json:"variable_lines"`
	FunctionLines int               `json:"function_lines"`
	ParsedAt      time.Time         `json:"parsed_at"`
	Variables     []string          `json:"variables"`         // List of variable names used
	Functions     []string          `json:"functions"`         // List of function names defined
}

// FunctionCallContext holds context for function execution
type FunctionCallContext struct {
	FunctionName string            `json:"function_name"`
	Parameters   []string          `json:"parameters"`
	LocalVars    map[string]string `json:"local_vars"`
	CallLine     int               `json:"call_line"`          // Line where function was called
}

// ExecutionContext holds the runtime context for script execution
type ExecutionContext struct {
	Client        *qmp.Client       `json:"-"`                 // QMP client (not serialized)
	VMID          string            `json:"vmid"`
	Variables     *VariableExpander `json:"-"`                 // Variable expander (not serialized)
	CurrentLine   int               `json:"current_line"`
	StartTime     time.Time         `json:"start_time"`
	Timeout       time.Duration     `json:"timeout"`
	DryRun        bool              `json:"dry_run"`
	Debug         bool              `json:"debug"`
	TrainingData  string            `json:"training_data"`      // Path to OCR training data file
	FunctionStack []*FunctionCallContext `json:"function_stack"` // For nested function calls
}

// VariableExpander handles bash-style variable expansion
type VariableExpander struct {
	environment map[string]string  // Environment variables
	variables   map[string]string  // Script-defined variables
	overrides   map[string]string  // Command-line overrides
}

// NewVariableExpander creates a new variable expander
func NewVariableExpander(env, vars, overrides map[string]string) *VariableExpander {
	return &VariableExpander{
		environment: env,
		variables:   vars,
		overrides:   overrides,
	}
}

// Parser handles script parsing
type Parser struct {
	variables    *VariableExpander
	currentLine  int
	debug        bool
}

// NewParser creates a new script parser
func NewParser(variables *VariableExpander, debug bool) *Parser {
	return &Parser{
		variables:   variables,
		currentLine: 0,
		debug:       debug,
	}
}

// Executor handles script execution
type Executor struct {
	context  *ExecutionContext
	parser   *Parser
	script   *Script
	debug    bool
}

// NewExecutor creates a new script executor
func NewExecutor(context *ExecutionContext, debug bool) *Executor {
	return &Executor{
		context: context,
		debug:   debug,
	}
}

// SetParser sets the parser for the executor (needed for validation)
func (e *Executor) SetParser(parser *Parser) {
	e.parser = parser
}

// SetScript sets the script for the executor (needed for function access)
func (e *Executor) SetScript(script *Script) {
	e.script = script
}

// ExecutionResult represents the result of script execution
type ExecutionResult struct {
	Success       bool              `json:"success"`
	LinesExecuted int               `json:"lines_executed"`
	Duration      time.Duration     `json:"duration"`
	ExitCode      int               `json:"exit_code"`
	Error         string            `json:"error,omitempty"`
	Variables     map[string]string `json:"final_variables"`
}

// ValidationResult represents script validation results
type ValidationResult struct {
	Valid         bool              `json:"valid"`
	Errors        []ValidationError `json:"errors,omitempty"`
	Warnings      []ValidationError `json:"warnings,omitempty"`
	Variables     []string          `json:"variables"`
	Directives    []string          `json:"directives"`
}

// ValidationError represents a validation error or warning
type ValidationError struct {
	LineNumber  int    `json:"line_number"`
	Type        string `json:"type"`        // "error" or "warning"
	Message     string `json:"message"`
	Suggestion  string `json:"suggestion,omitempty"`
}
