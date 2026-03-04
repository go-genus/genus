import * as vscode from 'vscode';
import * as cp from 'child_process';
import * as path from 'path';

let statusBarItem: vscode.StatusBarItem;

export function activate(context: vscode.ExtensionContext) {
    console.log('Genus ORM extension activated');

    // Status bar item
    statusBarItem = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Right, 100);
    statusBarItem.text = '$(database) Genus';
    statusBarItem.tooltip = 'Genus ORM';
    statusBarItem.command = 'genus.showSchema';
    statusBarItem.show();
    context.subscriptions.push(statusBarItem);

    // Register commands
    context.subscriptions.push(
        vscode.commands.registerCommand('genus.generateFields', generateFields),
        vscode.commands.registerCommand('genus.generateMigration', generateMigration),
        vscode.commands.registerCommand('genus.runMigration', runMigration),
        vscode.commands.registerCommand('genus.rollbackMigration', rollbackMigration),
        vscode.commands.registerCommand('genus.showSchema', showSchema),
        vscode.commands.registerCommand('genus.previewQuery', previewQuery),
        vscode.commands.registerCommand('genus.openPlayground', openPlayground),
        vscode.commands.registerCommand('genus.visualizeMigrations', visualizeMigrations)
    );

    // Register hover provider for SQL preview
    context.subscriptions.push(
        vscode.languages.registerHoverProvider('go', new GenusHoverProvider())
    );

    // Register completion provider
    context.subscriptions.push(
        vscode.languages.registerCompletionItemProvider('go', new GenusCompletionProvider(), '.')
    );

    // Watch for file saves to auto-generate fields
    context.subscriptions.push(
        vscode.workspace.onDidSaveTextDocument(onDocumentSave)
    );

    // Create tree view for schema
    const schemaProvider = new SchemaTreeProvider();
    vscode.window.registerTreeDataProvider('genusSchema', schemaProvider);

    // Create tree view for migrations
    const migrationsProvider = new MigrationsTreeProvider();
    vscode.window.registerTreeDataProvider('genusMigrations', migrationsProvider);
}

export function deactivate() {
    if (statusBarItem) {
        statusBarItem.dispose();
    }
}

// Commands

async function generateFields() {
    const editor = vscode.window.activeTextEditor;
    if (!editor) {
        vscode.window.showErrorMessage('No active editor');
        return;
    }

    const document = editor.document;
    if (document.languageId !== 'go') {
        vscode.window.showErrorMessage('Not a Go file');
        return;
    }

    const config = vscode.workspace.getConfiguration('genus');
    const modelsPath = config.get<string>('modelsPath') || './models';

    try {
        const result = await runGenusCommand(['generate', '-p', modelsPath]);
        vscode.window.showInformationMessage('Fields generated successfully');

        // Refresh the document
        const uri = document.uri;
        const newDoc = await vscode.workspace.openTextDocument(uri);
        await vscode.window.showTextDocument(newDoc);
    } catch (error) {
        vscode.window.showErrorMessage(`Failed to generate fields: ${error}`);
    }
}

async function generateMigration() {
    const name = await vscode.window.showInputBox({
        prompt: 'Migration name',
        placeHolder: 'e.g., add_users_table'
    });

    if (!name) {
        return;
    }

    const config = vscode.workspace.getConfiguration('genus');
    const migrationsPath = config.get<string>('migrationsPath') || './migrations';

    try {
        await runGenusCommand(['migrate', 'create', name, '-p', migrationsPath]);
        vscode.window.showInformationMessage(`Migration '${name}' created`);

        // Open the new migration file
        const files = await vscode.workspace.findFiles(`${migrationsPath}/*${name}*.go`);
        if (files.length > 0) {
            const doc = await vscode.workspace.openTextDocument(files[0]);
            await vscode.window.showTextDocument(doc);
        }
    } catch (error) {
        vscode.window.showErrorMessage(`Failed to create migration: ${error}`);
    }
}

async function runMigration() {
    const config = vscode.workspace.getConfiguration('genus');
    const dsn = config.get<string>('databaseUrl');
    const driver = config.get<string>('driver') || 'postgres';

    if (!dsn) {
        vscode.window.showErrorMessage('Database URL not configured. Set genus.databaseUrl in settings.');
        return;
    }

    try {
        statusBarItem.text = '$(sync~spin) Running migrations...';
        await runGenusCommand(['migrate', 'up', '--dsn', dsn, '--driver', driver]);
        statusBarItem.text = '$(database) Genus';
        vscode.window.showInformationMessage('Migrations completed');
    } catch (error) {
        statusBarItem.text = '$(database) Genus';
        vscode.window.showErrorMessage(`Migration failed: ${error}`);
    }
}

