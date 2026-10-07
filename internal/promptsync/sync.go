package promptsync

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
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

func Sync(configPath string) ([]string, error) {
	targets, root, err := generateTargets(configPath)
	if err != nil {
		return nil, err
	}

	written := make([]string, 0, len(targets))
	for _, target := range targets {
		outputPath := resolve(root, target.path)
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
	targets, root, err := generateTargets(configPath)
	if err != nil {
		return nil, err
	}

	statuses := make([]TargetStatus, 0, len(targets))
	for _, target := range targets {
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

func generateTargets(configPath string) ([]generatedTarget, string, error) {
	config, root, err := loadConfig(configPath)
	if err != nil {
		return nil, "", err
	}
	library, err := libraryRoot(config, root)
	if err != nil {
		return nil, "", err
	}

	sourcePath := resolve(library, config.Source)
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		return nil, "", fmt.Errorf("read source %q: %w", sourcePath, err)
	}

	content, err := render("source", string(source), config.Variables)
	if err != nil {
		return nil, "", err
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

			templatePath := resolve(library, target.Template)
			templateSource, err := os.ReadFile(templatePath)
			if err != nil {
				return nil, "", fmt.Errorf("read template %q: %w", templatePath, err)
			}
			output, err = render(target.Template, string(templateSource), values)
			if err != nil {
				return nil, "", err
			}
		}
		generated = append(generated, generatedTarget{path: target.Path, content: []byte(output)})
	}
	return generated, root, nil
}

func render(name, source string, values any) (string, error) {
	tmpl, err := template.New(name).Option("missingkey=error").Parse(source)
	if err != nil {
		return "", fmt.Errorf("parse template %q: %w", name, err)
	}

	var output bytes.Buffer
	if err := tmpl.Execute(&output, values); err != nil {
		return "", fmt.Errorf("render template %q: %w", name, err)
	}
	return output.String(), nil
}

func resolve(root, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
}
