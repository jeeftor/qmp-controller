import * as vscode from 'vscode';
import { QmpScript2DocumentFormatter } from './formatter';

export function activate(context: vscode.ExtensionContext) {
    // Register the document formatter
    const formatter = new QmpScript2DocumentFormatter();
    const formatterDisposable = vscode.languages.registerDocumentFormattingEditProvider(
        'qmp-script2',
        formatter
    );

    // Register format-on-save handler
    const saveDisposable = vscode.workspace.onWillSaveTextDocument((event) => {
        const document = event.document;

        // Only format QMP Script2 files
        if (document.languageId !== 'qmp-script2') {
            return;
        }

        // Check if format-on-save is enabled
        const config = vscode.workspace.getConfiguration('qmp-script2');
        const formatOnSave = config.get<boolean>('formatOnSave', true);

        if (formatOnSave) {
            // Apply formatting edits before save
            const edits = formatter.provideDocumentFormattingEdits(
                document,
                { insertSpaces: true, tabSize: 2 },
                new vscode.CancellationTokenSource().token
            );

            if (edits && edits.length > 0) {
                const workspaceEdit = new vscode.WorkspaceEdit();
                workspaceEdit.set(document.uri, edits);

                event.waitUntil(
                    vscode.workspace.applyEdit(workspaceEdit)
                );
            }
        }
    });

    context.subscriptions.push(formatterDisposable, saveDisposable);

    console.log('QMP Script2 formatter with format-on-save is now active!');
}

export function deactivate() {}