async function rollbackMigration() {
    const config = vscode.workspace.getConfiguration('genus');
    const dsn = config.get<string>('databaseUrl');
    const driver = config.get<string>('driver') || 'postgres';

    if (!dsn) {
        vscode.window.showErrorMessage('Database URL not configured');
        return;
    }

    const confirm = await vscode.window.showWarningMessage(
        'Are you sure you want to rollback the last migration?',
        'Yes', 'No'
    );

    if (confirm !== 'Yes') {
        return;
    }

    try {
        statusBarItem.text = '$(sync~spin) Rolling back...';
        await runGenusCommand(['migrate', 'down', '--dsn', dsn, '--driver', driver]);
        statusBarItem.text = '$(database) Genus';
        vscode.window.showInformationMessage('Rollback completed');
    } catch (error) {
        statusBarItem.text = '$(database) Genus';
        vscode.window.showErrorMessage(`Rollback failed: ${error}`);
    }
}

async function showSchema() {
    const config = vscode.workspace.getConfiguration('genus');
    const dsn = config.get<string>('databaseUrl');
    const driver = config.get<string>('driver') || 'postgres';

    if (!dsn) {
        vscode.window.showErrorMessage('Database URL not configured');
        return;
    }

    try {
        const schema = await runGenusCommand(['schema', '--dsn', dsn, '--driver', driver, '--format', 'json']);

        // Create a webview to display schema
        const panel = vscode.window.createWebviewPanel(
            'genusSchema',
            'Database Schema',
            vscode.ViewColumn.Two,
            { enableScripts: true }
        );

        panel.webview.html = getSchemaWebviewContent(schema);
    } catch (error) {
        vscode.window.showErrorMessage(`Failed to load schema: ${error}`);
    }
}

async function previewQuery() {
    const editor = vscode.window.activeTextEditor;
    if (!editor) {
        return;
    }

    const selection = editor.selection;
    const text = editor.document.getText(selection.isEmpty ? undefined : selection);

    // Parse Genus query builder calls and convert to SQL
    const sql = parseGenusQueryToSQL(text);

    if (sql) {
        const panel = vscode.window.createWebviewPanel(
            'genusSqlPreview',
            'SQL Preview',
            vscode.ViewColumn.Two,
            {}
        );

        panel.webview.html = `
            <html>
            <body style="padding: 20px; font-family: monospace;">
                <h3>Generated SQL:</h3>
                <pre style="background: #1e1e1e; color: #d4d4d4; padding: 15px; border-radius: 5px;">
${sql}
                </pre>
            </body>
            </html>
        `;
    }
}

async function openPlayground() {
    const config = vscode.workspace.getConfiguration('genus');
    const dsn = config.get<string>('databaseUrl');

    if (!dsn) {
        vscode.window.showErrorMessage('Database URL not configured');
        return;
    }

    // Start the playground server
    try {
        const result = await runGenusCommand(['playground', '--port', '8765']);
        vscode.env.openExternal(vscode.Uri.parse('http://localhost:8765'));
    } catch (error) {
        vscode.window.showErrorMessage(`Failed to start playground: ${error}`);
    }
}

async function visualizeMigrations() {
    const config = vscode.workspace.getConfiguration('genus');
    const migrationsPath = config.get<string>('migrationsPath') || './migrations';

    try {
        const result = await runGenusCommand(['migrate', 'visualize', '-p', migrationsPath, '--format', 'json']);

        const panel = vscode.window.createWebviewPanel(
            'genusMigrations',
            'Migrations Visualizer',
            vscode.ViewColumn.One,
            { enableScripts: true }
        );

        panel.webview.html = getMigrationsWebviewContent(result);
    } catch (error) {
        vscode.window.showErrorMessage(`Failed to visualize migrations: ${error}`);
    }
}

// Hover Provider

