import * as vscode from 'vscode';

interface DirectiveDocumentation {
    [key: string]: {
        syntax: string;
        description: string;
        examples: string[];
        parameters?: { [param: string]: string };
    };
}

export class QmpScript2HoverProvider implements vscode.HoverProvider {
    private directiveDocs: DirectiveDocumentation = {};

    constructor() {
        this.initializeDocumentation();
    }

    provideHover(
        document: vscode.TextDocument,
        position: vscode.Position,
        token: vscode.CancellationToken
    ): vscode.ProviderResult<vscode.Hover> {
        const wordRange = document.getWordRangeAtPosition(position);
        const lineText = document.lineAt(position).text;

        // Check if we're hovering over a directive
        const directiveMatch = lineText.match(/<([^>\s]+)/);
        if (directiveMatch && wordRange) {
            const directive = directiveMatch[1];
            const docs = this.directiveDocs[directive];

            if (docs) {
                const markdown = new vscode.MarkdownString();
                markdown.isTrusted = true;

                // Title
                markdown.appendMarkdown(`**\`<${directive}>\`** directive\\n\\n`);

                // Syntax
                markdown.appendMarkdown(`**Syntax:** \`${docs.syntax}\`\\n\\n`);

                // Description
                markdown.appendMarkdown(`${docs.description}\\n\\n`);

                // Parameters
                if (docs.parameters) {
                    markdown.appendMarkdown('**Parameters:**\\n');
                    for (const [param, desc] of Object.entries(docs.parameters)) {
                        markdown.appendMarkdown(`- \`${param}\`: ${desc}\\n`);
                    }
                    markdown.appendMarkdown('\\n');
                }

                // Examples
                if (docs.examples.length > 0) {
                    markdown.appendMarkdown('**Examples:**\\n');
                    docs.examples.forEach(example => {
                        markdown.appendCodeblock(example, 'qmp-script2');
                    });
                }

                return new vscode.Hover(markdown, wordRange);
            }
        }

        // Check if we're hovering over a variable
        const variableMatch = lineText.match(/\$([a-zA-Z_][a-zA-Z0-9_]*)/);
        if (variableMatch && wordRange) {
            const variableName = variableMatch[1];
            const variableInfo = this.findVariableDefinition(document, variableName);

            if (variableInfo) {
                const markdown = new vscode.MarkdownString();
                markdown.appendMarkdown(`**Variable:** \`$${variableName}\`\\n\\n`);

                if (variableInfo.value) {
                    markdown.appendMarkdown(`**Value:** \`${variableInfo.value}\`\\n\\n`);
                }

                markdown.appendMarkdown(`**Defined at:** Line ${variableInfo.line}\\n`);

                return new vscode.Hover(markdown, wordRange);
            }
        }

        // Check if we're hovering over a function call
        const functionMatch = lineText.match(/<call\s+([a-zA-Z_][a-zA-Z0-9_]*)/);
        if (functionMatch && wordRange) {
            const functionName = functionMatch[1];
            const functionInfo = this.findFunctionDefinition(document, functionName);

            if (functionInfo) {
                const markdown = new vscode.MarkdownString();
                markdown.appendMarkdown(`**Function:** \`${functionName}\`\\n\\n`);
                markdown.appendMarkdown(`**Defined at:** Line ${functionInfo.line}\\n`);

                if (functionInfo.parameters && functionInfo.parameters.length > 0) {
                    markdown.appendMarkdown(`**Parameters:** ${functionInfo.parameters.join(', ')}\\n`);
                }

                return new vscode.Hover(markdown, wordRange);
            }
        }

        return null;
    }

