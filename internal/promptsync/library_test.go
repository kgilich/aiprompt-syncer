package promptsync

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateAndInstallUsePinnedCommits(t *testing.T) {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not available")
	}

	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	working := filepath.Join(root, "working")
	cacheRoot := filepath.Join(root, "cache")
	lockPath := filepath.Join(root, "promptsync.lock")
	library := Library{Repository: remote, Ref: "main"}
	runGitTest(t, gitPath, "init", "--bare", remote)
	runGitTest(t, gitPath, "init", working)
	if err := os.WriteFile(filepath.Join(working, "master.md"), []byte("initial prompt\n"), 0o644); err != nil {
		t.Fatalf("write initial source file: %v", err)
	}
	runGitTest(t, gitPath, "-C", working, "add", "master.md")
	runGitTest(t, gitPath, "-C", working, "-c", "user.name=PromptSync Test", "-c", "user.email=promptsync@example.invalid", "commit", "-m", "initial")
	runGitTest(t, gitPath, "-C", working, "branch", "-M", "main")
	runGitTest(t, gitPath, "-C", working, "remote", "add", "origin", remote)
	runGitTest(t, gitPath, "-C", working, "push", "-u", "origin", "main")

	initial, err := updateLibrary(gitPath, library, lockPath, cacheRoot)
	if err != nil {
		t.Fatalf("initial update: %v", err)
	}
	lock, err := readLibraryLock(lockPath)
	if err != nil {
		t.Fatalf("read lockfile: %v", err)
	}
	if lock.Commit != initial || lock.Repository != library.Repository {
		t.Fatalf("lock = %+v, want initial commit %q", lock, initial)
	}
	cleanProject := filepath.Join(root, "clean-project")
	if err := os.MkdirAll(cleanProject, 0o755); err != nil {
		t.Fatalf("create clean project: %v", err)
	}
	if err := writeLibraryLock(filepath.Join(cleanProject, "promptsync.lock"), lock); err != nil {
		t.Fatalf("write clean project lock: %v", err)
	}
	config := Config{Library: &library}
	if _, err := libraryRootInCache(config, cleanProject, filepath.Join(root, "empty-cache")); err == nil || !strings.Contains(err.Error(), "promptsync install") {
		t.Fatalf("libraryRootInCache() error = %v, want offline install instruction", err)
	}
	initialPath := librarySnapshotPath(cacheRoot, library, initial)
	contents, err := os.ReadFile(filepath.Join(initialPath, "master.md"))
	if err != nil || normalizeLineEndings(contents) != "initial prompt\n" {
		t.Fatalf("initial snapshot contents = %q, error = %v", contents, err)
	}

	if err := os.WriteFile(filepath.Join(working, "master.md"), []byte("updated prompt\n"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}
	runGitTest(t, gitPath, "-C", working, "add", "master.md")
	runGitTest(t, gitPath, "-C", working, "-c", "user.name=PromptSync Test", "-c", "user.email=promptsync@example.invalid", "commit", "-m", "update prompt")
	runGitTest(t, gitPath, "-C", working, "push", "origin", "main")

	updated, err := updateLibrary(gitPath, library, lockPath, cacheRoot)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if initial == updated {
		t.Fatal("update did not advance cached revision")
	}
	contents, err = os.ReadFile(filepath.Join(librarySnapshotPath(cacheRoot, library, updated), "master.md"))
	if err != nil {
		t.Fatalf("read updated prompt: %v", err)
	}
	if normalizeLineEndings(contents) != "updated prompt\n" {
		t.Fatalf("cached prompt = %q", contents)
	}
	lockBeforeInstall, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("read updated lockfile: %v", err)
	}

	if err := installLibrary(gitPath, library, initial, filepath.Join(root, "restore-cache")); err != nil {
		t.Fatalf("install locked revision: %v", err)
	}
	lockAfterInstall, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("read lockfile after install: %v", err)
	}
	if string(lockAfterInstall) != string(lockBeforeInstall) {
		t.Fatal("install changed the lockfile")
	}
	contents, err = os.ReadFile(filepath.Join(librarySnapshotPath(filepath.Join(root, "restore-cache"), library, initial), "master.md"))
	if err != nil {
		t.Fatalf("read locked prompt: %v", err)
	}
	if normalizeLineEndings(contents) != "initial prompt\n" {
		t.Fatalf("locked prompt = %q, want initial prompt", contents)
	}

	cleanCache := filepath.Join(root, "clean-cache")
	if err := installLibrary(gitPath, library, initial, cleanCache); err != nil {
		t.Fatalf("install from lock into clean cache: %v", err)
	}
	lockedRoot, err := libraryRootInCache(config, cleanProject, cleanCache)
	if err != nil {
		t.Fatalf("resolve locked library after install: %v", err)
	}
	contents, err = os.ReadFile(filepath.Join(lockedRoot, "master.md"))
	if err != nil || normalizeLineEndings(contents) != "initial prompt\n" {
		t.Fatalf("clean-cache locked contents = %q, error = %v", contents, err)
	}
}

func TestCachedSnapshotRequiresInstall(t *testing.T) {
	library := Library{Repository: "https://example.com/prompts.git", Ref: "main"}
	if _, err := cachedSnapshot(t.TempDir(), library, strings.Repeat("a", 40)); err == nil || !strings.Contains(err.Error(), "promptsync install") {
		t.Fatalf("cachedSnapshot() error = %v, want install instruction", err)
	}
}

func TestConfigRejectsNonHTTPSLibraryRepository(t *testing.T) {
	config := Config{
		Version: 1,
		Library: &Library{Repository: "git@github.com:owner/prompts.git", Ref: "main"},
		Source:  "prompts/master.md",
		Targets: []Target{{Path: "CLAUDE.md"}},
	}
	if err := config.validate(); err == nil {
		t.Fatal("validate() accepted a non-HTTPS repository")
	}
}

func TestInspectRejectsSymlinkTargetOutsideProject(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	writeTestFile(t, root, "promptsync.yaml", `version: 1
source: prompts/master.md
targets:
  - path: linked/CLAUDE.md
`)
	writeTestFile(t, root, "prompts/master.md", "rules\n")
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Skipf("directory symlinks are unavailable: %v", err)
	}
	if _, err := Inspect(filepath.Join(root, "promptsync.yaml")); err == nil || !strings.Contains(err.Error(), "outside the project") {
		t.Fatalf("Inspect() error = %v, want symlink escape rejection", err)
	}
}

func TestInspectRejectsSourceOutsideProject(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "secret.md"), []byte("not a prompt"), 0o644); err != nil {
		t.Fatalf("write outside source: %v", err)
	}
	writeTestFile(t, root, "project/promptsync.yaml", `version: 1
source: ../secret.md
targets:
  - path: CLAUDE.md
`)
	if _, err := Inspect(filepath.Join(root, "project", "promptsync.yaml")); err == nil || !strings.Contains(err.Error(), "outside its root") {
		t.Fatalf("Inspect() error = %v, want source escape rejection", err)
	}
}

func runGitTest(t *testing.T, gitPath string, args ...string) {
	t.Helper()
	if _, err := runGit(gitPath, args...); err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
}

func normalizeLineEndings(contents []byte) string {
	return strings.ReplaceAll(string(contents), "\r\n", "\n")
}