class GenusHoverProvider implements vscode.HoverProvider {
    provideHover(document: vscode.TextDocument, position: vscode.Position): vscode.ProviderResult<vscode.Hover> {
        const config = vscode.workspace.getConfiguration('genus');
        if (!config.get<boolean>('showSqlPreview')) {
            return null;
        }

        const line = document.lineAt(position.line);
        const text = line.text;

        // Check if this looks like a Genus query
        if (text.includes('genus.Table') || text.includes('.Where(') || text.includes('.Find(')) {
            // Get the full query (might span multiple lines)
            let queryText = '';
            let lineNum = position.line;

            // Go back to find the start of the query
            while (lineNum >= 0) {
                const l = document.lineAt(lineNum).text;
                queryText = l + '\n' + queryText;
                if (l.includes('genus.Table')) {
                    break;
                }
                lineNum--;
            }

            // Go forward to find the end
            lineNum = position.line + 1;
            while (lineNum < document.lineCount) {
                const l = document.lineAt(lineNum).text;
                queryText += '\n' + l;
                if (l.includes('Find(') || l.includes('First(') || l.includes('One(')) {
                    break;
                }
                lineNum++;
            }

            const sql = parseGenusQueryToSQL(queryText);
            if (sql) {
                return new vscode.Hover(
                    new vscode.MarkdownString(`**Generated SQL:**\n\`\`\`sql\n${sql}\n\`\`\``)
                );
            }
        }

        return null;
    }
}

// Completion Provider

class GenusCompletionProvider implements vscode.CompletionItemProvider {
    provideCompletionItems(
        document: vscode.TextDocument,
        position: vscode.Position
    ): vscode.ProviderResult<vscode.CompletionItem[]> {
        const linePrefix = document.lineAt(position).text.substr(0, position.character);

        // After Table[Model](db).
        if (linePrefix.endsWith('.')) {
            return [
                this.createCompletion('Where', 'Add WHERE condition', 'Where(${1:condition})'),
                this.createCompletion('Find', 'Execute query and return all results', 'Find(ctx)'),
                this.createCompletion('First', 'Execute query and return first result', 'First(ctx)'),
                this.createCompletion('OrderBy', 'Add ORDER BY clause', 'OrderBy("${1:column}")'),
                this.createCompletion('OrderByDesc', 'Add ORDER BY DESC clause', 'OrderByDesc("${1:column}")'),
                this.createCompletion('Limit', 'Limit number of results', 'Limit(${1:10})'),
                this.createCompletion('Offset', 'Skip N results', 'Offset(${1:0})'),
                this.createCompletion('Preload', 'Eager load relationship', 'Preload("${1:RelationName}")'),
                this.createCompletion('Join', 'Add JOIN clause', 'Join("${1:table}", "${2:condition}")'),
                this.createCompletion('LeftJoin', 'Add LEFT JOIN clause', 'LeftJoin("${1:table}", "${2:condition}")'),
                this.createCompletion('Aggregate', 'Start aggregate query', 'Aggregate()'),
                this.createCompletion('Select', 'Select specific columns', 'Select("${1:columns}")'),
                this.createCompletion('Distinct', 'Select distinct values', 'Distinct()'),
            ];
        }

        // After a field like UserFields.
        if (linePrefix.match(/\w+Fields\.$/)) {
            // Would need to parse the model to get actual fields
            // For now, return common suggestions
            return [
                this.createCompletion('ID', 'ID field', 'ID'),
                this.createCompletion('Name', 'Name field', 'Name'),
                this.createCompletion('Email', 'Email field', 'Email'),
                this.createCompletion('CreatedAt', 'CreatedAt field', 'CreatedAt'),
                this.createCompletion('UpdatedAt', 'UpdatedAt field', 'UpdatedAt'),
            ];
        }

        // After a field method like .Eq(
        if (linePrefix.match(/\.\w+Field\.$/)) {
            return [
                this.createCompletion('Eq', 'Equal to', 'Eq(${1:value})'),
                this.createCompletion('Ne', 'Not equal to', 'Ne(${1:value})'),
                this.createCompletion('Gt', 'Greater than', 'Gt(${1:value})'),
                this.createCompletion('Gte', 'Greater than or equal', 'Gte(${1:value})'),
                this.createCompletion('Lt', 'Less than', 'Lt(${1:value})'),
                this.createCompletion('Lte', 'Less than or equal', 'Lte(${1:value})'),
                this.createCompletion('In', 'In list', 'In(${1:values})'),
                this.createCompletion('NotIn', 'Not in list', 'NotIn(${1:values})'),
                this.createCompletion('Like', 'LIKE pattern', 'Like("${1:%pattern%}")'),
                this.createCompletion('IsNull', 'Is NULL', 'IsNull()'),
                this.createCompletion('IsNotNull', 'Is NOT NULL', 'IsNotNull()'),
                this.createCompletion('Between', 'Between values', 'Between(${1:min}, ${2:max})'),
            ];
        }

        return [];
    }

