package cmd

import (
	"path/filepath"

	"github.com/jeeftor/qmp-controller/internal/filesystem"
)

// generateJetbrainsHelpers creates additional helper files for the JetBrains plugin
func generateJetbrainsHelpers(outputDir string) error {
	javaDir := filepath.Join(outputDir, "src", "main", "java", "com", "qmpcontroller", "script2")

	// Generate all helper classes
	helperClasses := map[string]string{
		"Script2LexerAdapter.java":          generateLexerAdapter(),
		"Script2SyntaxHighlighterFactory.java": generateSyntaxHighlighterFactory(),
		"Script2ParserDefinition.java":     generateParserDefinition(),
		"Script2ColorSettingsPage.java":    generateColorSettingsPage(),
		"Script2Annotator.java":            generateAnnotator(),
		"Script2ElementType.java":          generateElementType(),
		"Script2TokenType.java":            generateTokenType(),
		"Script2SimpleParser.java":         generateSimpleParser(),
		"Script2PsiFile.java":              generatePsiFile(),
		"Script2PsiElement.java":           generatePsiElement(),
	}

	for filename, content := range helperClasses {
		filePath := filepath.Join(javaDir, filename)
		if err := filesystem.WriteFileWithDirectory(filePath, []byte(content), 0644); err != nil {
			return err
		}
	}

	// Generate Gradle wrapper files
	if err := generateGradleWrapper(outputDir); err != nil {
		return err
	}

	// Generate example script
	if err := generateExampleScript(outputDir); err != nil {
		return err
	}

	return nil
}

func generateLexerAdapter() string {
	return `package com.qmpcontroller.script2;

import com.intellij.lexer.LexerBase;
import com.intellij.psi.tree.IElementType;
import com.intellij.psi.TokenType;
import org.jetbrains.annotations.Nullable;

public class Script2LexerAdapter extends LexerBase {
    private CharSequence buffer;
    private int startOffset;
    private int endOffset;
    private int currentOffset;
    private IElementType tokenType;
    private int tokenStart;
    private int tokenEnd;

    @Override
    public void start(CharSequence buffer, int startOffset, int endOffset, int initialState) {
        this.buffer = buffer;
        this.startOffset = startOffset;
        this.endOffset = endOffset;
        this.currentOffset = startOffset;
        advance();
    }

    @Override
    public int getState() {
        return 0;
    }

    @Nullable
    @Override
    public IElementType getTokenType() {
        return tokenType;
    }

    @Override
    public int getTokenStart() {
        return tokenStart;
    }

    @Override
    public int getTokenEnd() {
        return tokenEnd;
    }

    @Override
    public void advance() {
        if (currentOffset >= endOffset) {
            tokenType = null;
            return;
        }

        tokenStart = currentOffset;

        // Simple token recognition - just treat everything as text for now
        while (currentOffset < endOffset && buffer.charAt(currentOffset) != '\n') {
            currentOffset++;
        }

        if (currentOffset < endOffset && buffer.charAt(currentOffset) == '\n') {
            currentOffset++;
        }

        tokenEnd = currentOffset;
        tokenType = TokenType.WHITE_SPACE;
    }

    @Override
    public CharSequence getBufferSequence() {
        return buffer;
    }

    @Override
    public int getBufferEnd() {
        return endOffset;
    }
}`
}

func generateSyntaxHighlighterFactory() string {
	return `package com.qmpcontroller.script2;

import com.intellij.openapi.fileTypes.SyntaxHighlighter;
import com.intellij.openapi.fileTypes.SyntaxHighlighterFactory;
import com.intellij.openapi.project.Project;
import com.intellij.openapi.vfs.VirtualFile;
import org.jetbrains.annotations.NotNull;

public class Script2SyntaxHighlighterFactory extends SyntaxHighlighterFactory {
    @NotNull
    @Override
    public SyntaxHighlighter getSyntaxHighlighter(Project project, VirtualFile virtualFile) {
        return new Script2SyntaxHighlighter();
    }
}`
}