    private initializeDocumentation(): void {
        this.directiveDocs = {
            'watch': {
                syntax: '<watch "search_text" timeout>',
                description: 'Wait for specific text to appear on the console screen. Uses OCR to continuously scan the screen until the text is found or timeout is reached.',
                parameters: {
                    'search_text': 'Text pattern to search for on screen',
                    'timeout': 'Maximum time to wait (e.g., 30s, 2m, 1h)'
                },
                examples: [
                    '<watch "login:" 60s>',
                    '<watch "Password" 30s>',
                    '<watch "~]$" 10s>'
                ]
            },
            'wait': {
                syntax: '<wait duration>',
                description: 'Pause script execution for the specified duration.',
                parameters: {
                    'duration': 'Time to wait (e.g., 5s, 1m, 30s)'
                },
                examples: [
                    '<wait 5s>',
                    '<wait 1m>',
                    '<wait 500ms>'
                ]
            },
            'console': {
                syntax: '<console number>',
                description: 'Switch to a different virtual console using Ctrl+Alt+F[number]. Useful for accessing different TTY sessions.',
                parameters: {
                    'number': 'Console number (1-6, typically)'
                },
                examples: [
                    '<console 2>',
                    '<console 1>',
                    '<console 6>'
                ]
            },
            'call': {
                syntax: '<call function_name [args...]>',
                description: 'Call a previously defined function with optional arguments. Arguments are passed as $1, $2, etc.',
                parameters: {
                    'function_name': 'Name of the function to call',
                    'args': 'Optional arguments passed to the function'
                },
                examples: [
                    '<call login>',
                    '<call setup_user admin password123>',
                    '<call deploy_service web-app 8080>'
                ]
            },
            'function': {
                syntax: '<function name>',
                description: 'Define a reusable function. Functions can accept parameters and contain any script2 commands.',
                parameters: {
                    'name': 'Function name (must be unique)'
                },
                examples: [
                    '<function login>\\n    # Function body\\n<end-function>',
                    '<function setup_database>\\n    <call connect_mysql>\\n    <call create_tables>\\n<end-function>'
                ]
            },
            'set': {
                syntax: '<set variable="value">',
                description: 'Set a local variable within functions. Variables can use parameter expansion like ${1:-default}.',
                parameters: {
                    'variable': 'Variable name',
                    'value': 'Variable value (supports parameter expansion)'
                },
                examples: [
                    '<set user="admin">',
                    '<set password="${2:-default_password}">',
                    '<set host="${1:-localhost}">'
                ]
            },
            'include': {
                syntax: '<include "filepath">',
                description: 'Include another script2 file. Functions and variables from included files become available.',
                parameters: {
                    'filepath': 'Path to script file (relative or absolute)'
                },
                examples: [
                    '<include "./scripts/common.sc2">',
                    '<include "/etc/qmp/login.sc2">',
                    '<include "../shared/database.sc2">'
                ]
            },
            'switch': {
                syntax: '<switch timeout=duration poll=interval>',
                description: 'Pattern matching construct that continuously scans the screen for multiple possible text patterns. Executes the first matching case.',
                parameters: {
                    'timeout': 'Maximum time to scan for patterns',
                    'poll': 'Interval between screen scans'
                },
                examples: [
                    '<switch timeout=30s poll=1s>\\n    <case "success">\\n        <return>\\n    <end-case>\\n    <case "error">\\n        <exit 1>\\n    <end-case>\\n<end-switch>'
                ]
            },
            'case': {
                syntax: '<case "pattern">',
                description: 'Define a pattern to match within a switch statement. If the pattern is found on screen, the case body is executed.',
                parameters: {
                    'pattern': 'Text pattern to match on screen'
                },
                examples: [
                    '<case "Login successful">\\n    <return>\\n<end-case>',
                    '<case "Access denied">\\n    <exit 1>\\n<end-case>'
                ]
            },
            'exit': {
                syntax: '<exit code>',
                description: 'Terminate script execution with the specified exit code. Non-zero codes indicate errors.',
                parameters: {
                    'code': 'Exit code (0 = success, non-zero = error)'
                },
                examples: [
                    '<exit 0>',
                    '<exit 1>',
                    '<exit 255>'
                ]
            },
            'break': {
                syntax: '<break>',
                description: 'Insert a debug breakpoint. When reached during execution with debug mode, the script will pause for inspection.',
                examples: [
                    '<break>',
                    '# Debug checkpoint\\n<break>'
                ]
            },
            'return': {
                syntax: '<return>',
                description: 'Return from the current function. Execution continues after the function call.',
                examples: [
                    '<return>',
                    '# Early return on success\\n<return>'
                ]
            },
            'screenshot': {
                syntax: '<screenshot "filename">',
                description: 'Capture a screenshot of the VM console and save it to the specified file.',
                parameters: {
                    'filename': 'Output filename (supports .png, .ppm formats)'
                },
                examples: [
                    '<screenshot "login_screen.png">',
                    '<screenshot "/tmp/debug_$(date).png">',
                    '<screenshot "error_state.ppm">'
                ]
            },
            'if-found': {
                syntax: '<if-found "text" timeout>',
                description: 'Conditional execution based on whether text is found on screen within the timeout period.',
                parameters: {
                    'text': 'Text pattern to search for',
                    'timeout': 'Maximum time to search'
                },
                examples: [
                    '<if-found "success" 10s>\\n    <return>\\n<end-if>',
                    '<if-found "Continue?" 5s>\\n    yes\\n    <enter>\\n<end-if>'
                ]
            },
            'while-found': {
                syntax: '<while-found "text" timeout poll interval>',
                description: 'Loop while the specified text is present on screen. Useful for waiting for processes to complete.',
                parameters: {
                    'text': 'Text pattern to monitor',
                    'timeout': 'Maximum loop duration',
                    'interval': 'Time between checks'
                },
                examples: [
                    '<while-found "Processing..." 60s poll 2s>\\n    <wait 5s>\\n<end-while>',
                    '<while-found "Installing" 300s poll 1s>\\n    # Wait for installation\\n<end-while>'
                ]
            },
            'retry': {
                syntax: '<retry count>',
                description: 'Retry the following operation up to count times if it fails.',
                parameters: {
                    'count': 'Maximum number of retry attempts'
                },
                examples: [
                    '<retry 3>\\n<call connect_database>',
                    '<retry 5>\\n<watch "ready" 10s>'
                ]
            },
            'repeat': {
                syntax: '<repeat count>',
                description: 'Repeat the following operation exactly count times.',
                parameters: {
                    'count': 'Number of times to repeat'
                },
                examples: [
                    '<repeat 3>\\n<enter>',
                    '<repeat 5>\\n<wait 1s>\\nping'
                ]
            }
        };
    }

    private findVariableDefinition(document: vscode.TextDocument, variableName: string): { value?: string; line: number } | null {
        // Search for variable assignments
        for (let i = 0; i < document.lineCount; i++) {
            const line = document.lineAt(i).text;

            // Check for VAR=value assignments
            const assignmentMatch = line.match(new RegExp(`^\\s*${variableName}\\s*=\\s*(.+)$`));
            if (assignmentMatch) {
                return { value: assignmentMatch[1], line: i + 1 };
            }

            // Check for <set var="value"> directives
            const setMatch = line.match(new RegExp(`<set\\s+${variableName}\\s*=\\s*"([^"]*)">`));
            if (setMatch) {
                return { value: setMatch[1], line: i + 1 };
            }
        }

        return null;
    }

    private findFunctionDefinition(document: vscode.TextDocument, functionName: string): { line: number; parameters?: string[] } | null {
        for (let i = 0; i < document.lineCount; i++) {
            const line = document.lineAt(i).text;
            const functionMatch = line.match(new RegExp(`<function\\s+${functionName}>`));
            if (functionMatch) {
                return { line: i + 1, parameters: [] }; // TODO: Parse parameters
            }
        }

        return null;
    }
}
