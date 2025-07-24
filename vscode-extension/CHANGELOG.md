# Change Log

## [1.0.0] - Initial Release

### Added
- Complete syntax highlighting for QMP Script2 language
- Automatic grammar generation from parser source code
- Support for all Script2 features:
  - Variables and expansion syntax
  - Directive syntax with proper highlighting
  - Function definitions and calls
  - Control flow (conditionals, loops)
  - Comments and escape sequences
  - Script composition and debugging features

### Features
- Color-coded syntax highlighting
- Automatic file type detection for .script2 and .qmp2 files
- Grammar synchronized with actual parser implementation
- Comprehensive language support

### Technical
- TextMate grammar generated from Go parser regex patterns
- Automated generation process for future updates
- Clean separation of language features in grammar
