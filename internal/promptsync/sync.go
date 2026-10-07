package promptsync

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

func Sync(configPath string) ([]string, error) {
	config, root, err := loadConfig(configPath)
	if err != nil {
		return nil, err
	}

	sourcePath := resolve(root, config.Source)
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("read source %q: %w", sourcePath, err)
	}

	content, err := render("source", string(source), config.Variables)
	if err != nil {
		return nil, err
	}

	written := make([]string, 0, len(config.Targets))
	for _, target := range config.Targets {
		output := content
		if target.Template != "" {
			values := make(map[string]any, len(config.Variables)+1)
			for key, value := range config.Variables {
				values[key] = value
			}
			values["Content"] = content

			templatePath := resolve(root, target.Template)
			templateSource, err := os.ReadFile(templatePath)
			if err != nil {
				return nil, fmt.Errorf("read template %q: %w", templatePath, err)
			}
			output, err = render(target.Template, string(templateSource), values)
			if err != nil {
				return nil, err
			}
		}

		outputPath := resolve(root, target.Path)
		if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
			return nil, fmt.Errorf("create directory for %q: %w", outputPath, err)
		}
		if err := os.WriteFile(outputPath, []byte(output), 0o644); err != nil {
			return nil, fmt.Errorf("write target %q: %w", outputPath, err)
		}
		written = append(written, target.Path)
	}
	return written, nil
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