    private createCompletion(label: string, detail: string, insertText: string): vscode.CompletionItem {
        const item = new vscode.CompletionItem(label, vscode.CompletionItemKind.Method);
        item.detail = detail;
        item.insertText = new vscode.SnippetString(insertText);
        return item;
    }
}

// Tree Providers

class SchemaTreeProvider implements vscode.TreeDataProvider<SchemaItem> {
    private _onDidChangeTreeData = new vscode.EventEmitter<SchemaItem | undefined>();
    readonly onDidChangeTreeData = this._onDidChangeTreeData.event;

    refresh(): void {
        this._onDidChangeTreeData.fire(undefined);
    }

    getTreeItem(element: SchemaItem): vscode.TreeItem {
        return element;
    }

    async getChildren(element?: SchemaItem): Promise<SchemaItem[]> {
        if (!element) {
            // Root level - return tables
            // In real implementation, query database schema
            return [
                new SchemaItem('users', vscode.TreeItemCollapsibleState.Collapsed, 'table'),
                new SchemaItem('posts', vscode.TreeItemCollapsibleState.Collapsed, 'table'),
                new SchemaItem('comments', vscode.TreeItemCollapsibleState.Collapsed, 'table'),
            ];
        }

        // Return columns for table
        return [
            new SchemaItem('id (bigint)', vscode.TreeItemCollapsibleState.None, 'column'),
            new SchemaItem('name (varchar)', vscode.TreeItemCollapsibleState.None, 'column'),
            new SchemaItem('created_at (timestamp)', vscode.TreeItemCollapsibleState.None, 'column'),
        ];
    }
}

class SchemaItem extends vscode.TreeItem {
    constructor(
        public readonly label: string,
        public readonly collapsibleState: vscode.TreeItemCollapsibleState,
        public readonly itemType: 'table' | 'column'
    ) {
        super(label, collapsibleState);
        this.iconPath = new vscode.ThemeIcon(itemType === 'table' ? 'table' : 'symbol-field');
    }
}

class MigrationsTreeProvider implements vscode.TreeDataProvider<MigrationItem> {
    getTreeItem(element: MigrationItem): vscode.TreeItem {
        return element;
    }

    async getChildren(): Promise<MigrationItem[]> {
        // In real implementation, read migrations from files
        return [
            new MigrationItem('001_create_users', 'applied'),
            new MigrationItem('002_create_posts', 'applied'),
            new MigrationItem('003_add_comments', 'pending'),
        ];
    }
}

class MigrationItem extends vscode.TreeItem {
    constructor(
        public readonly label: string,
        public readonly status: 'applied' | 'pending'
    ) {
        super(label, vscode.TreeItemCollapsibleState.None);
        this.description = status;
        this.iconPath = new vscode.ThemeIcon(
            status === 'applied' ? 'check' : 'circle-outline'
        );
    }
}

// Helper Functions

async function runGenusCommand(args: string[]): Promise<string> {
    return new Promise((resolve, reject) => {
        const workspaceFolder = vscode.workspace.workspaceFolders?.[0];
        const cwd = workspaceFolder?.uri.fsPath || process.cwd();

        const proc = cp.spawn('genus', args, { cwd });
        let stdout = '';
        let stderr = '';

        proc.stdout.on('data', (data) => {
            stdout += data.toString();
        });

        proc.stderr.on('data', (data) => {
            stderr += data.toString();
        });

        proc.on('close', (code) => {
            if (code === 0) {
                resolve(stdout);
            } else {
                reject(new Error(stderr || `Process exited with code ${code}`));
            }
        });

        proc.on('error', (err) => {
            reject(err);
        });
    });
}

