import * as vscode from 'vscode';
import * as fs from 'fs';
import * as path from 'path';

interface DirectiveCompletion {
    label: string;
    insertText: string;
    documentation: string;
    kind: vscode.CompletionItemKind;
    snippet?: boolean;
}

interface FunctionInfo {
    name: string;
    parameters: string[];
    documentation: string;
    line: number;
}

interface VariableInfo {
    name: string;
    value?: string;
    line: number;
}

export class QmpScript2CompletionProvider implements vscode.CompletionItemProvider {
    private directiveCompletions: DirectiveCompletion[] = [];
    private keySequenceCompletions: DirectiveCompletion[] = [];

    constructor() {
        this.initializeDirectiveCompletions();
        this.initializeKeySequenceCompletions();
    }

    provideCompletionItems(
        document: vscode.TextDocument,
        position: vscode.Position,
        token: vscode.CancellationToken,
        context: vscode.CompletionContext
    ): vscode.ProviderResult<vscode.CompletionList> {
        const lineText = document.lineAt(position).text;
        const textBeforeCursor = lineText.substring(0, position.character);

        // Parse current document for functions and variables
        const documentFunctions = this.parseDocumentFunctions(document);
        const documentVariables = this.parseDocumentVariables(document);
        const includedFunctions = this.parseIncludedFunctions(document);

        const completions: vscode.CompletionItem[] = [];

        // Directive completion: <|
        if (textBeforeCursor.match(/<[^>]*$/)) {
            completions.push(...this.getDirectiveCompletions());
            completions.push(...this.getKeySequenceCompletions());
        }

        // Function call completion: <call |
        if (textBeforeCursor.match(/<call\s+[^>]*$/)) {
            completions.push(...this.getFunctionCallCompletions(documentFunctions, includedFunctions));
        }

        // Variable completion: $|
        if (textBeforeCursor.match(/\$[^}\s]*$/) || textBeforeCursor.match(/\$\{[^}]*$/)) {
            completions.push(...this.getVariableCompletions(documentVariables));
        }

        // Function definition completion in function context
        if (this.isInFunctionContext(document, position)) {
            completions.push(...this.getFunctionContextCompletions());
        }

        // Snippet completions at line start
        if (textBeforeCursor.trim() === '' || textBeforeCursor.match(/^\s*$/)) {
            completions.push(...this.getSnippetCompletions());
        }