func generateParserDefinition() string {
	return `package com.qmpcontroller.script2;

import com.intellij.lang.ASTNode;
import com.intellij.lang.ParserDefinition;
import com.intellij.lang.PsiParser;
import com.intellij.lexer.Lexer;
import com.intellij.openapi.project.Project;
import com.intellij.psi.FileViewProvider;
import com.intellij.psi.PsiElement;
import com.intellij.psi.PsiFile;
import com.intellij.psi.TokenType;
import com.intellij.psi.tree.IFileElementType;
import com.intellij.psi.tree.TokenSet;
import org.jetbrains.annotations.NotNull;

public class Script2ParserDefinition implements ParserDefinition {
    public static final TokenSet WHITE_SPACES = TokenSet.create(TokenType.WHITE_SPACE);
    public static final TokenSet COMMENTS = TokenSet.create(Script2TokenType.COMMENT);
    public static final TokenSet STRINGS = TokenSet.create(Script2TokenType.STRING);

    public static final IFileElementType FILE = new IFileElementType(Script2Language.INSTANCE);

    @NotNull
    @Override
    public Lexer createLexer(Project project) {
        return new Script2LexerAdapter();
    }

    @NotNull
    @Override
    public TokenSet getWhitespaceTokens() {
        return WHITE_SPACES;
    }

    @NotNull
    @Override
    public TokenSet getCommentTokens() {
        return COMMENTS;
    }

    @NotNull
    @Override
    public TokenSet getStringLiteralElements() {
        return STRINGS;
    }

    @NotNull
    @Override
    public PsiParser createParser(final Project project) {
        return new Script2SimpleParser();
    }

    @Override
    public IFileElementType getFileNodeType() {
        return FILE;
    }

    @Override
    public PsiFile createFile(FileViewProvider viewProvider) {
        return new Script2PsiFile(viewProvider);
    }

    @Override
    public SpaceRequirements spaceExistenceTypeBetweenTokens(ASTNode left, ASTNode right) {
        return SpaceRequirements.MAY;
    }

    @NotNull
    @Override
    public PsiElement createElement(ASTNode node) {
        return new Script2PsiElement(node);
    }
}`
}

func generateColorSettingsPage() string {
	return `package com.qmpcontroller.script2;

import com.intellij.openapi.editor.colors.TextAttributesKey;
import com.intellij.openapi.fileTypes.SyntaxHighlighter;
import com.intellij.openapi.options.colors.AttributesDescriptor;
import com.intellij.openapi.options.colors.ColorDescriptor;
import com.intellij.openapi.options.colors.ColorSettingsPage;
import org.jetbrains.annotations.NotNull;
import org.jetbrains.annotations.Nullable;

import javax.swing.*;
import java.util.Map;

public class Script2ColorSettingsPage implements ColorSettingsPage {
    private static final AttributesDescriptor[] DESCRIPTORS = new AttributesDescriptor[]{
        new AttributesDescriptor("Comment", Script2SyntaxHighlighter.COMMENT),
        new AttributesDescriptor("Directive", Script2SyntaxHighlighter.DIRECTIVE),
        new AttributesDescriptor("Variable", Script2SyntaxHighlighter.VARIABLE),
        new AttributesDescriptor("String", Script2SyntaxHighlighter.STRING),
        new AttributesDescriptor("Number", Script2SyntaxHighlighter.NUMBER),
        new AttributesDescriptor("Function", Script2SyntaxHighlighter.FUNCTION),
    };

    @Nullable
    @Override
    public Icon getIcon() {
        return Script2FileType.INSTANCE.getIcon();
    }

    @NotNull
    @Override
    public SyntaxHighlighter getHighlighter() {
        return new Script2SyntaxHighlighter();
    }

    @NotNull
    @Override
    public String getDemoText() {
        return "# QMP Script2 Example\n" +
               "# Variable assignments\n" +
               "USER=${USER:-admin}\n" +
               "PASSWORD=${PASSWORD:-secret}\n" +
               "\n" +
               "# Text to type\n" +
               "ssh $USER@server\n" +
               "\n" +
               "# Directives\n" +
               "<watch \"password:\" 10s>\n" +
               "$PASSWORD\n" +
               "<enter>\n" +
               "<wait 2s>\n" +
               "\n" +
               "# Conditional execution\n" +
               "<if-found \"$ \" 5s>\n" +
               "echo \"Login successful\"\n" +
               "<else>\n" +
               "echo \"Login failed\"\n" +
               "\n" +
               "# Functions\n" +
               "<function deploy>\n" +
               "echo \"Deploying to $1\"\n" +
               "<screenshot \"deploy-{timestamp}.png\">\n" +
               "<end-function>\n" +
               "\n" +
               "<call deploy production>\n" +
               "\n" +
               "# Debugging\n" +
               "<break>\n" +
               "<console 2>";
    }

    @Nullable
    @Override
    public Map<String, TextAttributesKey> getAdditionalHighlightingTagToDescriptorMap() {
        return null;
    }

    @NotNull
    @Override
    public AttributesDescriptor[] getAttributeDescriptors() {
        return DESCRIPTORS;
    }

    @NotNull
    @Override
    public ColorDescriptor[] getColorDescriptors() {
        return ColorDescriptor.EMPTY_ARRAY;
    }

    @NotNull
    @Override
    public String getDisplayName() {
        return "Script2";
    }
}`
}

