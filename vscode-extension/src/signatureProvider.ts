import * as vscode from 'vscode';

interface DirectiveSignature {
    label: string;
    documentation: string;
    parameters: {
        label: string;
        documentation: string;
    }[];
}

export class QmpScript2SignatureHelpProvider implements vscode.SignatureHelpProvider {
    private signatures: { [key: string]: DirectiveSignature } = {};

    constructor() {
        this.initializeSignatures();
    }

    provideSignatureHelp(
        document: vscode.TextDocument,
        position: vscode.Position,
        token: vscode.CancellationToken,
        context: vscode.SignatureHelpContext
    ): vscode.ProviderResult<vscode.SignatureHelp> {
        const lineText = document.lineAt(position).text;
        const textBeforeCursor = lineText.substring(0, position.character);

        // Check if we're inside a directive
        const directiveMatch = textBeforeCursor.match(/<([a-zA-Z-]+)([^>]*)$/);
        if (!directiveMatch) {
            return null;
        }

        const directiveName = directiveMatch[1];
        const signature = this.signatures[directiveName];

        if (!signature) {
            return null;
        }

        const signatureInfo = new vscode.SignatureInformation(signature.label);
        signatureInfo.documentation = new vscode.MarkdownString(signature.documentation);

        signature.parameters.forEach(param => {
            const paramInfo = new vscode.ParameterInformation(param.label, param.documentation);
            signatureInfo.parameters.push(paramInfo);
        });

        const signatureHelp = new vscode.SignatureHelp();
        signatureHelp.signatures = [signatureInfo];
        signatureHelp.activeSignature = 0;

        // Determine active parameter based on cursor position
        const paramText = directiveMatch[2];
        const activeParameter = this.getActiveParameterIndex(paramText, directiveName);
        signatureHelp.activeParameter = activeParameter;

        return signatureHelp;
    }