        return new vscode.CompletionList(completions, false);
    }

    private initializeDirectiveCompletions(): void {
        this.directiveCompletions = [
            {
                label: 'watch',
                insertText: 'watch "${1:search_text}" ${2:30s}',
                documentation: 'Wait for specific text to appear on screen with timeout',
                kind: vscode.CompletionItemKind.Keyword,
                snippet: true
            },
            {
                label: 'wait',
                insertText: 'wait ${1:5s}',
                documentation: 'Wait for specified duration',
                kind: vscode.CompletionItemKind.Keyword,
                snippet: true
            },
            {
                label: 'console',
                insertText: 'console ${1:2}',
                documentation: 'Switch to console (Ctrl+Alt+F[1-6])',
                kind: vscode.CompletionItemKind.Keyword,
                snippet: true
            },
            {
                label: 'call',
                insertText: 'call ${1:function_name} ${2:args}',
                documentation: 'Call a defined function with optional arguments',
                kind: vscode.CompletionItemKind.Function,
                snippet: true
            },
            {
                label: 'function',
                insertText: 'function ${1:function_name}>\n    ${2:# Function body}\n<end-function',
                documentation: 'Define a reusable function',
                kind: vscode.CompletionItemKind.Function,
                snippet: true
            },
            {
                label: 'set',
                insertText: 'set ${1:variable}="${2:value}"',
                documentation: 'Set a local variable within functions',
                kind: vscode.CompletionItemKind.Variable,
                snippet: true
            },
            {
                label: 'include',
                insertText: 'include "${1:./scripts/filename.sc2}"',
                documentation: 'Include another script file',
                kind: vscode.CompletionItemKind.Module,
                snippet: true
            },
            {
                label: 'switch',
                insertText: 'switch timeout=${1:30s} poll=${2:1s}>\n    <case "${3:pattern}">\n        ${4:# Actions}\n    <end-case>\n<end-switch',
                documentation: 'Switch statement for pattern matching with OCR',
                kind: vscode.CompletionItemKind.Keyword,
                snippet: true
            },
            {
                label: 'case',
                insertText: 'case "${1:pattern}">\n    ${2:# Actions}\n<end-case',
                documentation: 'Case branch in switch statement',
                kind: vscode.CompletionItemKind.Keyword,
                snippet: true
            },
            {
                label: 'exit',
                insertText: 'exit ${1:0}',
                documentation: 'Exit script with specified code',
                kind: vscode.CompletionItemKind.Keyword,
                snippet: true
            },
            {
                label: 'break',
                insertText: 'break',
                documentation: 'Debug breakpoint - pause execution for debugging',
                kind: vscode.CompletionItemKind.Keyword
            },
            {
                label: 'return',
                insertText: 'return',
                documentation: 'Return from current function',
                kind: vscode.CompletionItemKind.Keyword
            },
            {
                label: 'screenshot',
                insertText: 'screenshot "${1:filename.png}"',
                documentation: 'Take a screenshot of the VM console',
                kind: vscode.CompletionItemKind.Method,
                snippet: true
            },
            {
                label: 'if-found',
                insertText: 'if-found "${1:text}" ${2:5s}>\n    ${3:# Actions if found}\n<end-if',
                documentation: 'Conditional execution if text is found',
                kind: vscode.CompletionItemKind.Keyword,
                snippet: true
            },
            {
                label: 'if-not-found',
                insertText: 'if-not-found "${1:text}" ${2:5s}>\n    ${3:# Actions if not found}\n<end-if',
                documentation: 'Conditional execution if text is not found',
                kind: vscode.CompletionItemKind.Keyword,
                snippet: true
            },
            {
                label: 'while-found',
                insertText: 'while-found "${1:text}" ${2:30s} poll ${3:1s}>\n    ${4:# Loop body}\n<end-while',
                documentation: 'Loop while text is present on screen',
                kind: vscode.CompletionItemKind.Keyword,
                snippet: true
            },
            {
                label: 'retry',
                insertText: 'retry ${1:3}',
                documentation: 'Retry the following operation N times',
                kind: vscode.CompletionItemKind.Keyword,
                snippet: true
            },
            {
                label: 'repeat',
                insertText: 'repeat ${1:5}',
                documentation: 'Repeat the following operation N times',
                kind: vscode.CompletionItemKind.Keyword,
                snippet: true
            }
        ];
    }

    private initializeKeySequenceCompletions(): void {
        this.keySequenceCompletions = [
            { label: 'enter', insertText: 'enter', documentation: 'Press Enter key', kind: vscode.CompletionItemKind.Constant },
            { label: 'tab', insertText: 'tab', documentation: 'Press Tab key', kind: vscode.CompletionItemKind.Constant },
            { label: 'escape', insertText: 'escape', documentation: 'Press Escape key', kind: vscode.CompletionItemKind.Constant },
            { label: 'backspace', insertText: 'backspace', documentation: 'Press Backspace key', kind: vscode.CompletionItemKind.Constant },
            { label: 'delete', insertText: 'delete', documentation: 'Press Delete key', kind: vscode.CompletionItemKind.Constant },
            { label: 'space', insertText: 'space', documentation: 'Press Space key', kind: vscode.CompletionItemKind.Constant },
            { label: 'up', insertText: 'up', documentation: 'Press Up arrow key', kind: vscode.CompletionItemKind.Constant },
            { label: 'down', insertText: 'down', documentation: 'Press Down arrow key', kind: vscode.CompletionItemKind.Constant },
            { label: 'left', insertText: 'left', documentation: 'Press Left arrow key', kind: vscode.CompletionItemKind.Constant },
            { label: 'right', insertText: 'right', documentation: 'Press Right arrow key', kind: vscode.CompletionItemKind.Constant },
            { label: 'home', insertText: 'home', documentation: 'Press Home key', kind: vscode.CompletionItemKind.Constant },
            { label: 'end', insertText: 'end', documentation: 'Press End key', kind: vscode.CompletionItemKind.Constant },
            { label: 'pageup', insertText: 'pageup', documentation: 'Press Page Up key', kind: vscode.CompletionItemKind.Constant },
            { label: 'pagedown', insertText: 'pagedown', documentation: 'Press Page Down key', kind: vscode.CompletionItemKind.Constant },
            { label: 'ctrl+c', insertText: 'ctrl+c', documentation: 'Press Ctrl+C (interrupt)', kind: vscode.CompletionItemKind.Constant },
            { label: 'ctrl+z', insertText: 'ctrl+z', documentation: 'Press Ctrl+Z (suspend)', kind: vscode.CompletionItemKind.Constant },
            { label: 'ctrl+d', insertText: 'ctrl+d', documentation: 'Press Ctrl+D (EOF)', kind: vscode.CompletionItemKind.Constant },
            { label: 'ctrl+l', insertText: 'ctrl+l', documentation: 'Press Ctrl+L (clear screen)', kind: vscode.CompletionItemKind.Constant },
            { label: 'alt+tab', insertText: 'alt+tab', documentation: 'Press Alt+Tab', kind: vscode.CompletionItemKind.Constant },
            { label: 'f1', insertText: 'f1', documentation: 'Press F1 function key', kind: vscode.CompletionItemKind.Constant },
            { label: 'f12', insertText: 'f12', documentation: 'Press F12 function key', kind: vscode.CompletionItemKind.Constant }
        ];
    }

    private getDirectiveCompletions(): vscode.CompletionItem[] {
        return this.directiveCompletions.map(directive => {
            const item = new vscode.CompletionItem(directive.label, directive.kind);
            if (directive.snippet) {
                item.insertText = new vscode.SnippetString(directive.insertText);
            } else {
                item.insertText = directive.insertText;
            }
            item.documentation = new vscode.MarkdownString(directive.documentation);
            item.sortText = '0' + directive.label; // Prioritize directives
            return item;
        });
    }

    private getKeySequenceCompletions(): vscode.CompletionItem[] {
        return this.keySequenceCompletions.map(key => {
            const item = new vscode.CompletionItem(key.label, key.kind);
            item.insertText = key.insertText;
            item.documentation = new vscode.MarkdownString(key.documentation);
            item.sortText = '1' + key.label; // Sort after directives
            return item;
        });
    }

    private parseDocumentFunctions(document: vscode.TextDocument): FunctionInfo[] {
        const functions: FunctionInfo[] = [];
        const functionRegex = /<function\s+([a-zA-Z_][a-zA-Z0-9_]*)>/g;

        for (let i = 0; i < document.lineCount; i++) {
            const line = document.lineAt(i);
            const match = functionRegex.exec(line.text);
            if (match) {
                functions.push({
                    name: match[1],
                    parameters: [], // TODO: Parse function parameters
                    documentation: `Function defined at line ${i + 1}`,
                    line: i + 1
                });
            }
        }

        return functions;
    }

    private parseDocumentVariables(document: vscode.TextDocument): VariableInfo[] {
        const variables: VariableInfo[] = [];
        const variableRegex = /^([A-Z_][A-Z0-9_]*)\s*=\s*(.*)$/gm;
        const setRegex = /<set\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*=\s*"([^"]*)">/g;

        for (let i = 0; i < document.lineCount; i++) {
            const line = document.lineAt(i);

            // Parse variable assignments: VAR=value
            const varMatch = variableRegex.exec(line.text);
            if (varMatch) {
                variables.push({
                    name: varMatch[1],
                    value: varMatch[2],
                    line: i + 1
                });
            }

            // Parse set directives: <set var="value">
            const setMatch = setRegex.exec(line.text);
            if (setMatch) {
                variables.push({
                    name: setMatch[1],
                    value: setMatch[2],
                    line: i + 1
                });
            }
        }

        return variables;
    }

    private parseIncludedFunctions(document: vscode.TextDocument): FunctionInfo[] {
        const functions: FunctionInfo[] = [];
        const includeRegex = /<include\s+"([^"]+)">/g;

        for (let i = 0; i < document.lineCount; i++) {
            const line = document.lineAt(i);
            const match = includeRegex.exec(line.text);
            if (match) {
                const includePath = match[1];
                const resolvedPath = this.resolveIncludePath(document.uri, includePath);
                if (resolvedPath && fs.existsSync(resolvedPath)) {
                    try {
                        const includeContent = fs.readFileSync(resolvedPath, 'utf8');
                        const includedFunctions = this.parseFunctionsFromContent(includeContent, includePath);
                        functions.push(...includedFunctions);
                    } catch (error) {
                        // Ignore file read errors
                    }
                }
            }
        }

        return functions;
    }

    private resolveIncludePath(documentUri: vscode.Uri, includePath: string): string | null {
        const documentDir = path.dirname(documentUri.fsPath);

        if (path.isAbsolute(includePath)) {
            return includePath;
        }

        return path.resolve(documentDir, includePath);
    }

    private parseFunctionsFromContent(content: string, filename: string): FunctionInfo[] {
        const functions: FunctionInfo[] = [];
        const functionRegex = /<function\s+([a-zA-Z_][a-zA-Z0-9_]*)>/g;
        const lines = content.split('\n');

        lines.forEach((line, index) => {
            const match = functionRegex.exec(line);
            if (match) {
                functions.push({
                    name: match[1],
                    parameters: [],
                    documentation: `Function from ${filename} (line ${index + 1})`,
                    line: index + 1
                });
            }
        });

        return functions;
    }

    private getFunctionCallCompletions(documentFunctions: FunctionInfo[], includedFunctions: FunctionInfo[]): vscode.CompletionItem[] {
        const allFunctions = [...documentFunctions, ...includedFunctions];

        return allFunctions.map(func => {
            const item = new vscode.CompletionItem(func.name, vscode.CompletionItemKind.Function);
            item.insertText = func.name;
            item.documentation = new vscode.MarkdownString(func.documentation);
            item.detail = `Function: ${func.name}`;
            item.sortText = '0' + func.name; // Prioritize functions
            return item;
        });
    }

    private getVariableCompletions(variables: VariableInfo[]): vscode.CompletionItem[] {
        const completions: vscode.CompletionItem[] = [];

        // Add document variables
        variables.forEach(variable => {
            const item = new vscode.CompletionItem(variable.name, vscode.CompletionItemKind.Variable);
            item.insertText = variable.name;
            item.documentation = new vscode.MarkdownString(
                `Variable: ${variable.name}\\n` +
                (variable.value ? `Value: \`${variable.value}\`` : '') +
                `\\nDefined at line ${variable.line}`
            );
            item.detail = variable.value || 'Variable';
            completions.push(item);
        });

        // Add common environment variables
        const envVars = ['USER', 'HOME', 'PATH', 'PWD', 'SHELL'];
        envVars.forEach(envVar => {
            const item = new vscode.CompletionItem(envVar, vscode.CompletionItemKind.Variable);
            item.insertText = envVar;
            item.documentation = new vscode.MarkdownString(`Environment variable: ${envVar}`);
            item.detail = 'Environment Variable';
            item.sortText = '1' + envVar; // Sort after document variables
            completions.push(item);
        });

        return completions;
    }

    private isInFunctionContext(document: vscode.TextDocument, position: vscode.Position): boolean {
        // Check if current position is inside a function definition
        for (let i = position.line; i >= 0; i--) {
            const line = document.lineAt(i).text;
            if (line.match(/<end-function>/)) {
                return false; // We're after a function end
            }
            if (line.match(/<function\s+[^>]+>/)) {
                return true; // We're inside a function
            }
        }
        return false;
    }

    private getFunctionContextCompletions(): vscode.CompletionItem[] {
        return [
            {
                label: 'return',
                insertText: 'return',
                documentation: 'Return from current function',
                kind: vscode.CompletionItemKind.Keyword
            },
            {
                label: 'set',
                insertText: 'set ${1:variable}="${2:value}"',
                documentation: 'Set a local variable',
                kind: vscode.CompletionItemKind.Variable,
                snippet: true
            }
        ].map(item => {
            const completion = new vscode.CompletionItem(item.label, item.kind);
            if (item.snippet) {
                completion.insertText = new vscode.SnippetString(item.insertText);
            } else {
                completion.insertText = item.insertText;
            }
            completion.documentation = new vscode.MarkdownString(item.documentation);
            return completion;
        });
    }

    private getSnippetCompletions(): vscode.CompletionItem[] {
        const snippets = [
            {
                label: 'login-template',
                insertText: [
                    '<function ${1:login}>',
                    '    <set user="${2:$1}">',
                    '    <set password="${3:${2:-}}">',
                    '    ',
                    '    <watch "login:" 60s>',
                    '    $user',
                    '    <enter>',
                    '    <watch "Password" 60s>',
                    '    $password',
                    '    <enter>',
                    '    ',
                    '    <switch timeout=10s poll=1s>',
                    '        <case "Login incorrect">',
                    '            <exit 1>',
                    '        <end-case>',
                    '        <case "${4:~]$}">',
                    '            <return>',
                    '        <end-case>',
                    '    <end-switch>',
                    '<end-function>'
                ].join('\n'),
                documentation: 'Template for a login function with error handling',
                kind: vscode.CompletionItemKind.Snippet
            },
            {
                label: 'switch-template',
                insertText: [
                    '<switch timeout=${1:30s} poll=${2:1s}>',
                    '    <case "${3:success_pattern}">',
                    '        ${4:# Success actions}',
                    '        <return>',
                    '    <end-case>',
                    '    <case "${5:error_pattern}">',
                    '        ${6:# Error handling}',
                    '        <exit 1>',
                    '    <end-case>',
                    '    <default>',
                    '        ${7:# Default timeout action}',
                    '        <exit 2>',
                    '    <end-default>',
                    '<end-switch>'
                ].join('\n'),
                documentation: 'Template for switch statement with success/error cases',
                kind: vscode.CompletionItemKind.Snippet
            },
            {
                label: 'conditional-template',
                insertText: [
                    '<if-found "${1:condition_text}" ${2:5s}>',
                    '    ${3:# Actions if condition is met}',
                    '<else>',
                    '    ${4:# Actions if condition is not met}',
                    '<end-if>'
                ].join('\n'),
                documentation: 'Template for conditional execution',
                kind: vscode.CompletionItemKind.Snippet
            }
        ];

        return snippets.map(snippet => {
            const item = new vscode.CompletionItem(snippet.label, snippet.kind);
            item.insertText = new vscode.SnippetString(snippet.insertText);
            item.documentation = new vscode.MarkdownString(snippet.documentation);
            item.sortText = '2' + snippet.label; // Sort after other completions
            return item;
        });
    }
}