func generateAnnotator() string {
	return `package com.qmpcontroller.script2;

import com.intellij.lang.annotation.AnnotationHolder;
import com.intellij.lang.annotation.Annotator;
import com.intellij.lang.annotation.HighlightSeverity;
import com.intellij.psi.PsiElement;
import org.jetbrains.annotations.NotNull;

public class Script2Annotator implements Annotator {
    @Override
    public void annotate(@NotNull final PsiElement element, @NotNull AnnotationHolder holder) {
        // Add custom annotations here
        // For example, validate directive syntax, check variable references, etc.

        // Example: Highlight unknown directives
        String text = element.getText();
        if (text.startsWith("<") && text.endsWith(">") && !isKnownDirective(text)) {
            holder.newAnnotation(HighlightSeverity.WARNING, "Unknown directive: " + text)
                    .range(element)
                    .create();
        }
    }

    private boolean isKnownDirective(String text) {
        // List of known directives
        return text.matches("<(enter|tab|space|escape|backspace|delete|up|down|left|right|home|end|page_up|page_down|" +
                           "ctrl\\+[a-z]|alt\\+[a-z]|shift\\+[a-z]|f[1-9][0-9]?|" +
                           "watch|console|wait|exit|break|screenshot|" +
                           "if-found|if-not-found|else|retry|repeat|while-found|while-not-found|" +
                           "function|end-function|call|include).*>");
    }
}`
}

func generateElementType() string {
	return `package com.qmpcontroller.script2;

import com.intellij.psi.tree.IElementType;
import org.jetbrains.annotations.NonNls;
import org.jetbrains.annotations.NotNull;

public class Script2ElementType extends IElementType {
    public Script2ElementType(@NotNull @NonNls String debugName) {
        super(debugName, Script2Language.INSTANCE);
    }
}`
}

func generateTokenType() string {
	return `package com.qmpcontroller.script2;

import com.intellij.psi.tree.IElementType;
import org.jetbrains.annotations.NonNls;
import org.jetbrains.annotations.NotNull;

public class Script2TokenType extends IElementType {
    public static final Script2TokenType COMMENT = new Script2TokenType("COMMENT");
    public static final Script2TokenType STRING = new Script2TokenType("STRING");
    public static final Script2TokenType DIRECTIVE = new Script2TokenType("DIRECTIVE");
    public static final Script2TokenType VARIABLE = new Script2TokenType("VARIABLE");
    public static final Script2TokenType FUNCTION = new Script2TokenType("FUNCTION");
    public static final Script2TokenType NUMBER = new Script2TokenType("NUMBER");

    public Script2TokenType(@NotNull @NonNls String debugName) {
        super(debugName, Script2Language.INSTANCE);
    }

    @Override
    public String toString() {
        return "Script2TokenType." + super.toString();
    }
}`
}

