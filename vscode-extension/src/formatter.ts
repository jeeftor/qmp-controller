import * as vscode from 'vscode';

export class QmpScript2DocumentFormatter implements vscode.DocumentFormattingEditProvider, vscode.DocumentRangeFormattingEditProvider {

    provideDocumentFormattingEdits(
        document: vscode.TextDocument,
        options: vscode.FormattingOptions,
        token: vscode.CancellationToken
    ): vscode.TextEdit[] {
        const edits: vscode.TextEdit[] = [];
        let indentLevel = 0;
        const indentSize = options.tabSize || 2;
        const useSpaces = options.insertSpaces;

        // Stack to track nested structures for proper indentation
        const indentStack: string[] = [];

        // Patterns for indentation control
        const blockStartPatterns = [
            { pattern: /^\s*<function\s+[^>]+>/, type: 'function', increase: true },
            { pattern: /^\s*<if-found\s+[^>]+>/, type: 'if', increase: true },
            { pattern: /^\s*<if-not-found\s+[^>]+>/, type: 'if', increase: true },
            { pattern: /^\s*<while-found\s+[^>]+>/, type: 'while', increase: true },
            { pattern: /^\s*<while-not-found\s+[^>]+>/, type: 'while', increase: true },
            { pattern: /^\s*<switch(\s+[^>]*)?>/, type: 'switch', increase: true },
            { pattern: /^\s*<case\s+[^>]+>/, type: 'case', increase: true },
            { pattern: /^\s*<default>/, type: 'default', increase: true },
            { pattern: /^\s*<retry\s+\d+>/, type: 'retry', increase: true },
            { pattern: /^\s*<repeat\s+\d+>/, type: 'repeat', increase: true }
        ];

        const blockEndPatterns = [
            { pattern: /^\s*<end-function>/, type: 'function' },
            { pattern: /^\s*<end-if>/, type: 'if' },
            { pattern: /^\s*<end-while>/, type: 'while' },
            { pattern: /^\s*<end-switch>/, type: 'switch' },
            { pattern: /^\s*<end-case>/, type: 'case' },
            { pattern: /^\s*<end-default>/, type: 'default' }
        ];

        const specialPatterns = [
            { pattern: /^\s*<else>/, type: 'else' },
            { pattern: /^\s*<case\s+[^>]+>/, type: 'case-start' },
            { pattern: /^\s*<default>/, type: 'default-start' }
        ];

        console.log('QMP Script2 Formatter: Starting formatting process');

        for (let i = 0; i < document.lineCount; i++) {
            const line = document.lineAt(i);
            const originalText = line.text;
            const trimmedText = originalText.trim();

            // Skip completely empty lines
            if (trimmedText === '') {
                continue;
            }

            let currentIndentLevel = indentLevel;
            let shouldDecreaseIndent = false;
            let shouldIncreaseAfter = false;

            // Check for block end patterns first
            for (const endPattern of blockEndPatterns) {
                if (endPattern.pattern.test(trimmedText)) {
                    // Pop from stack and decrease indent
                    if (indentStack.length > 0 && indentStack[indentStack.length - 1] === endPattern.type) {
                        indentStack.pop();
                        indentLevel = Math.max(0, indentLevel - 1);
                        currentIndentLevel = indentLevel;
                        shouldDecreaseIndent = true;
                    }
                    break;
                }
            }

            // Handle special cases like else, case, default
            for (const special of specialPatterns) {
                if (special.pattern.test(trimmedText)) {
                    if (special.type === 'else') {
                        // Else should be at same level as if, then increase for content
                        currentIndentLevel = Math.max(0, indentLevel - 1);
                        shouldIncreaseAfter = true;
                    } else if (special.type === 'case-start' || special.type === 'default-start') {
                        // Cases should be at switch level
                        if (indentStack.length > 0 && indentStack[indentStack.length - 1] === 'case') {
                            // End previous case
                            indentStack.pop();
                            indentLevel = Math.max(0, indentLevel - 1);
                        }
                        currentIndentLevel = indentLevel;
                        shouldIncreaseAfter = true;
                    }
                    break;
                }
            }

            // Generate the correct indentation
            const indentString = useSpaces ?
                ' '.repeat(currentIndentLevel * indentSize) :
                '\t'.repeat(currentIndentLevel);

            const expectedText = indentString + trimmedText;

            // Add edit if the line doesn't match expected formatting
            if (originalText !== expectedText) {
                const range = new vscode.Range(
                    new vscode.Position(i, 0),
                    new vscode.Position(i, originalText.length)
                );
                edits.push(vscode.TextEdit.replace(range, expectedText));
                console.log(`QMP Script2 Formatter: Line ${i + 1}: "${originalText.trim()}" -> indent level ${currentIndentLevel}`);
            }

            // Check for block start patterns and handle special increases
            if (shouldIncreaseAfter) {
                indentLevel = currentIndentLevel + 1;
                if (trimmedText.match(/^\s*<case\s+[^>]+>/)) {
                    indentStack.push('case');
                } else if (trimmedText.match(/^\s*<default>/)) {
                    indentStack.push('default');
                }
            } else {
                for (const startPattern of blockStartPatterns) {
                    if (startPattern.pattern.test(trimmedText) && startPattern.increase) {
                        indentLevel = currentIndentLevel + 1;
                        indentStack.push(startPattern.type);
                        break;
                    }
                }
            }
        }

        console.log(`QMP Script2 Formatter: Generated ${edits.length} formatting edits`);
        return edits;
    }

    provideDocumentRangeFormattingEdits(
        document: vscode.TextDocument,
        range: vscode.Range,
        options: vscode.FormattingOptions,
        token: vscode.CancellationToken
    ): vscode.TextEdit[] {
        // For range formatting, we'll format the entire document and filter to the range
        const allEdits = this.provideDocumentFormattingEdits(document, options, token);

        // Filter edits to only include those within the specified range
        return allEdits.filter(edit =>
            range.contains(edit.range) || range.intersection(edit.range)
        );
    }
}
