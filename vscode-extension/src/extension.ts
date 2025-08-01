import * as vscode from 'vscode';
import { QmpScript2DocumentFormatter } from './formatter';
import { QmpScript2CompletionProvider } from './completionProvider';
import { QmpScript2HoverProvider } from './hoverProvider';
import { QmpScript2SignatureHelpProvider } from './signatureProvider';

export function activate(context: vscode.ExtensionContext) {
    const languageId = 'qmp-script2';

    console.log('QMP Script2 extension is being activated...');

    // Register the document formatter
    const formatter = new QmpScript2DocumentFormatter();
    const formatterDisposable = vscode.languages.registerDocumentFormattingEditProvider(
        { scheme: 'file', language: languageId },
        formatter
    );

    // Register range formatter as well
    const rangeFormatterDisposable = vscode.languages.registerDocumentRangeFormattingEditProvider(
        { scheme: 'file', language: languageId },
        formatter
    );

    // Register IntelliSense providers
    const completionProvider = new QmpScript2CompletionProvider();
    const completionDisposable = vscode.languages.registerCompletionItemProvider(
        { scheme: 'file', language: languageId },
        completionProvider,
        '<', '$', '"', ' '  // Trigger characters
    );

    const hoverProvider = new QmpScript2HoverProvider();
    const hoverDisposable = vscode.languages.registerHoverProvider(
        { scheme: 'file', language: languageId },
        hoverProvider
    );

    const signatureProvider = new QmpScript2SignatureHelpProvider();
    const signatureDisposable = vscode.languages.registerSignatureHelpProvider(
        { scheme: 'file', language: languageId },
        signatureProvider,
        ' ', '"', '='  // Trigger characters
    );

    // Register manual format command
    const formatCommandDisposable = vscode.commands.registerCommand('qmp-script2.format', () => {
        const editor = vscode.window.activeTextEditor;
        if (editor && editor.document.languageId === languageId) {
            vscode.commands.executeCommand('editor.action.formatDocument');
        }
    });

    // Enhanced format-on-save handler
    const saveDisposable = vscode.workspace.onWillSaveTextDocument(async (event) => {
        const document = event.document;

        // Only format QMP Script2 files
        if (document.languageId !== languageId) {
            return;
        }

        // Check if format-on-save is enabled
        const config = vscode.workspace.getConfiguration('qmp-script2');
        const formatOnSave = config.get<boolean>('formatOnSave', true);

        console.log(`QMP Script2: Format on save enabled: ${formatOnSave}`);

        if (formatOnSave) {
            try {
                // Apply formatting edits before save
                const edits = formatter.provideDocumentFormattingEdits(
                    document,
                    {
                        insertSpaces: true,
                        tabSize: 2
                    },
                    new vscode.CancellationTokenSource().token
                );

                if (edits && edits.length > 0) {
                    console.log(`QMP Script2: Applying ${edits.length} formatting edits`);
                    const workspaceEdit = new vscode.WorkspaceEdit();
                    workspaceEdit.set(document.uri, edits);

                    event.waitUntil(
                        vscode.workspace.applyEdit(workspaceEdit).then(success => {
                            console.log(`QMP Script2: Format on save ${success ? 'successful' : 'failed'}`);
                            return success;
                        })
                    );
                } else {
                    console.log('QMP Script2: No formatting changes needed');
                }
            } catch (error) {
                console.error('QMP Script2: Error during format on save:', error);
                vscode.window.showWarningMessage(`QMP Script2 formatting error: ${error}`);
            }
        }
    });

    // Register configuration change handler
    const configChangeDisposable = vscode.workspace.onDidChangeConfiguration((event) => {
        if (event.affectsConfiguration('qmp-script2')) {
            console.log('QMP Script2: Configuration changed');
            const config = vscode.workspace.getConfiguration('qmp-script2');
            console.log('Current config:', {
                formatOnSave: config.get('formatOnSave'),
                enableRainbowBrackets: config.get('enableRainbowBrackets'),
                enableSemanticHighlighting: config.get('enableSemanticHighlighting')
            });
        }
    });

    // Register status bar item to show formatter status
    const statusBarItem = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Right, 100);
    statusBarItem.text = "$(symbol-color) QMP2";
    statusBarItem.tooltip = "QMP Script2 extension active - Click to format document";
    statusBarItem.command = 'qmp-script2.format';

    // Show status bar item only for QMP Script2 files
    const updateStatusBar = () => {
        const editor = vscode.window.activeTextEditor;
        if (editor && editor.document.languageId === languageId) {
            statusBarItem.show();
        } else {
            statusBarItem.hide();
        }
    };

    // Update status bar on editor change
    const editorChangeDisposable = vscode.window.onDidChangeActiveTextEditor(updateStatusBar);
    updateStatusBar(); // Initial update

    // Register all disposables
    context.subscriptions.push(
        formatterDisposable,
        rangeFormatterDisposable,
        completionDisposable,
        hoverDisposable,
        signatureDisposable,
        formatCommandDisposable,
        saveDisposable,
        configChangeDisposable,
        statusBarItem,
        editorChangeDisposable
    );

    console.log('QMP Script2 extension with enhanced IntelliSense and formatting is now active!');
}

export function deactivate() {
    console.log('QMP Script2 extension is being deactivated');
}