func generateGradleWrapper(outputDir string) error {
	// Create gradle wrapper properties
	wrapperProps := `distributionBase=GRADLE_USER_HOME
distributionPath=wrapper/dists
distributionUrl=https\://services.gradle.org/distributions/gradle-7.6-bin.zip
zipStoreBase=GRADLE_USER_HOME
zipStorePath=wrapper/dists
`

	gradlewScript := `#!/usr/bin/env sh

##############################################################################
##
##  Gradle start up script for UN*X
##
##############################################################################

# Attempt to set APP_HOME
# Resolve links: $0 may be a link
PRG="$0"
# Need this for relative symlinks.
while [ -h "$PRG" ] ; do
    ls=` + "`ls -ld \"$PRG\"`" + `
    link=` + "`expr \"$ls\" : '.*-> \\(.*\\)$'`" + `
    if expr "$link" : '/.*' > /dev/null; then
        PRG="$link"
    else
        PRG=` + "`dirname \"$PRG\"`" + `"/$link"
    fi
done
SAVED="` + "`pwd`" + `"
cd "` + "`dirname \"$PRG\"`" + `/" >/dev/null
APP_HOME="` + "`pwd -P`" + `"
cd "$SAVED" >/dev/null

APP_NAME="Gradle"
APP_BASE_NAME=` + "`basename \"$0\"`" + `

# Add default JVM options here. You can also use JAVA_OPTS and GRADLE_OPTS to pass JVM options to this script.
DEFAULT_JVM_OPTS='"-Xmx64m" "-Xms64m"'

# Use the maximum available, or set MAX_FD != -1 to use that value.
MAX_FD="maximum"

warn () {
    echo "$*"
}

die () {
    echo
    echo "$*"
    echo
    exit 1
}

# OS specific support (must be 'true' or 'false').
cygwin=false
msys=false
darwin=false
nonstop=false
case "` + "`uname`" + `" in
  CYGWIN* )
    cygwin=true
    ;;
  Darwin* )
    darwin=true
    ;;
  MINGW* )
    msys=true
    ;;
  NONSTOP* )
    nonstop=true
    ;;
esac

CLASSPATH=$APP_HOME/gradle/wrapper/gradle-wrapper.jar

# Determine the Java command to use to start the JVM.
if [ -n "$JAVA_HOME" ] ; then
    if [ -x "$JAVA_HOME/jre/sh/java" ] ; then
        # IBM's JDK on AIX uses strange locations for the executables
        JAVACMD="$JAVA_HOME/jre/sh/java"
    else
        JAVACMD="$JAVA_HOME/bin/java"
    fi
    if [ ! -x "$JAVACMD" ] ; then
        die "ERROR: JAVA_HOME is set to an invalid directory: $JAVA_HOME

Please set the JAVA_HOME variable in your environment to match the
location of your Java installation."
    fi
else
    JAVACMD="java"
    which java >/dev/null 2>&1 || die "ERROR: JAVA_HOME is not set and no 'java' command could be found in your PATH.

Please set the JAVA_HOME variable in your environment to match the
location of your Java installation."
fi

# Increase the maximum file descriptors if we can.
if [ "$cygwin" = "false" -a "$darwin" = "false" -a "$nonstop" = "false" ] ; then
    MAX_FD_LIMIT=` + "`ulimit -H -n`" + `
    if [ $? -eq 0 ] ; then
        if [ "$MAX_FD" = "maximum" -o "$MAX_FD" = "max" ] ; then
            MAX_FD="$MAX_FD_LIMIT"
        fi
        ulimit -n $MAX_FD
        if [ $? -ne 0 ] ; then
            warn "Could not set maximum file descriptor limit: $MAX_FD"
        fi
    else
        warn "Could not query maximum file descriptor limit: $MAX_FD_LIMIT"
    fi
fi

# For Darwin, add options to specify how the application appears in the dock
if [ "$darwin" = "true" ]; then
    GRADLE_OPTS="$GRADLE_OPTS \"-Xdock:name=$APP_NAME\" \"-Xdock:icon=$APP_HOME/media/gradle.icns\""
fi

# For Cygwin or MSYS, switch paths to Windows format before running java
if [ "$cygwin" = "true" -o "$msys" = "true" ] ; then
    APP_HOME=` + "`cygpath --path --mixed \"$APP_HOME\"`" + `
    CLASSPATH=` + "`cygpath --path --mixed \"$CLASSPATH\"`" + `

    JAVACMD=` + "`cygpath --unix \"$JAVACMD\"`" + `

    # We build the pattern for arguments to be converted via cygpath
    ROOTDIRSRAW=` + "`find -L / -maxdepth 1 -mindepth 1 -type d 2>/dev/null`" + `
    SEP=""
    for dir in $ROOTDIRSRAW ; do
        ROOTDIRS="$ROOTDIRS$SEP$dir"
        SEP="|"
    done
    OURCYGPATTERN="(^($ROOTDIRS))"
    # Add a user-defined pattern to the cygpath arguments
    if [ "$GRADLE_CYGPATTERN" != "" ] ; then
        OURCYGPATTERN="$OURCYGPATTERN|($GRADLE_CYGPATTERN)"
    fi
    # Now convert the arguments - kludge to limit ourselves to /bin/sh
    i=0
    for arg in "$@" ; do
        CHECK=` + "`echo \"$arg\"|egrep -c \"$OURCYGPATTERN\" -`" + `
        CHECK2=` + "`echo \"$arg\"|egrep -c \"^-\"`" + `                                 ### Determine if an option

        if [ $CHECK -ne 0 ] && [ $CHECK2 -eq 0 ] ; then                    ### Added a condition
            eval ` + "`echo args$i`" + `=` + "`cygpath --path --ignore --mixed \"$arg\"`" + `
        else
            eval ` + "`echo args$i`" + `="\"$arg\""
        fi
        i=` + "`expr $i + 1`" + `
    done
    case $i in
        0) set -- ;;
        1) set -- "$args0" ;;
        2) set -- "$args0" "$args1" ;;
        3) set -- "$args0" "$args1" "$args2" ;;
        4) set -- "$args0" "$args1" "$args2" "$args3" ;;
        5) set -- "$args0" "$args1" "$args2" "$args3" "$args4" ;;
        6) set -- "$args0" "$args1" "$args2" "$args3" "$args4" "$args5" ;;
        7) set -- "$args0" "$args1" "$args2" "$args3" "$args4" "$args5" "$args6" ;;
        8) set -- "$args0" "$args1" "$args2" "$args3" "$args4" "$args5" "$args6" "$args7" ;;
        9) set -- "$args0" "$args1" "$args2" "$args3" "$args4" "$args5" "$args6" "$args7" "$args8" ;;
    esac
fi

# Escape application args
save () {
    for i do printf %s\\n "$i" | sed "s/'/'\\\\''/g;1s/^/'/;$s/$/' \\\\/" ; done
    echo " "
}
APP_ARGS=` + "`save \"$@\"`" + `

# Collect all arguments for the java command, following the shell quoting and substitution rules
eval set -- $DEFAULT_JVM_OPTS $JAVA_OPTS $GRADLE_OPTS \"-Dorg.gradle.appname=$APP_BASE_NAME\" -classpath \"$CLASSPATH\" org.gradle.wrapper.GradleWrapperMain "$APP_ARGS"

exec "$JAVACMD" "$@"`

	// Write files
	wrapperPropsPath := filepath.Join(outputDir, "gradle", "wrapper", "gradle-wrapper.properties")
	if err := filesystem.WriteFileWithDirectory(wrapperPropsPath, []byte(wrapperProps), 0644); err != nil {
		return err
	}

	gradlewPath := filepath.Join(outputDir, "gradlew")
	if err := filesystem.WriteFileWithDirectory(gradlewPath, []byte(gradlewScript), 0755); err != nil {
		return err
	}

	return nil
}

