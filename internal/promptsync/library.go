package promptsync

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func Update(configPath string) (string, error) {
	config, root, err := loadConfig(configPath)
	if err != nil {
		return "", err
	}
	if config.Library == nil {
		return "", fmt.Errorf("no remote library configured")
	}
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return "", fmt.Errorf("git is required to update a remote library: %w", err)
	}
	cacheRoot, err := userLibraryCacheRoot()
	if err != nil {
		return "", err
	}
	commit, err := updateLibrary(gitPath, *config.Library, filepath.Join(root, "promptsync.lock"), cacheRoot)
	if err != nil {
		return "", err
	}
	return commit[:12], nil
}

func Install(configPath string) (string, error) {
	config, root, err := loadConfig(configPath)
	if err != nil {
		return "", err
	}
	if config.Library == nil {
		return "", fmt.Errorf("no remote library configured")
	}
	lock, err := readLibraryLock(filepath.Join(root, "promptsync.lock"))
	if err != nil {
		return "", err
	}
	if err := validateLibraryLock(lock, *config.Library); err != nil {
		return "", err
	}
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return "", fmt.Errorf("git is required to install a remote library: %w", err)
	}
	cacheRoot, err := userLibraryCacheRoot()
	if err != nil {
		return "", err
	}
	if err := installLibrary(gitPath, *config.Library, lock.Commit, cacheRoot); err != nil {
		return "", err
	}
	return lock.Commit[:12], nil
}

func libraryRoot(config Config, projectRoot string) (string, error) {
	if config.Library == nil {
		return projectRoot, nil
	}
	cacheRoot, err := userLibraryCacheRoot()
	if err != nil {
		return "", err
	}
	return libraryRootInCache(config, projectRoot, cacheRoot)
}

func libraryRootInCache(config Config, projectRoot, cacheRoot string) (string, error) {
	if config.Library == nil {
		return projectRoot, nil
	}
	lock, err := readLibraryLock(filepath.Join(projectRoot, "promptsync.lock"))
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("remote library is not locked; run 'promptsync update' first")
		}
		return "", err
	}
	if err := validateLibraryLock(lock, *config.Library); err != nil {
		return "", err
	}
	return cachedSnapshot(cacheRoot, *config.Library, lock.Commit)
}

type LibraryLock struct {
	Version    int    `json:"version"`
	Repository string `json:"repository"`
	Ref        string `json:"ref"`
	Commit     string `json:"commit"`
}

func readLibraryLock(path string) (LibraryLock, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return LibraryLock{}, err
	}
	var lock LibraryLock
	if err := json.Unmarshal(contents, &lock); err != nil {
		return LibraryLock{}, fmt.Errorf("parse lockfile %q: %w", path, err)
	}
	if lock.Version != 1 {
		return LibraryLock{}, fmt.Errorf("unsupported lockfile version %d", lock.Version)
	}
	return lock, nil
}

func writeLibraryLock(path string, lock LibraryLock) error {
	contents, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return fmt.Errorf("encode lockfile: %w", err)
	}
	contents = append(contents, '\n')
	if err := os.WriteFile(path, contents, 0o644); err != nil {
		return fmt.Errorf("write lockfile %q: %w", path, err)
	}
	return nil
}

func validateLibraryLock(lock LibraryLock, library Library) error {
	if lock.Repository != library.Repository || lock.Ref != library.Ref {
		return fmt.Errorf("promptsync.lock does not match the configured library; run 'promptsync update'")
	}
	if !validCommit(lock.Commit) {
		return fmt.Errorf("promptsync.lock contains an invalid commit SHA")
	}
	return nil
}

func validCommit(commit string) bool {
	if len(commit) != 40 && len(commit) != 64 {
		return false
	}
	_, err := hex.DecodeString(commit)
	return err == nil
}

func userLibraryCacheRoot() (string, error) {
	cacheRoot, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("locate user cache directory: %w", err)
	}
	return filepath.Join(cacheRoot, "promptsync", "libraries"), nil
}

func librarySnapshotPath(cacheRoot string, library Library, commit string) string {
	key := sha256.Sum256([]byte(library.Repository + "\x00" + commit))
	return filepath.Join(cacheRoot, hex.EncodeToString(key[:]))
}

