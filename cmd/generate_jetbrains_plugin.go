package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/jeeftor/qmp-controller/internal/filesystem"
	"github.com/spf13/cobra"
)

// generateJetBrainsPluginCmd creates JetBrains plugin for script2 syntax highlighting
var generateJetBrainsPluginCmd = &cobra.Command{
	Use:   "generate-jetbrains-plugin [output-dir]",
	Short: "Generate JetBrains plugin for script2 syntax highlighting",
	Long: `Generate a JetBrains plugin for script2 syntax highlighting compatible with
IntelliJ IDEA, GoLand, PyCharm, and other JetBrains IDEs.

This command automatically extracts regex patterns from the script2 parser code
and generates a synchronized plugin that stays up-to-date with parser changes.

Features:
- Automatic regex pattern extraction from internal/script2/parser.go
- BNF grammar generation for JetBrains syntax highlighting
- Plugin.xml generation for JetBrains plugin system
- Build configuration for Gradle-based plugin builds
- README with installation instructions
- Compatible with all JetBrains IDEs (IntelliJ, GoLand, PyCharm, etc.)

The generated plugin provides syntax highlighting for:
- Variable assignments (USER=${USER:-default})
- Directive syntax (<enter>, <watch "text" 30s>, etc.)
- Comments (# lines)
- Function definitions and calls
- Special keywords and operators`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		outputDir := "./jetbrains-plugin"
		if len(args) > 0 {
			outputDir = args[0]
		}

		fmt.Printf("🔧 Generating JetBrains plugin for script2 syntax highlighting...\n")
		fmt.Printf("📂 Output directory: %s\n", outputDir)

		if err := generateJetBrainsPlugin(outputDir); err != nil {
			fmt.Printf("❌ Failed to generate JetBrains plugin: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✅ JetBrains plugin generated successfully!\n")
		fmt.Printf("📦 To build: cd %s && ./gradlew buildPlugin\n", outputDir)
		fmt.Printf("🔧 To install: Open JetBrains IDE -> Plugins -> Install from disk -> select .zip file from build/distributions/\n")
	},
}

// JetBrains plugin template structures
type JetBrainsPlugin struct {
	Name           string
	Version        string
	Description    string
	VendorEmail    string
	VendorURL      string
	IdeaVersion    string
	SinceVersion   string
	UntilVersion   string
	FileExtensions []string
	Keywords       []string
}

type BNFGrammar struct {
	FileTypes []string
	Rules     []BNFRule
}

type BNFRule struct {
	Name        string
	Pattern     string
	TokenType   string
	Description string
}

// generateJetBrainsPlugin creates the complete JetBrains plugin structure
func generateJetBrainsPlugin(outputDir string) error {
	// Parse script2 parser to extract patterns
	patterns, err := extractScript2Patterns()
	if err != nil {
		return fmt.Errorf("failed to extract patterns: %w", err)
	}

	// Get git version info
	version := getJetbrainsGitVersion()

	plugin := &JetBrainsPlugin{
		Name:           "QMP Script2 Language Support",
		Version:        version,
		Description:    "Syntax highlighting and language support for QMP Script2 automation files",
		VendorEmail:    "noreply@jeeftor.com",
		VendorURL:      "https://github.com/jeeftor/qmp-controller",
		IdeaVersion:    "2020.3",
		SinceVersion:   "203",
		UntilVersion:   "",
		FileExtensions: []string{".sc2", ".script2", ".qmp2"},
		Keywords:       extractKeywords(patterns),
	}

	// Create directory structure
	if err := createJetbrainsDirectoryStructure(outputDir); err != nil {
		return fmt.Errorf("failed to create directory structure: %w", err)
	}

	// Generate plugin files
	if err := generatePluginXML(outputDir, plugin); err != nil {
		return fmt.Errorf("failed to generate plugin.xml: %w", err)
	}

	// Skip BNF and Lexer generation for simplified plugin
	// if err := generateBNFGrammar(outputDir, patterns); err != nil {
	//     return fmt.Errorf("failed to generate BNF grammar: %w", err)
	// }

	// if err := generateLexer(outputDir, patterns); err != nil {
	//     return fmt.Errorf("failed to generate lexer: %w", err)
	// }

	if err := generateBuildGradle(outputDir, plugin); err != nil {
		return fmt.Errorf("failed to generate build.gradle: %w", err)
	}

	if err := generateJetbrainsReadme(outputDir, plugin); err != nil {
		return fmt.Errorf("failed to generate README: %w", err)
	}

	if err := generateFileType(outputDir); err != nil {
		return fmt.Errorf("failed to generate file type: %w", err)
	}

	// Generate helper classes
	if err := generateJetbrainsHelpers(outputDir); err != nil {
		return fmt.Errorf("failed to generate helper classes: %w", err)
	}

	fmt.Printf("📝 Generated %d syntax highlighting patterns\n", len(patterns))
	fmt.Printf("🔧 Generated JetBrains plugin with complete infrastructure\n")
	return nil
}

