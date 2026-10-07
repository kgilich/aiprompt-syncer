package promptsync

import (
	"fmt"
	"os"
	"path/filepath"
)

type starterFile struct {
	path    string
	content string
}

func Init(root string) error {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolve project directory: %w", err)
	}

	files := []starterFile{
		{path: "promptsync.yaml", content: starterConfig},
		{path: "prompts/master.md", content: starterPrompt},
		{path: "templates/copilot.tmpl", content: "{{ .Content }}\n"},
		{path: "templates/claude.tmpl", content: "{{ .Content }}\n"},
		{path: "templates/cursor.tmpl", content: "{{ .Content }}\n"},
	}
	for _, file := range files {
		path := filepath.Join(absRoot, file.path)
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("refusing to overwrite existing file %q", path)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("check file %q: %w", path, err)
		}
	}

	for _, file := range files {
		path := filepath.Join(absRoot, file.path)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("create directory for %q: %w", path, err)
		}
		if err := os.WriteFile(path, []byte(file.content), 0o644); err != nil {
			return fmt.Errorf("write starter file %q: %w", path, err)
		}
	}
	return nil
}

const starterConfig = `version: 1
source: prompts/master.md
variables:
  Project: My Project
targets:
  - path: .github/copilot-instructions.md
    template: templates/copilot.tmpl
  - path: CLAUDE.md
    template: templates/claude.tmpl
  - path: .cursorrules
    template: templates/cursor.tmpl
`

const starterPrompt = `# Project instructions for {{ .Project }}

Describe shared engineering conventions, architecture, and coding guidance here.
`