func cachedSnapshot(cacheRoot string, library Library, commit string) (string, error) {
	path := librarySnapshotPath(cacheRoot, library, commit)
	if info, err := os.Stat(filepath.Join(path, ".git")); err == nil && info.IsDir() {
		return path, nil
	} else if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("inspect cached library snapshot: %w", err)
	}
	return "", fmt.Errorf("locked library revision is not installed; run 'promptsync install'")
}

func updateLibrary(gitPath string, library Library, lockPath, cacheRoot string) (commitResult string, returnErr error) {
	if err := os.MkdirAll(cacheRoot, 0o755); err != nil {
		return "", fmt.Errorf("create library cache directory: %w", err)
	}
	temporary, err := os.MkdirTemp(cacheRoot, "update-")
	if err != nil {
		return "", fmt.Errorf("create temporary library checkout: %w", err)
	}
	defer func() {
		if cleanupErr := os.RemoveAll(temporary); cleanupErr != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("clean temporary library checkout: %w", cleanupErr))
		}
	}()

	if _, err := runGit(gitPath, "clone", "--depth=1", "--single-branch", "--branch", library.Ref, "--", library.Repository, temporary); err != nil {
		return "", err
	}
	commit, err := runGit(gitPath, "-C", temporary, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	commit = strings.TrimSpace(commit)
	if !validCommit(commit) {
		return "", fmt.Errorf("git returned an invalid commit SHA %q", commit)
	}
	if err := installSnapshot(temporary, library, commit, cacheRoot); err != nil {
		return "", err
	}
	lock := LibraryLock{Version: 1, Repository: library.Repository, Ref: library.Ref, Commit: commit}
	if err := writeLibraryLock(lockPath, lock); err != nil {
		return "", err
	}
	return commit, nil
}

func installLibrary(gitPath string, library Library, commit, cacheRoot string) (returnErr error) {
	if !validCommit(commit) {
		return fmt.Errorf("promptsync.lock contains an invalid commit SHA")
	}
	if _, err := cachedSnapshot(cacheRoot, library, commit); err == nil {
		return nil
	}
	if err := os.MkdirAll(cacheRoot, 0o755); err != nil {
		return fmt.Errorf("create library cache directory: %w", err)
	}
	temporary, err := os.MkdirTemp(cacheRoot, "install-")
	if err != nil {
		return fmt.Errorf("create temporary library checkout: %w", err)
	}
	defer func() {
		if cleanupErr := os.RemoveAll(temporary); cleanupErr != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("clean temporary library checkout: %w", cleanupErr))
		}
	}()

	if _, err := runGit(gitPath, "init", temporary); err != nil {
		return err
	}
	if _, err := runGit(gitPath, "-C", temporary, "remote", "add", "origin", library.Repository); err != nil {
		return err
	}
	if _, err := runGit(gitPath, "-C", temporary, "fetch", "--depth=1", "origin", commit); err != nil {
		return fmt.Errorf("fetch locked library commit %s: %w", commit, err)
	}
	if _, err := runGit(gitPath, "-C", temporary, "checkout", "--detach", "FETCH_HEAD"); err != nil {
		return err
	}
	actual, err := runGit(gitPath, "-C", temporary, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if strings.TrimSpace(actual) != commit {
		return fmt.Errorf("fetched library commit %s does not match lockfile commit %s", strings.TrimSpace(actual), commit)
	}
	return installSnapshot(temporary, library, commit, cacheRoot)
}

func installSnapshot(temporary string, library Library, commit, cacheRoot string) error {
	destination := librarySnapshotPath(cacheRoot, library, commit)
	if _, err := os.Stat(filepath.Join(destination, ".git")); err == nil {
		return nil
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("inspect library snapshot: %w", err)
	}
	if err := os.Rename(temporary, destination); err != nil {
		if _, statErr := os.Stat(filepath.Join(destination, ".git")); statErr == nil {
			return nil
		}
		return fmt.Errorf("cache library snapshot: %w", err)
	}
	return nil
}

func runGit(gitPath string, args ...string) (string, error) {
	command := exec.Command(gitPath, args...)
	command.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}
