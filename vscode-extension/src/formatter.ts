import * as vscode from 'vscode';

export class QmpScript2DocumentFormatter implements vscode.DocumentFormattingEditProvider {

    provideDocumentFormattingEdits(
        document: vscode.TextDocument,
        options: vscode.FormattingOptions,
        token: vscode.CancellationToken
    ): vscode.TextEdit[] {
        const edits: vscode.TextEdit[] = [];
        let indentLevel = 0;
        const indentSize = 2; // Fixed 2-space indentation

        // Patterns for indentation control
        const increaseIndentPatterns = [
            /^\s*<(function|if-found|if-not-found|retry|repeat|while-found|while-not-found|switch)\b/,
            /^\s*<case\s/,
            /^\s*<default>/
        ];

        const decreaseIndentPatterns = [
            /^\s*<(end-function|end-if|end-switch|end-case|else)>/,
            /^\s*<case\s/,
            /^\s*<default>/
        ];

        const neutralPatterns = [
            /^\s*<else>/
        ];

        for (let i = 0; i < document.lineCount; i++) {
            const line = document.lineAt(i);
            const originalText = line.text;
            const trimmedText = originalText.trim();

            // Skip empty lines
            if (trimmedText === '') {
                continue;
            }

            // Check if this line should decrease indent before processing
            let shouldDecreaseIndent = false;
            for (const pattern of decreaseIndentPatterns) {
                if (pattern.test(trimmedText)) {
                    // Special handling for case/default - they reset to switch level
                    if (/^\s*<(case\s|default>)/.test(trimmedText)) {
                        // Case/default statements align with switch
                        indentLevel = Math.max(0, indentLevel - 1);
                    } else {
                        // Regular end statements
                        indentLevel = Math.max(0, indentLevel - 1);
                    }
                    shouldDecreaseIndent = true;
                    break;
                }
            }

            // Handle else - it should be at same level as if
            if (/^\s*<else>/.test(trimmedText)) {
                indentLevel = Math.max(0, indentLevel - 1);
            }

            // Calculate the expected indentation
            const expectedIndent = ' '.repeat(indentLevel * indentSize);
            const expectedText = expectedIndent + trimmedText;

            // If the line doesn't match expected formatting, add an edit
            if (originalText !== expectedText) {
                const range = new vscode.Range(
                    new vscode.Position(i, 0),
                    new vscode.Position(i, originalText.length)
                );
                edits.push(vscode.TextEdit.replace(range, expectedText));
            }

            // Check if this line should increase indent after processing
            for (const pattern of increaseIndentPatterns) {
                if (pattern.test(trimmedText)) {
                    // Special case for case/default - content inside should be indented
                    if (/^\s*<(case\s|default>)/.test(trimmedText)) {
                        indentLevel += 1;
                    } else {
                        indentLevel += 1;
                    }
                    break;
                }
            }

            // Handle else - content after else should be indented
            if (/^\s*<else>/.test(trimmedText)) {
                indentLevel += 1;
            }
        }

        return edits;
    }
}
