package promptsync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyncRendersMarkdownAndTargetTemplate(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "promptsync.yaml", `version: 1
source: prompts/master.md
variables:
  Project: Example App
targets:
  - path: .github/copilot-instructions.md
    template: templates/copilot.tmpl
`)
	writeTestFile(t, root, "prompts/master.md", "# {{ .Project }}\n\nShared rules.\n")
	writeTestFile(t, root, "templates/copilot.tmpl", "<!-- generated -->\n{{ .Content }}")

	written, err := Sync(filepath.Join(root, "promptsync.yaml"))
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if len(written) != 1 || written[0] != ".github/copilot-instructions.md" {
		t.Fatalf("Sync() paths = %v", written)
	}

	output, err := os.ReadFile(filepath.Join(root, ".github", "copilot-instructions.md"))
	if err != nil {
		t.Fatalf("read generated target: %v", err)
	}
	if got := string(output); !strings.Contains(got, "<!-- generated -->") || !strings.Contains(got, "# Example App") {
		t.Fatalf("generated output = %q", got)
	}
}

func TestSyncProducesSameOutputForLFAndCRLFInputs(t *testing.T) {
	lfRoot := t.TempDir()
	crlfRoot := t.TempDir()
	configs := []string{filepath.Join(lfRoot, "promptsync.yaml"), filepath.Join(crlfRoot, "promptsync.yaml")}
	roots := []string{lfRoot, crlfRoot}
	lineEndings := []string{"\n", "\r\n"}
	for index, root := range roots {
		lineEnding := lineEndings[index]
		writeTestFile(t, root, "promptsync.yaml", "version: 1\nsource: prompts/master.md\ntargets:\n  - path: CLAUDE.md\n    template: templates/claude.tmpl\n")
		writeTestFile(t, root, "prompts/master.md", "# Shared rules"+lineEnding+"Second line"+lineEnding)
		writeTestFile(t, root, "templates/claude.tmpl", "<!-- generated -->"+lineEnding+"{{ .Content }}")
		if _, err := Sync(configs[index]); err != nil {
			t.Fatalf("Sync() for %q inputs: %v", lineEnding, err)
		}
	}

	lfOutput, err := os.ReadFile(filepath.Join(lfRoot, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("read LF output: %v", err)
	}
	crlfOutput, err := os.ReadFile(filepath.Join(crlfRoot, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("read CRLF output: %v", err)
	}
	if string(lfOutput) != string(crlfOutput) {
		t.Fatalf("outputs differ by input line endings: LF %q, CRLF %q", lfOutput, crlfOutput)
	}
	if strings.ContainsRune(string(crlfOutput), '\r') {
		t.Fatalf("generated output contains carriage returns: %q", crlfOutput)
	}
}

func TestInspectReportsMissingCurrentAndModifiedTargets(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "promptsync.yaml")
	writeTestFile(t, root, "promptsync.yaml", `version: 1
source: prompts/master.md
targets:
  - path: CLAUDE.md
`)
	writeTestFile(t, root, "prompts/master.md", "Shared rules.\n")

	statuses, err := Inspect(configPath)
	if err != nil {
		t.Fatalf("Inspect() for missing target error = %v", err)
	}
	if len(statuses) != 1 || statuses[0].State != TargetMissing {
		t.Fatalf("Inspect() for missing target = %v", statuses)
	}
	if _, err := os.Stat(filepath.Join(root, "CLAUDE.md")); !os.IsNotExist(err) {
		t.Fatalf("Inspect() created missing target, stat error = %v", err)
	}

	if _, err := Sync(configPath); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	statuses, err = Inspect(configPath)
	if err != nil {
		t.Fatalf("Inspect() for current target error = %v", err)
	}
	if statuses[0].State != TargetCurrent {
		t.Fatalf("Inspect() state = %q, want %q", statuses[0].State, TargetCurrent)
	}

	writeTestFile(t, root, "CLAUDE.md", "manual edit\n")
	statuses, err = Inspect(configPath)
	if err != nil {
		t.Fatalf("Inspect() for modified target error = %v", err)
	}
	if statuses[0].State != TargetModified {
		t.Fatalf("Inspect() state = %q, want %q", statuses[0].State, TargetModified)
	}
}

func TestConfigRejectsDuplicateTargets(t *testing.T) {
	config := Config{
		Version: 1,
		Source:  "prompts/master.md",
		Targets: []Target{{Path: "CLAUDE.md"}, {Path: "./CLAUDE.md"}},
	}
	if err := config.validate(); err == nil {
		t.Fatal("validate() accepted duplicate target paths")
	}
}

func TestSyncProtectsModifiedTargetsUnlessForced(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "promptsync.yaml")
	writeTestFile(t, root, "promptsync.yaml", `version: 1
source: prompts/master.md
targets:
  - path: CLAUDE.md
`)
	writeTestFile(t, root, "prompts/master.md", "generated rules\n")
	writeTestFile(t, root, "CLAUDE.md", "manual changes\n")

	if _, err := Sync(configPath); err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("Sync() error = %v, want overwrite guard", err)
	}
	contents, err := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("read protected target: %v", err)
	}
	if string(contents) != "manual changes\n" {
		t.Fatalf("protected target contents = %q", contents)
	}

	if _, err := SyncWithOptions(configPath, SyncOptions{Force: true}); err != nil {
		t.Fatalf("SyncWithOptions(force) error = %v", err)
	}
	contents, err = os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("read forced target: %v", err)
	}
	if string(contents) != "generated rules\n" {
		t.Fatalf("forced target contents = %q", contents)
	}
}

func TestSyncDoesNotPartiallyWriteWhenAnyTargetIsModified(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "promptsync.yaml")
	writeTestFile(t, root, "promptsync.yaml", `version: 1
source: prompts/master.md
targets:
  - path: generated/CLAUDE.md
  - path: COPILOT.md
`)
	writeTestFile(t, root, "prompts/master.md", "generated rules\n")
	writeTestFile(t, root, "COPILOT.md", "manual changes\n")

	if _, err := Sync(configPath); err == nil {
		t.Fatal("Sync() succeeded despite a modified target")
	}
	if _, err := os.Stat(filepath.Join(root, "generated", "CLAUDE.md")); !os.IsNotExist(err) {
		t.Fatalf("sync partially wrote first target, stat error = %v", err)
	}
}

func TestConfigRejectsTargetOutsideProjectByDefault(t *testing.T) {
	config := Config{
		Version: 1,
		Source:  "prompts/master.md",
		Targets: []Target{{Path: "../outside.md"}},
	}
	if err := config.validate(); err == nil {
		t.Fatal("validate() accepted a target outside the project")
	}
	config.AllowExternalTargets = true
	if err := config.validate(); err != nil {
		t.Fatalf("validate() rejected explicitly allowed external target: %v", err)
	}
}

func TestInitDoesNotOverwriteExistingFiles(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "promptsync.yaml", "user config")
	if err := Init(root); err == nil {
		t.Fatal("Init() overwrote an existing file")
	}
}

func writeTestFile(t *testing.T, root, path, contents string) {
	t.Helper()
	fullPath := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("create test directory: %v", err)
	}
	if err := os.WriteFile(fullPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
}