function parseGenusQueryToSQL(code: string): string | null {
    // Simple parser - in real implementation would be more sophisticated
    let sql = 'SELECT * FROM ';

    // Extract table name
    const tableMatch = code.match(/Table\[(\w+)\]/);
    if (tableMatch) {
        sql += tableMatch[1].toLowerCase() + 's';
    } else {
        return null;
    }

    // Extract WHERE conditions
    const whereMatches = code.matchAll(/Where\((\w+)Fields\.(\w+)\.(\w+)\((.*?)\)\)/g);
    const conditions: string[] = [];
    for (const match of whereMatches) {
        const field = match[2].toLowerCase();
        const op = match[3];
        const value = match[4];

        let sqlOp = '=';
        switch (op) {
            case 'Eq': sqlOp = '='; break;
            case 'Ne': sqlOp = '!='; break;
            case 'Gt': sqlOp = '>'; break;
            case 'Gte': sqlOp = '>='; break;
            case 'Lt': sqlOp = '<'; break;
            case 'Lte': sqlOp = '<='; break;
            case 'Like': sqlOp = 'LIKE'; break;
        }

        conditions.push(`${field} ${sqlOp} ${value}`);
    }

    if (conditions.length > 0) {
        sql += ' WHERE ' + conditions.join(' AND ');
    }

    // Extract ORDER BY
    const orderMatch = code.match(/OrderBy(?:Desc)?\("(\w+)"\)/);
    if (orderMatch) {
        sql += ' ORDER BY ' + orderMatch[1];
        if (code.includes('OrderByDesc')) {
            sql += ' DESC';
        }
    }

    // Extract LIMIT
    const limitMatch = code.match(/Limit\((\d+)\)/);
    if (limitMatch) {
        sql += ' LIMIT ' + limitMatch[1];
    }

    return sql;
}

function getSchemaWebviewContent(schemaJson: string): string {
    return `
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: var(--vscode-font-family); padding: 20px; }
        .table { margin-bottom: 20px; }
        .table-name { font-weight: bold; font-size: 16px; margin-bottom: 10px; }
        .column { padding: 5px 10px; border-bottom: 1px solid #333; }
        .column-name { font-weight: 500; }
        .column-type { color: #888; margin-left: 10px; }
    </style>
</head>
<body>
    <h2>Database Schema</h2>
    <div id="schema"></div>
    <script>
        const schema = ${schemaJson || '[]'};
        const container = document.getElementById('schema');

        schema.forEach(table => {
            const div = document.createElement('div');
            div.className = 'table';
            div.innerHTML = '<div class="table-name">' + table.name + '</div>';

            table.columns.forEach(col => {
                div.innerHTML += '<div class="column"><span class="column-name">' +
                    col.name + '</span><span class="column-type">' + col.type + '</span></div>';
            });

            container.appendChild(div);
        });
    </script>
</body>
</html>
    `;
}

function getMigrationsWebviewContent(migrationsJson: string): string {
    return `
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: var(--vscode-font-family); padding: 20px; }
        .migration { display: flex; align-items: center; padding: 10px; border-bottom: 1px solid #333; }
        .migration.applied { opacity: 0.7; }
        .migration.pending { background: #1e3a5f; }
        .status { width: 20px; height: 20px; border-radius: 50%; margin-right: 10px; }
        .status.applied { background: #4caf50; }
        .status.pending { background: #ff9800; }
        .name { flex: 1; }
        .arrow { margin: 0 20px; color: #666; }

        /* DAG visualization */
        svg { width: 100%; height: 400px; }
        .node circle { fill: #1e88e5; stroke: #fff; stroke-width: 2px; }
        .node text { fill: #fff; font-size: 12px; }
        .link { fill: none; stroke: #666; stroke-width: 2px; }
    </style>
</head>
<body>
    <h2>Migrations DAG</h2>
    <svg id="dag"></svg>

    <h2>Migration List</h2>
    <div id="migrations"></div>

    <script>
        const migrations = ${migrationsJson || '[]'};
        const container = document.getElementById('migrations');

        migrations.forEach((m, i) => {
            const div = document.createElement('div');
            div.className = 'migration ' + m.status;
            div.innerHTML =
                '<div class="status ' + m.status + '"></div>' +
                '<div class="name">' + m.version + ' - ' + m.description + '</div>';

            if (i < migrations.length - 1) {
                div.innerHTML += '<div class="arrow">→</div>';
            }

            container.appendChild(div);
        });
    </script>
</body>
</html>
    `;
}

async function onDocumentSave(document: vscode.TextDocument) {
    if (document.languageId !== 'go') {
        return;
    }

    const config = vscode.workspace.getConfiguration('genus');
    if (!config.get<boolean>('autoGenerateFields')) {
        return;
    }

    // Check if this is a model file
    const text = document.getText();
    if (text.includes('core.Model') || text.includes('genus:')) {
        try {
            await runGenusCommand(['generate', '-f', document.uri.fsPath]);
        } catch (error) {
            // Silent fail for auto-generation
            console.error('Auto-generate fields failed:', error);
        }
    }
}