func generateExampleScript(outputDir string) error {
	exampleScript := `# QMP Script2 Example
# This example demonstrates Script2 syntax highlighting

# Variables
USER=${USER:-admin}
PASSWORD=${PASSWORD:-secret}
TARGET_HOST=${TARGET_HOST:-server.example.com}

# Connect to server
ssh $USER@$TARGET_HOST

# Wait for password prompt and login
<watch "password:" 10s>
$PASSWORD
<enter>

# Wait for shell prompt
<watch "$ " 5s>

# Conditional execution based on screen content
<if-found "$ " 5s>
echo "Login successful"
<screenshot "login-success-{timestamp}.png">
<else>
echo "Login failed"
<exit 1>

# System commands with timing
whoami
<wait 1s>
pwd
<wait 1s>

# Function definition
<function check_disk_space>
df -h $1
echo "Disk space check for $1 completed"
<end-function>

# Function call
<call check_disk_space /var/log>

# Loop constructs
<retry 3>
ping -c 1 google.com
<wait 2s>

# Switch to different console
<console 2>
top
<wait 5s>

# Debugging breakpoint
<break>

# More complex conditionals
<while-not-found "completed" 60s poll 2s>
echo "Waiting for process to complete..."
<wait 2s>

# Include other scripts
<include "common-functions.sc2">

# Final screenshot and exit
<screenshot "final-state-{datetime}.png">
echo "Script completed successfully"
<exit 0>`

	examplePath := filepath.Join(outputDir, "example-script.sc2")
	return filesystem.WriteFileWithDirectory(examplePath, []byte(exampleScript), 0644)
}

