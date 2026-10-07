package promptsync

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	lockPath := filepath.Join(root, "promptsync.lock")

	cachePath, err := libraryCachePath(*config.Library)
	if err != nil {
		return "", err
	}
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return "", fmt.Errorf("git is required to update a remote library: %w", err)
	}

	if err := updateGitRepository(gitPath, config.Library.Repository, config.Library.Ref, cachePath); err != nil {
		return "", err
	}

	commit, err := runGit(gitPath, "-C", cachePath, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	commit = strings.TrimSpace(commit)
	lock := LibraryLock{Version: 1, Repository: config.Library.Repository, Ref: config.Library.Ref, Commit: commit}
	if err := writeLibraryLock(lockPath, lock); err != nil {
		return "", err
	}
	return commit[:12], nil
}

func libraryRoot(config Config, projectRoot string) (string, error) {
	if config.Library == nil {
		return projectRoot, nil
	}
	lockPath := filepath.Join(projectRoot, "promptsync.lock")
	lock, err := readLibraryLock(lockPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("remote library is not locked; run 'promptsync update' first")
		}
		return "", err
	}
	if lock.Repository != config.Library.Repository || lock.Ref != config.Library.Ref {
		return "", fmt.Errorf("promptsync.lock does not match the configured library; run 'promptsync update'")
	}
	if !validCommit(lock.Commit) {
		return "", fmt.Errorf("promptsync.lock contains an invalid commit SHA")
	}

	cachePath, err := libraryCachePath(*config.Library)
	if err != nil {
		return "", err
	}
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return "", fmt.Errorf("git is required to use the remote library: %w", err)
	}
	if _, err := os.Stat(filepath.Join(cachePath, ".git")); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("remote library is not cached; run 'promptsync update' first")
		}
		return "", fmt.Errorf("inspect library cache: %w", err)
	}
	if err := checkoutLibraryCommit(gitPath, cachePath, lock.Commit); err != nil {
		return "", err
	}
	return cachePath, nil
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

func validCommit(commit string) bool {
	if len(commit) != 40 && len(commit) != 64 {
		return false
	}
	_, err := hex.DecodeString(commit)
	return err == nil
}

func checkoutLibraryCommit(gitPath, cachePath, commit string) error {
	if _, err := runGit(gitPath, "-C", cachePath, "cat-file", "-e", commit+"^{commit}"); err != nil {
		if _, fetchErr := runGit(gitPath, "-C", cachePath, "fetch", "--depth=1", "origin", commit); fetchErr != nil {
			return fmt.Errorf("fetch locked library commit %s: %w", commit, fetchErr)
		}
	}
	if _, err := runGit(gitPath, "-C", cachePath, "checkout", "--detach", commit); err != nil {
		return err
	}
	return nil
}

func libraryCachePath(library Library) (string, error) {
	cacheRoot, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("locate user cache directory: %w", err)
	}
	key := sha256.Sum256([]byte(library.Repository + "\x00" + library.Ref))
	return filepath.Join(cacheRoot, "promptsync", "libraries", hex.EncodeToString(key[:])), nil
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

func updateGitRepository(gitPath, repository, ref, cachePath string) error {
	if _, err := os.Stat(filepath.Join(cachePath, ".git")); err == nil {
		if _, err := runGit(gitPath, "-C", cachePath, "fetch", "--depth=1", "origin", ref); err != nil {
			return err
		}
		if _, err := runGit(gitPath, "-C", cachePath, "checkout", "--detach", "FETCH_HEAD"); err != nil {
			return err
		}
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect library cache: %w", err)
	}
	if _, err := os.Stat(cachePath); err == nil {
		return fmt.Errorf("library cache path exists but is not a Git repository: %s", cachePath)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect library cache path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		return fmt.Errorf("create library cache directory: %w", err)
	}
	if _, err := runGit(gitPath, "clone", "--depth=1", "--single-branch", "--branch", ref, "--", repository, cachePath); err != nil {
		return err
	}
	return nil
}
