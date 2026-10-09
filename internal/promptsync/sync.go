package promptsync

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"text/template"
)

type TargetState string

const (
	TargetMissing  TargetState = "missing"
	TargetCurrent  TargetState = "current"
	TargetModified TargetState = "modified"
)

type TargetStatus struct {
	Path  string
	State TargetState
}

type generatedTarget struct {
	path    string
	content []byte
}

type SyncOptions struct {
	Force bool
}

func Sync(configPath string) ([]string, error) {
	return SyncWithOptions(configPath, SyncOptions{})
}

func SyncWithOptions(configPath string, options SyncOptions) ([]string, error) {
	targets, root, allowExternal, err := generateTargets(configPath)
	if err != nil {
		return nil, err
	}
	modified := make([]string, 0)
	for _, target := range targets {
		if err := validatePathWithin(root, target.path, allowExternal, "target"); err != nil {
			return nil, err
		}
		outputPath := resolve(root, target.path)
		current, err := os.ReadFile(outputPath)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("inspect target %q: %w", outputPath, err)
		}
		if !slices.Equal(current, target.content) {
			modified = append(modified, target.path)
		}
	}
	if len(modified) > 0 && !options.Force {
		return nil, fmt.Errorf("refusing to overwrite modified targets: %s; inspect with 'promptsync status' or rerun with --force", strings.Join(modified, ", "))
	}

	written := make([]string, 0, len(targets))
	for _, target := range targets {
		outputPath := resolve(root, target.path)
		current, err := os.ReadFile(outputPath)
		if err == nil && slices.Equal(current, target.content) {
			continue
		}
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("inspect target %q: %w", outputPath, err)
		}
		if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
			return nil, fmt.Errorf("create directory for %q: %w", outputPath, err)
		}
		if err := os.WriteFile(outputPath, target.content, 0o644); err != nil {
			return nil, fmt.Errorf("write target %q: %w", outputPath, err)
		}
		written = append(written, target.path)
	}
	return written, nil
}

func Inspect(configPath string) ([]TargetStatus, error) {
	targets, root, allowExternal, err := generateTargets(configPath)
	if err != nil {
		return nil, err
	}

	statuses := make([]TargetStatus, 0, len(targets))
	for _, target := range targets {
		if err := validatePathWithin(root, target.path, allowExternal, "target"); err != nil {
			return nil, err
		}
		path := resolve(root, target.path)
		current, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			statuses = append(statuses, TargetStatus{Path: target.path, State: TargetMissing})
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read target %q: %w", path, err)
		}
		state := TargetModified
		if slices.Equal(current, target.content) {
			state = TargetCurrent
		}
		statuses = append(statuses, TargetStatus{Path: target.path, State: state})
	}
	return statuses, nil
}

func generateTargets(configPath string) ([]generatedTarget, string, bool, error) {
	config, root, err := loadConfig(configPath)
	if err != nil {
		return nil, "", false, err
	}
	library, err := libraryRoot(config, root)
	if err != nil {
		return nil, "", false, err
	}
	if err := validatePathWithin(library, config.Source, false, "source"); err != nil {
		return nil, "", false, err
	}

	sourcePath := resolve(library, config.Source)
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		return nil, "", false, fmt.Errorf("read source %q: %w", sourcePath, err)
	}

	content, err := render("source", string(source), config.Variables)
	if err != nil {
		return nil, "", false, err
	}

	generated := make([]generatedTarget, 0, len(config.Targets))
	for _, target := range config.Targets {
		output := content
		if target.Template != "" {
			values := make(map[string]any, len(config.Variables)+1)
			for key, value := range config.Variables {
				values[key] = value
			}
			values["Content"] = content

			if err := validatePathWithin(library, target.Template, false, "template"); err != nil {
				return nil, "", false, err
			}
			templatePath := resolve(library, target.Template)
			templateSource, err := os.ReadFile(templatePath)
			if err != nil {
				return nil, "", false, fmt.Errorf("read template %q: %w", templatePath, err)
			}
			output, err = render(target.Template, string(templateSource), values)
			if err != nil {
				return nil, "", false, err
			}
		}
		generated = append(generated, generatedTarget{path: target.Path, content: []byte(output)})
	}
	return generated, root, config.AllowExternalTargets, nil
}

func render(name, source string, values any) (string, error) {
	source = canonicalizeLineEndings(source)
	tmpl, err := template.New(name).Option("missingkey=error").Parse(source)
	if err != nil {
		return "", fmt.Errorf("parse template %q: %w", name, err)
	}

	var output bytes.Buffer
	if err := tmpl.Execute(&output, values); err != nil {
		return "", fmt.Errorf("render template %q: %w", name, err)
	}
	return canonicalizeLineEndings(output.String()), nil
}

func canonicalizeLineEndings(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	return strings.ReplaceAll(value, "\r", "\n")
}

func resolve(root, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
}

func validatePathWithin(root, path string, allowExternal bool, kind string) error {
	if allowExternal {
		return nil
	}
	projectRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("resolve root directory %q: %w", root, err)
	}
	projectRoot, err = filepath.Abs(projectRoot)
	if err != nil {
		return fmt.Errorf("resolve project directory: %w", err)
	}
	candidate, err := filepath.Abs(resolve(root, path))
	if err != nil {
		return fmt.Errorf("resolve %s path %q: %w", kind, path, err)
	}
	for {
		if _, err := os.Lstat(candidate); err == nil {
			resolved, err := filepath.EvalSymlinks(candidate)
			if err != nil {
				return fmt.Errorf("resolve %s path %q: %w", kind, path, err)
			}
			resolved, err = filepath.Abs(resolved)
			if err != nil {
				return fmt.Errorf("resolve %s path %q: %w", kind, path, err)
			}
			relative, err := filepath.Rel(projectRoot, resolved)
			if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
				if kind == "target" {
					return fmt.Errorf("target path %q resolves outside the project; set allow_external_targets: true to permit it", path)
				}
				return fmt.Errorf("%s path %q resolves outside its root", kind, path)
			}
			return nil
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("inspect %s path %q: %w", kind, path, err)
		}
		parent := filepath.Dir(candidate)
		if parent == candidate {
			return fmt.Errorf("%s path %q is outside its root", kind, path)
		}
		candidate = parent
	}
}