func generateSimpleParser() string {
	return `package com.qmpcontroller.script2;

import com.intellij.lang.ASTNode;
import com.intellij.lang.PsiBuilder;
import com.intellij.lang.PsiParser;
import com.intellij.psi.tree.IElementType;
import org.jetbrains.annotations.NotNull;

public class Script2SimpleParser implements PsiParser {
    @NotNull
    @Override
    public ASTNode parse(@NotNull IElementType root, @NotNull PsiBuilder builder) {
        final PsiBuilder.Marker rootMarker = builder.mark();

        // Simple parser that just creates basic structure
        while (!builder.eof()) {
            builder.advanceLexer();
        }

        rootMarker.done(root);
        return builder.getTreeBuilt();
    }
}`
}

func generatePsiFile() string {
	return `package com.qmpcontroller.script2;

import com.intellij.extapi.psi.PsiFileBase;
import com.intellij.openapi.fileTypes.FileType;
import com.intellij.psi.FileViewProvider;
import org.jetbrains.annotations.NotNull;

public class Script2PsiFile extends PsiFileBase {
    public Script2PsiFile(@NotNull FileViewProvider viewProvider) {
        super(viewProvider, Script2Language.INSTANCE);
    }

    @NotNull
    @Override
    public FileType getFileType() {
        return Script2FileType.INSTANCE;
    }
}`
}

func generatePsiElement() string {
	return `package com.qmpcontroller.script2;

import com.intellij.extapi.psi.ASTWrapperPsiElement;
import com.intellij.lang.ASTNode;
import org.jetbrains.annotations.NotNull;

public class Script2PsiElement extends ASTWrapperPsiElement {
    public Script2PsiElement(@NotNull ASTNode node) {
        super(node);
    }
}`
}
