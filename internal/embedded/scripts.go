package embedded

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

//go:embed scripts/*
var ScriptsFS embed.FS

// ExtractScripts extracts all embedded scripts to the specified directory
func ExtractScripts(targetDir string) error {
	// Create target directory if it doesn't exist
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create target directory: %v", err)
	}

	// Walk through all embedded files
	return fs.WalkDir(ScriptsFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip the root directory
		if path == "." {
			return nil
		}

		// Calculate target path, stripping "scripts/" prefix
		relativePath := strings.TrimPrefix(path, "scripts/")

		// Skip if this is just the "scripts" directory itself
		if relativePath == "scripts" {
			return nil
		}

		targetPath := filepath.Join(targetDir, relativePath)

		if d.IsDir() {
			// Create directory
			return os.MkdirAll(targetPath, 0755)
		}

		// Read embedded file
		content, err := ScriptsFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read embedded file %s: %v", path, err)
		}

		// Write file to target
		if err := os.WriteFile(targetPath, content, 0755); err != nil {
			return fmt.Errorf("failed to write file %s: %v", targetPath, err)
		}

		fmt.Printf("Extracted: %s\n", targetPath)
		return nil
	})
}

// ListEmbeddedScripts lists all embedded scripts
func ListEmbeddedScripts() ([]string, error) {
	var scripts []string

	err := fs.WalkDir(ScriptsFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && path != "." {
			scripts = append(scripts, path)
		}
		return nil
	})

	return scripts, err
}

// GetScriptContent returns the content of a specific embedded script
func GetScriptContent(scriptName string) ([]byte, error) {
	return ScriptsFS.ReadFile(scriptName)
}