    private initializeSignatures(): void {
        this.signatures = {
            'watch': {
                label: '<watch "search_text" timeout>',
                documentation: 'Wait for specific text to appear on screen with OCR scanning',
                parameters: [
                    {
                        label: '"search_text"',
                        documentation: 'Text pattern to search for on the console screen'
                    },
                    {
                        label: 'timeout',
                        documentation: 'Maximum time to wait (e.g., 30s, 2m, 1h)'
                    }
                ]
            },
            'wait': {
                label: '<wait duration>',
                documentation: 'Pause script execution for specified duration',
                parameters: [
                    {
                        label: 'duration',
                        documentation: 'Time to wait (e.g., 5s, 1m, 500ms)'
                    }
                ]
            },
            'console': {
                label: '<console number>',
                documentation: 'Switch to virtual console using Ctrl+Alt+F[number]',
                parameters: [
                    {
                        label: 'number',
                        documentation: 'Console number (1-6 typically)'
                    }
                ]
            },
            'call': {
                label: '<call function_name [args...]>',
                documentation: 'Call a defined function with optional arguments',
                parameters: [
                    {
                        label: 'function_name',
                        documentation: 'Name of the function to call'
                    },
                    {
                        label: '[args...]',
                        documentation: 'Optional arguments passed as $1, $2, etc.'
                    }
                ]
            },
            'function': {
                label: '<function name>',
                documentation: 'Define a reusable function',
                parameters: [
                    {
                        label: 'name',
                        documentation: 'Unique function name'
                    }
                ]
            },
            'set': {
                label: '<set variable="value">',
                documentation: 'Set a local variable within functions',
                parameters: [
                    {
                        label: 'variable',
                        documentation: 'Variable name'
                    },
                    {
                        label: '"value"',
                        documentation: 'Variable value (supports parameter expansion like ${1:-default})'
                    }
                ]
            },
            'include': {
                label: '<include "filepath">',
                documentation: 'Include another script2 file',
                parameters: [
                    {
                        label: '"filepath"',
                        documentation: 'Path to script file (relative or absolute)'
                    }
                ]
            },
            'switch': {
                label: '<switch timeout=duration poll=interval>',
                documentation: 'Pattern matching construct for multiple screen conditions',
                parameters: [
                    {
                        label: 'timeout=duration',
                        documentation: 'Maximum time to scan for patterns (e.g., timeout=30s)'
                    },
                    {
                        label: 'poll=interval',
                        documentation: 'Interval between screen scans (e.g., poll=1s)'
                    }
                ]
            },
            'case': {
                label: '<case "pattern">',
                documentation: 'Define a pattern to match within switch statement',
                parameters: [
                    {
                        label: '"pattern"',
                        documentation: 'Text pattern to match on screen'
                    }
                ]
            },
            'exit': {
                label: '<exit code>',
                documentation: 'Terminate script with exit code',
                parameters: [
                    {
                        label: 'code',
                        documentation: 'Exit code (0=success, non-zero=error)'
                    }
                ]
            },
            'screenshot': {
                label: '<screenshot "filename">',
                documentation: 'Capture VM console screenshot',
                parameters: [
                    {
                        label: '"filename"',
                        documentation: 'Output filename (.png, .ppm supported)'
                    }
                ]
            },
            'if-found': {
                label: '<if-found "text" timeout>',
                documentation: 'Conditional execution if text is found on screen',
                parameters: [
                    {
                        label: '"text"',
                        documentation: 'Text pattern to search for'
                    },
                    {
                        label: 'timeout',
                        documentation: 'Maximum time to search'
                    }
                ]
            },
            'if-not-found': {
                label: '<if-not-found "text" timeout>',
                documentation: 'Conditional execution if text is NOT found on screen',
                parameters: [
                    {
                        label: '"text"',
                        documentation: 'Text pattern to search for'
                    },
                    {
                        label: 'timeout',
                        documentation: 'Maximum time to search'
                    }
                ]
            },
            'while-found': {
                label: '<while-found "text" timeout poll interval>',
                documentation: 'Loop while text is present on screen',
                parameters: [
                    {
                        label: '"text"',
                        documentation: 'Text pattern to monitor'
                    },
                    {
                        label: 'timeout',
                        documentation: 'Maximum loop duration'
                    },
                    {
                        label: 'poll',
                        documentation: 'Keyword "poll"'
                    },
                    {
                        label: 'interval',
                        documentation: 'Time between checks'
                    }
                ]
            },
            'while-not-found': {
                label: '<while-not-found "text" timeout poll interval>',
                documentation: 'Loop while text is NOT present on screen',
                parameters: [
                    {
                        label: '"text"',
                        documentation: 'Text pattern to monitor'
                    },
                    {
                        label: 'timeout',
                        documentation: 'Maximum loop duration'
                    },
                    {
                        label: 'poll',
                        documentation: 'Keyword "poll"'
                    },
                    {
                        label: 'interval',
                        documentation: 'Time between checks'
                    }
                ]
            },
            'retry': {
                label: '<retry count>',
                documentation: 'Retry following operation up to count times',
                parameters: [
                    {
                        label: 'count',
                        documentation: 'Maximum number of retry attempts'
                    }
                ]
            },
            'repeat': {
                label: '<repeat count>',
                documentation: 'Repeat following operation exactly count times',
                parameters: [
                    {
                        label: 'count',
                        documentation: 'Number of repetitions'
                    }
                ]
            }
        };
    }

    private getActiveParameterIndex(paramText: string, directiveName: string): number {
        const signature = this.signatures[directiveName];
        if (!signature) {
            return 0;
        }

        // Simple parameter counting based on spaces and quotes
        // This is a basic implementation - could be made more sophisticated
        const trimmed = paramText.trim();
        if (!trimmed) {
            return 0;
        }

        // Count parameters by splitting on spaces, but respecting quoted strings
        const tokens = this.tokenizeParameters(trimmed);
        return Math.min(tokens.length - 1, signature.parameters.length - 1);
    }

    private tokenizeParameters(text: string): string[] {
        const tokens: string[] = [];
        let current = '';
        let inQuotes = false;
        let escapeNext = false;

        for (let i = 0; i < text.length; i++) {
            const char = text[i];

            if (escapeNext) {
                current += char;
                escapeNext = false;
                continue;
            }

            if (char === '\\') {
                escapeNext = true;
                current += char;
                continue;
            }

            if (char === '"') {
                inQuotes = !inQuotes;
                current += char;
                continue;
            }

            if (char === ' ' && !inQuotes) {
                if (current.trim()) {
                    tokens.push(current.trim());
                    current = '';
                }
                continue;
            }

            current += char;
        }

        if (current.trim()) {
            tokens.push(current.trim());
        }

        return tokens;
    }
}
