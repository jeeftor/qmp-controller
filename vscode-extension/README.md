# QMP Script2 Syntax Highlighting

VSCode extension providing syntax highlighting for QMP Script2 automation files.

## Features

- **Comprehensive syntax highlighting** for all Script2 language features
- **Automatic grammar generation** from actual parser code
- **Always up-to-date** with the latest Script2 syntax
- **Color-coded** directives, variables, functions, and comments

## Supported Syntax

### Variables
- Variable assignments: `USER=admin`
- Variable expansion: `$USER`, `${USER:-default}`
- Complex expansions: `${VAR:=value}`, `${VAR:+value}`

### Directives
- Key sequences: `<enter>`, `<tab>`, `<ctrl+c>`
- Wait commands: `<wait 5s>`, `<watch "text" 30s>`
- Console switching: `<console 2>`
- Control flow: `<exit 1>`, `<retry 3>`, `<repeat 5>`

### Advanced Features
- **Functions**: `<function name>`, `<call function args>`
- **Conditionals**: `<if-found "text" 5s>`, `<else>`
- **Loops**: `<while-found "text" 30s poll 1s>`
- **Composition**: `<include "script.txt">`
- **Debugging**: `<screenshot "file.png">`

### Comments and Escaping
- Comments: `# This is a comment`
- Escaped directives: `\<literal angle brackets>`

## Installation

### From VSIX (Recommended)
1. Download the latest `qmp-script2-*.vsix` file
2. Open VSCode
3. Run: `code --install-extension qmp-script2-*.vsix`

### From Source
1. Clone the repository
2. Navigate to the extension directory
3. Run: `npm install && npm run compile`
4. Press F5 to launch extension development host

## File Associations

The extension automatically activates for files with these extensions:
- `.sc2` (recommended)
- `.script2`
- `.qmp2`

You can also manually set the language mode in VSCode:
1. Open a script file
2. Press `Ctrl+K M` (or `Cmd+K M` on Mac)
3. Type "QMP Script2" and select it

## Grammar Generation

This extension uses **automated grammar generation** from the actual QMP Script2 parser code.
The grammar is automatically synchronized with parser changes to ensure accuracy.

### Extracted Patterns

The following patterns were automatically extracted from the parser:


#### Conditionals
- **elseRegex**: Else directive: else

#### Control
- **consoleRegex**: Console switching: console 2
- **exitRegex**: Exit command: exit 1
- **waitRegex**: Wait delay: wait 5s
- **watchRegex**: Watch patterns: watch "text" 30s

#### Debugging
- **screenshotRegex**: Screenshot directive: screenshot "filename.png"

#### Directives
- **directiveRegex**: Directive patterns: <something> but not \<something>
- **escapedDirectiveRegex**: Escaped directive patterns: \<something>

#### Functions
- **endFunctionRegex**: End function: end-function
- **functionCallRegex**: Function call: call function_name args...
- **functionDefRegex**: Function definition: function name

#### Structure
- **emptyLineRegex**: Empty or whitespace-only lines

#### Variables
- **variableAssignmentRegex**: Variable assignment: USER=value or USER=${USER:-default}

#### Comments
- **commentRegex**: Comment lines starting with #

#### Keys
- **keySequenceRegex**: Key sequence patterns: enter, tab, ctrl+c, etc.

#### Loops
- **repeatRegex**: Repeat directive: repeat 5
- **retryRegex**: Retry directive: retry 3
- **whileFoundRegex**: While-found loop: while-found "text" 30s poll 1s
- **whileNotFoundRegex**: While-not-found loop: while-not-found "text" 30s poll 1s

#### Misc
- **caseRegex**: Script2 pattern: caseRegex
- **defaultRegex**: Script2 pattern: defaultRegex
- **endCaseRegex**: Script2 pattern: endCaseRegex
- **endIfRegex**: Script2 pattern: endIfRegex
- **endSwitchRegex**: Script2 pattern: endSwitchRegex
- **returnRegex**: Script2 pattern: returnRegex
- **setRegex**: Script2 pattern: setRegex
- **switchRegex**: Script2 pattern: switchRegex

#### Composition
- **includeRegex**: Include directive: include "script.txt"


## Development

### Updating Grammar
When the Script2 parser is updated, regenerate the grammar:

```bash
go run main.go generate-vscode-grammar ./vscode-extension
```

### Building Extension
```bash
npm install
npm run compile
vsce package
```

## License

MIT License - see LICENSE file for details.