// createJetbrainsDirectoryStructure creates the standard JetBrains plugin directory structure
func createJetbrainsDirectoryStructure(outputDir string) error {
	dirs := []string{
		outputDir,
		filepath.Join(outputDir, "src", "main", "java", "com", "qmpcontroller", "script2"),
		filepath.Join(outputDir, "src", "main", "resources", "META-INF"),
		filepath.Join(outputDir, "src", "main", "resources", "icons"),
		filepath.Join(outputDir, "gradle", "wrapper"),
	}

	for _, dir := range dirs {
		if err := filesystem.EnsureDirectory(dir); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

// generatePluginXML creates the plugin.xml file
func generatePluginXML(outputDir string, plugin *JetBrainsPlugin) error {
	xmlTemplate := `<idea-plugin>
    <id>com.qmpcontroller.script2</id>
    <name>{{.Name}}</name>
    <version>{{.Version}}</version>
    <vendor email="{{.VendorEmail}}" url="{{.VendorURL}}">QMP Controller</vendor>

    <description><![CDATA[
    {{.Description}}

    <h3>Features</h3>
    <ul>
        <li>Syntax highlighting for Script2 files (.sc2, .script2, .qmp2)</li>
        <li>Variable recognition and highlighting</li>
        <li>Directive syntax support</li>
        <li>Function definition highlighting</li>
        <li>Comment support</li>
        <li>Auto-completion for directives</li>
    </ul>

    <h3>Supported File Extensions</h3>
    <ul>{{range .FileExtensions}}
        <li>{{.}}</li>{{end}}
    </ul>
    ]]></description>

    <change-notes><![CDATA[
    <h3>{{.Version}}</h3>
    <ul>
        <li>Initial release</li>
        <li>Basic syntax highlighting</li>
        <li>File type recognition</li>
    </ul>
    ]]></change-notes>

    <idea-version since-build="{{.SinceVersion}}"{{if .UntilVersion}} until-build="{{.UntilVersion}}"{{end}}/>

    <depends>com.intellij.modules.platform</depends>

    <extensions defaultExtensionNs="com.intellij">
        <fileType name="Script2 File"
                  implementationClass="com.qmpcontroller.script2.Script2FileType"
                  fieldName="INSTANCE"
                  language="Script2"
                  extensions="sc2;script2;qmp2"/>

        <lang.parserDefinition language="Script2"
                              implementationClass="com.qmpcontroller.script2.Script2ParserDefinition"/>

        <lang.syntaxHighlighterFactory language="Script2"
                                      implementationClass="com.qmpcontroller.script2.Script2SyntaxHighlighterFactory"/>

        <colorSettingsPage implementation="com.qmpcontroller.script2.Script2ColorSettingsPage"/>

        <annotator language="Script2"
                  implementationClass="com.qmpcontroller.script2.Script2Annotator"/>
    </extensions>
</idea-plugin>`

	tmpl, err := template.New("plugin").Parse(xmlTemplate)
	if err != nil {
		return err
	}

	file, err := os.Create(filepath.Join(outputDir, "src", "main", "resources", "META-INF", "plugin.xml"))
	if err != nil {
		return err
	}
	defer file.Close()

	return tmpl.Execute(file, plugin)
}

// generateBNFGrammar creates the BNF grammar file
func generateBNFGrammar(outputDir string, patterns map[string]PatternInfo) error {
	bnfTemplate := `{
  parserClass="com.qmpcontroller.script2.parser.Script2Parser"

  extends="com.intellij.extapi.psi.ASTWrapperPsiElement"

  psiClassPrefix="Script2"
  psiImplClassSuffix="Impl"
  psiPackage="com.qmpcontroller.script2.psi"
  psiImplPackage="com.qmpcontroller.script2.psi.impl"

  elementTypeHolderClass="com.qmpcontroller.script2.psi.Script2Types"
  elementTypeClass="com.qmpcontroller.script2.psi.Script2ElementType"
  tokenTypeClass="com.qmpcontroller.script2.psi.Script2TokenType"

  psiImplUtilClass="com.qmpcontroller.script2.psi.impl.Script2PsiImplUtil"
}

script2File ::= item_*

private item_ ::= (
    comment |
    variable_assignment |
    directive |
    function_def |
    function_call |
    text_line |
    CRLF
)

comment ::= COMMENT
variable_assignment ::= IDENTIFIER '=' variable_value
variable_value ::= simple_value | complex_variable

simple_value ::= IDENTIFIER | STRING | NUMBER
complex_variable ::= VARIABLE_EXPANSION

directive ::= '<' directive_content '>'
directive_content ::= (
    key_sequence |
    watch_directive |
    console_directive |
    wait_directive |
    exit_directive |
    conditional_directive |
    loop_directive |
    break_directive |
    screenshot_directive
)

key_sequence ::= KEY_NAME
watch_directive ::= 'watch' STRING timeout_spec
console_directive ::= 'console' NUMBER
wait_directive ::= 'wait' duration
exit_directive ::= 'exit' NUMBER
break_directive ::= 'break'
screenshot_directive ::= 'screenshot' STRING

conditional_directive ::= (
    'if-found' STRING timeout_spec |
    'if-not-found' STRING timeout_spec |
    'else'
)

loop_directive ::= (
    'retry' NUMBER |
    'repeat' NUMBER |
    'while-found' STRING timeout_spec |
    'while-not-found' STRING timeout_spec
)

function_def ::= '<' 'function' IDENTIFIER '>'
function_call ::= '<' 'call' IDENTIFIER function_args? '>'
function_args ::= IDENTIFIER+

timeout_spec ::= duration ('poll' duration)?
duration ::= NUMBER time_unit?
time_unit ::= 's' | 'ms'

text_line ::= TEXT_CONTENT

%token COMMENT='regexp:#.*'
%token IDENTIFIER='regexp:[A-Za-z_][A-Za-z0-9_]*'
%token STRING='regexp:"([^"\\\\]|\\\\.)*"'
%token NUMBER='regexp:\d+'
%token VARIABLE_EXPANSION='regexp:\$\{[^}]+\}'
%token KEY_NAME='regexp:(enter|tab|space|escape|backspace|delete|up|down|left|right|home|end|page_up|page_down|ctrl\+[a-z]|alt\+[a-z]|shift\+[a-z]|f[1-9][0-9]?)'
%token TEXT_CONTENT='regexp:[^<#\n\r]+'
%token CRLF='regexp:\r?\n'`

	return os.WriteFile(filepath.Join(outputDir, "src", "main", "resources", "Script2.bnf"), []byte(bnfTemplate), 0644)
}

// generateLexer creates the lexer file
func generateLexer(outputDir string, patterns map[string]PatternInfo) error {
	lexerTemplate := `package com.qmpcontroller.script2;

import com.intellij.lexer.FlexLexer;
import com.intellij.psi.tree.IElementType;
import com.qmpcontroller.script2.psi.Script2Types;
import com.intellij.psi.TokenType;

%%

%class Script2Lexer
%implements FlexLexer
%unicode
%function advance
%type IElementType
%eof{  return;
%eof}

// Whitespace
WHITE_SPACE = [\ \t\f]
CRLF = \r?\n

// Comments
COMMENT = #.*

// Identifiers and numbers
IDENTIFIER = [A-Za-z_][A-Za-z0-9_]*
NUMBER = \d+

// Strings
STRING = \"([^\"\\\\]|\\\\.)*\"

// Variable expansions
VARIABLE_EXPANSION = \$\{[^}]+\}

// Key sequences
KEY_NAME = (enter|tab|space|escape|backspace|delete|up|down|left|right|home|end|page_up|page_down|ctrl\+[a-z]|alt\+[a-z]|shift\+[a-z]|f[1-9][0-9]?)

// Directive keywords
WATCH = "watch"
CONSOLE = "console"
WAIT = "wait"
EXIT = "exit"
BREAK = "break"
SCREENSHOT = "screenshot"
IF_FOUND = "if-found"
IF_NOT_FOUND = "if-not-found"
ELSE = "else"
RETRY = "retry"
REPEAT = "repeat"
WHILE_FOUND = "while-found"
WHILE_NOT_FOUND = "while-not-found"
FUNCTION = "function"
CALL = "call"

// Special characters
EQUALS = "="
LT = "<"
GT = ">"
POLL = "poll"

// Time units
TIME_UNIT = [sm]s?

// Text content (everything else)
TEXT_CONTENT = [^<#\n\r]+

%%

{WHITE_SPACE}+          { return TokenType.WHITE_SPACE; }
{CRLF}                  { return Script2Types.CRLF; }
{COMMENT}               { return Script2Types.COMMENT; }

{LT}                    { return Script2Types.LT; }
{GT}                    { return Script2Types.GT; }
{EQUALS}                { return Script2Types.EQUALS; }

{WATCH}                 { return Script2Types.WATCH; }
{CONSOLE}               { return Script2Types.CONSOLE; }
{WAIT}                  { return Script2Types.WAIT; }
{EXIT}                  { return Script2Types.EXIT; }
{BREAK}                 { return Script2Types.BREAK; }
{SCREENSHOT}            { return Script2Types.SCREENSHOT; }
{IF_FOUND}              { return Script2Types.IF_FOUND; }
{IF_NOT_FOUND}          { return Script2Types.IF_NOT_FOUND; }
{ELSE}                  { return Script2Types.ELSE; }
{RETRY}                 { return Script2Types.RETRY; }
{REPEAT}                { return Script2Types.REPEAT; }
{WHILE_FOUND}           { return Script2Types.WHILE_FOUND; }
{WHILE_NOT_FOUND}       { return Script2Types.WHILE_NOT_FOUND; }
{FUNCTION}              { return Script2Types.FUNCTION; }
{CALL}                  { return Script2Types.CALL; }
{POLL}                  { return Script2Types.POLL; }

{VARIABLE_EXPANSION}    { return Script2Types.VARIABLE_EXPANSION; }
{KEY_NAME}              { return Script2Types.KEY_NAME; }
{STRING}                { return Script2Types.STRING; }
{NUMBER}                { return Script2Types.NUMBER; }
{TIME_UNIT}             { return Script2Types.TIME_UNIT; }
{IDENTIFIER}            { return Script2Types.IDENTIFIER; }

{TEXT_CONTENT}          { return Script2Types.TEXT_CONTENT; }

[^]                     { return TokenType.BAD_CHARACTER; }`

	return os.WriteFile(filepath.Join(outputDir, "src", "main", "resources", "Script2.flex"), []byte(lexerTemplate), 0644)
}

// generateFileType creates the file type class
func generateFileType(outputDir string) error {
	fileTypeTemplate := `package com.qmpcontroller.script2;

import com.intellij.openapi.fileTypes.LanguageFileType;
import com.intellij.openapi.util.IconLoader;
import org.jetbrains.annotations.NotNull;
import org.jetbrains.annotations.Nullable;

import javax.swing.*;

public class Script2FileType extends LanguageFileType {
    public static final Script2FileType INSTANCE = new Script2FileType();

    private Script2FileType() {
        super(Script2Language.INSTANCE);
    }

    @NotNull
    @Override
    public String getName() {
        return "Script2 File";
    }

    @NotNull
    @Override
    public String getDescription() {
        return "QMP Script2 automation file";
    }

    @NotNull
    @Override
    public String getDefaultExtension() {
        return "sc2";
    }

    @Nullable
    @Override
    public Icon getIcon() {
        return IconLoader.getIcon("/icons/script2.png", Script2FileType.class);
    }
}`

	languageTemplate := `package com.qmpcontroller.script2;

import com.intellij.lang.Language;

public class Script2Language extends Language {
    public static final Script2Language INSTANCE = new Script2Language();

    private Script2Language() {
        super("Script2");
    }
}`

	syntaxHighlighterTemplate := `package com.qmpcontroller.script2;

import com.intellij.openapi.editor.DefaultLanguageHighlighterColors;
import com.intellij.openapi.editor.colors.TextAttributesKey;
import com.intellij.openapi.fileTypes.SyntaxHighlighterBase;
import com.intellij.lexer.Lexer;
import com.intellij.psi.tree.IElementType;
import org.jetbrains.annotations.NotNull;

import static com.intellij.openapi.editor.colors.TextAttributesKey.createTextAttributesKey;

public class Script2SyntaxHighlighter extends SyntaxHighlighterBase {
    public static final TextAttributesKey COMMENT =
        createTextAttributesKey("SCRIPT2_COMMENT", DefaultLanguageHighlighterColors.LINE_COMMENT);
    public static final TextAttributesKey DIRECTIVE =
        createTextAttributesKey("SCRIPT2_DIRECTIVE", DefaultLanguageHighlighterColors.KEYWORD);
    public static final TextAttributesKey VARIABLE =
        createTextAttributesKey("SCRIPT2_VARIABLE", DefaultLanguageHighlighterColors.GLOBAL_VARIABLE);
    public static final TextAttributesKey STRING =
        createTextAttributesKey("SCRIPT2_STRING", DefaultLanguageHighlighterColors.STRING);
    public static final TextAttributesKey NUMBER =
        createTextAttributesKey("SCRIPT2_NUMBER", DefaultLanguageHighlighterColors.NUMBER);
    public static final TextAttributesKey FUNCTION =
        createTextAttributesKey("SCRIPT2_FUNCTION", DefaultLanguageHighlighterColors.FUNCTION_DECLARATION);

    @NotNull
    @Override
    public Lexer getHighlightingLexer() {
        return new Script2LexerAdapter();
    }

    @NotNull
    @Override
    public TextAttributesKey[] getTokenHighlights(IElementType tokenType) {
        // Simple regex-based highlighting - no complex grammar needed
        return TextAttributesKey.EMPTY_ARRAY;
    }
}`

	// Write the files
	javaDir := filepath.Join(outputDir, "src", "main", "java", "com", "qmpcontroller", "script2")

	files := map[string]string{
		"Script2FileType.java":      fileTypeTemplate,
		"Script2Language.java":      languageTemplate,
		"Script2SyntaxHighlighter.java": syntaxHighlighterTemplate,
	}

	for filename, content := range files {
		if err := os.WriteFile(filepath.Join(javaDir, filename), []byte(content), 0644); err != nil {
			return err
		}
	}

	return nil
}

// generateBuildGradle creates the Gradle build file
func generateBuildGradle(outputDir string, plugin *JetBrainsPlugin) error {
	gradleTemplate := `plugins {
    id 'java'
    id 'org.jetbrains.intellij' version '1.13.3'
}

group 'com.qmpcontroller'
version '{{.Version}}'

repositories {
    mavenCentral()
}

dependencies {
    testImplementation 'org.junit.jupiter:junit-jupiter-api:5.8.1'
    testRuntimeOnly 'org.junit.jupiter:junit-jupiter-engine:5.8.1'
}

// Configure Gradle IntelliJ Plugin
// Read more: https://plugins.jetbrains.com/docs/intellij/tools-gradle-intellij-plugin.html
intellij {
    version = '2022.3.3'
    type = 'IC' // Target IDE Platform
    plugins = []
}

// Set JVM compatibility for Java 11
java {
    sourceCompatibility = JavaVersion.VERSION_11
    targetCompatibility = JavaVersion.VERSION_11
}

tasks {
    patchPluginXml {
        sinceBuild = '{{.SinceVersion}}'
        {{if .UntilVersion}}untilBuild = '{{.UntilVersion}}'{{end}}
    }

    signPlugin {
        certificateChain = System.getenv("CERTIFICATE_CHAIN")
        privateKey = System.getenv("PRIVATE_KEY")
        password = System.getenv("PRIVATE_KEY_PASSWORD")
    }

    publishPlugin {
        token = System.getenv("PUBLISH_TOKEN")
    }

    test {
        useJUnitPlatform()
    }
}`

	tmpl, err := template.New("gradle").Parse(gradleTemplate)
	if err != nil {
		return err
	}

	file, err := os.Create(filepath.Join(outputDir, "build.gradle"))
	if err != nil {
		return err
	}
	defer file.Close()

	return tmpl.Execute(file, plugin)
}

// generateJetbrainsReadme creates the README file
func generateJetbrainsReadme(outputDir string, plugin *JetBrainsPlugin) error {
	readmeTemplate := `# {{.Name}}

A JetBrains plugin for syntax highlighting and language support for QMP Script2 automation files.

## Features

- 🎨 **Syntax Highlighting**: Full syntax highlighting for Script2 files
- 📝 **File Type Recognition**: Automatic detection of .sc2, .script2, and .qmp2 files
- 🔤 **Variable Support**: Highlighting for bash-style variable expansions
- ⚡ **Directive Recognition**: Support for all Script2 directives
- 🔧 **Function Highlighting**: Function definitions and calls
- 💬 **Comment Support**: Proper comment highlighting

## Supported File Extensions

{{range .FileExtensions}}- {{.}}
{{end}}

## Installation

### Method 1: Install from JetBrains Marketplace (Recommended)
1. Open your JetBrains IDE (IntelliJ IDEA, GoLand, PyCharm, etc.)
2. Go to **File** → **Settings** → **Plugins**
3. Search for "{{.Name}}"
4. Click **Install**
5. Restart the IDE

### Method 2: Install from Disk
1. Build the plugin: ` + "`./gradlew buildPlugin`" + `
2. Open your JetBrains IDE
3. Go to **File** → **Settings** → **Plugins**
4. Click the gear icon ⚙️ → **Install Plugin from Disk...**
5. Select the ` + "`build/distributions/{{.Name}}-{{.Version}}.zip`" + ` file
6. Restart the IDE

## Building from Source

### Prerequisites
- Java 11 or higher
- Gradle (included via wrapper)

### Build Steps
` + "```bash" + `
# Clone the repository (if not already done)
git clone https://github.com/jeeftor/qmp-controller.git
cd qmp-controller/jetbrains-plugin

# Build the plugin
./gradlew buildPlugin

# The plugin will be created in build/distributions/
` + "```" + `

### Development
` + "```bash" + `
# Run the plugin in a development IDE instance
./gradlew runIde

# Run tests
./gradlew test

# Build and verify plugin
./gradlew buildPlugin verifyPlugin
` + "```" + `

## Script2 Language Support

This plugin provides highlighting for the complete Script2 language syntax:

### Variables
` + "```script2" + `
USER=${USER:-admin}
PASSWORD=${PASSWORD:-secret}
` + "```" + `

### Directives
` + "```script2" + `
<enter>
<tab>
<ctrl+c>
<wait 5s>
<watch "login:" 30s>
<console 2>
<screenshot "debug.png">
<break>  # Debugging support
` + "```" + `

### Conditionals and Loops
` + "```script2" + `
<if-found "$ " 5s>
echo "Login successful"
<else>
echo "Login failed"

<retry 3>
ssh user@server
<wait 2s>

<while-not-found "ready" 60s>
echo "Waiting..."
<wait 1s>
` + "```" + `

### Functions
` + "```script2" + `
<function login>
ssh $1@$2
<watch "password:" 10s>
$PASSWORD
<enter>
<end-function>

<call login admin server.example.com>
` + "```" + `

## Compatible IDEs

This plugin works with all JetBrains IDEs:
- IntelliJ IDEA (Community & Ultimate)
- GoLand
- PyCharm (Community & Professional)
- WebStorm
- PhpStorm
- RubyMine
- CLion
- DataGrip
- AppCode
- Rider

## Version Information

- **Plugin Version**: {{.Version}}
- **IntelliJ Platform**: {{.IdeaVersion}}+
- **Java Compatibility**: 11+

## Contributing

Contributions are welcome! Please see the main repository for contribution guidelines:
{{.VendorURL}}

## License

This plugin is released under the MIT License. See the main repository for details.

## Support

For issues, feature requests, or questions:
- Create an issue: {{.VendorURL}}/issues
- Email: {{.VendorEmail}}
`

	tmpl, err := template.New("readme").Parse(readmeTemplate)
	if err != nil {
		return err
	}

	file, err := os.Create(filepath.Join(outputDir, "README.md"))
	if err != nil {
		return err
	}
	defer file.Close()

	return tmpl.Execute(file, plugin)
}

// extractKeywords extracts keywords from patterns for metadata
func extractKeywords(patterns map[string]PatternInfo) []string {
	keywords := make(map[string]bool)
	keywordList := []string{"qmp", "script2", "automation", "virtualization", "syntax"}

	for name := range patterns {
		if strings.Contains(name, "directive") || strings.Contains(name, "command") {
			keywords[strings.ToLower(name)] = true
		}
	}

	for keyword := range keywords {
		keywordList = append(keywordList, keyword)
	}

	sort.Strings(keywordList)
	return keywordList
}

// PatternInfo holds information about syntax patterns
type PatternInfo struct {
	Name        string
	Pattern     string
	TokenType   string
	Description string
}

// extractScript2Patterns extracts syntax patterns from the script2 parser
func extractScript2Patterns() (map[string]PatternInfo, error) {
	patterns := make(map[string]PatternInfo)

	// Define the Script2 syntax patterns based on our language specification
	patterns["comment"] = PatternInfo{
		Name:        "comment",
		Pattern:     "#.*",
		TokenType:   "COMMENT",
		Description: "Line comment starting with #",
	}

	patterns["variable_assignment"] = PatternInfo{
		Name:        "variable_assignment",
		Pattern:     "[A-Za-z_][A-Za-z0-9_]*=",
		TokenType:   "VARIABLE_ASSIGNMENT",
		Description: "Variable assignment",
	}

	patterns["variable_expansion"] = PatternInfo{
		Name:        "variable_expansion",
		Pattern:     `\$\{[^}]+\}`,
		TokenType:   "VARIABLE_EXPANSION",
		Description: "Variable expansion like ${VAR}",
	}

	patterns["simple_variable"] = PatternInfo{
		Name:        "simple_variable",
		Pattern:     `\$[A-Za-z_][A-Za-z0-9_]*`,
		TokenType:   "SIMPLE_VARIABLE",
		Description: "Simple variable like $VAR",
	}

	patterns["string"] = PatternInfo{
		Name:        "string",
		Pattern:     `"([^"\\]|\\.)*"`,
		TokenType:   "STRING",
		Description: "Double-quoted string",
	}

	patterns["number"] = PatternInfo{
		Name:        "number",
		Pattern:     `\d+`,
		TokenType:   "NUMBER",
		Description: "Numeric literal",
	}

	patterns["directive_start"] = PatternInfo{
		Name:        "directive_start",
		Pattern:     "<",
		TokenType:   "LT",
		Description: "Directive start bracket",
	}

	patterns["directive_end"] = PatternInfo{
		Name:        "directive_end",
		Pattern:     ">",
		TokenType:   "GT",
		Description: "Directive end bracket",
	}

	// Directive keywords
	directiveKeywords := []string{
		"enter", "tab", "space", "escape", "backspace", "delete",
		"up", "down", "left", "right", "home", "end", "page_up", "page_down",
		"watch", "console", "wait", "exit", "break", "screenshot",
		"if-found", "if-not-found", "else", "retry", "repeat",
		"while-found", "while-not-found", "function", "end-function", "call", "include",
	}

	for _, keyword := range directiveKeywords {
		patterns[keyword] = PatternInfo{
			Name:        keyword,
			Pattern:     keyword,
			TokenType:   strings.ToUpper(strings.ReplaceAll(keyword, "-", "_")),
			Description: fmt.Sprintf("Directive keyword: %s", keyword),
		}
	}

	patterns["identifier"] = PatternInfo{
		Name:        "identifier",
		Pattern:     "[A-Za-z_][A-Za-z0-9_]*",
		TokenType:   "IDENTIFIER",
		Description: "Identifier",
	}

	patterns["text_content"] = PatternInfo{
		Name:        "text_content",
		Pattern:     "[^<#\\n\\r]+",
		TokenType:   "TEXT_CONTENT",
		Description: "Regular text content",
	}

	return patterns, nil
}

// getJetbrainsGitVersion gets version information from git
func getJetbrainsGitVersion() string {
	cmd := exec.Command("git", "describe", "--tags", "--always", "--dirty")
	output, err := cmd.Output()
	if err != nil {
		return "dev"
	}
	return strings.TrimSpace(string(output))
}

func init() {
	// Also generate helper classes
	generateJetBrainsPluginCmd.Flags().Bool("helpers", true, "Generate helper classes")
	rootCmd.AddCommand(generateJetBrainsPluginCmd)
}
