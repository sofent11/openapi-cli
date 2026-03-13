package openapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"time"
)

var specHTTPClient = &http.Client{Timeout: 30 * time.Second}

type CreateProjectOptions struct {
	Source     string
	BaseURL    string
	AuthHeader string
	AuthToken  string
	UpdateURL  string
	SpecType   string
}

func BaseDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home: %w", err)
	}
	return filepath.Join(home, DefaultConfigDirName, "apis"), nil
}

func ListProjects() ([]string, error) {
	baseDir, err := BaseDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("read api directory: %w", err)
	}
	var projects []string
	for _, entry := range entries {
		if entry.IsDir() {
			projects = append(projects, entry.Name())
		}
	}
	sort.Strings(projects)
	return projects, nil
}

func ProjectDir(project string) (string, error) {
	if project == "" {
		return "", errors.New("project name is required; use -p <project>, for example: openapi list -p erp")
	}
	baseDir, err := BaseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(baseDir, project), nil
}

func ReadConfig(project string) (Config, string, error) {
	projectDir, err := ProjectDir(project)
	if err != nil {
		return Config{}, "", err
	}
	configPath := filepath.Join(projectDir, "setting.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, configPath, fmt.Errorf("project config not found: %s; create the project first with `openapi config create -p %s <local-file-or-url>`", configPath, project)
		}
		return Config{}, configPath, fmt.Errorf("read project config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, configPath, fmt.Errorf("parse project config: %s; fix the JSON in %s", err, configPath)
	}
	return cfg, configPath, nil
}

func ReadConfigIfExists(project string) (Config, string, error) {
	projectDir, err := ProjectDir(project)
	if err != nil {
		return Config{}, "", err
	}
	configPath := filepath.Join(projectDir, "setting.json")
	if _, err := os.Stat(configPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, configPath, nil
		}
		return Config{}, configPath, fmt.Errorf("read project config: %w", err)
	}
	return ReadConfig(project)
}

func WriteConfig(project string, cfg Config) (string, error) {
	projectDir, err := ProjectDir(project)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		return "", fmt.Errorf("create project directory: %w", err)
	}
	configPath := filepath.Join(projectDir, "setting.json")
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode project config: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		return "", fmt.Errorf("write project config: %w", err)
	}
	return configPath, nil
}

func FindSpecPath(project string) (string, error) {
	projectDir, err := ProjectDir(project)
	if err != nil {
		return "", err
	}
	candidates := []string{"api.json", "api.yaml", "api.yml"}
	for _, candidate := range candidates {
		fullPath := filepath.Join(projectDir, candidate)
		if _, err := os.Stat(fullPath); err == nil {
			return fullPath, nil
		}
	}
	return "", fmt.Errorf("api spec not found in %s; expected api.json, api.yaml, or api.yml. You can import one with `openapi config create -p %s <local-file-or-url>`", projectDir, project)
}

func CreateProject(project string, opts CreateProjectOptions) (string, string, error) {
	projectDir, err := ProjectDir(project)
	if err != nil {
		return "", "", err
	}
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		return "", "", fmt.Errorf("create project directory: %w", err)
	}

	cfg, _, err := ReadConfigIfExists(project)
	if err != nil {
		return "", "", err
	}
	specData, isRemote, err := loadSourceData(opts.Source)
	if err != nil {
		return "", "", err
	}
	specPath := filepath.Join(projectDir, "api.json")
	if err := os.WriteFile(specPath, specData, 0o644); err != nil {
		return "", "", fmt.Errorf("write api spec: %w", err)
	}
	if opts.BaseURL != "" {
		cfg.BaseURL = opts.BaseURL
	}
	if opts.AuthHeader != "" {
		cfg.AuthHeader = opts.AuthHeader
	}
	if opts.AuthToken != "" {
		cfg.AuthToken = opts.AuthToken
	}
	if opts.SpecType != "" {
		cfg.SpecType = opts.SpecType
	}
	if opts.UpdateURL != "" {
		cfg.UpdateURL = opts.UpdateURL
	}
	if isRemote {
		cfg.UpdateURL = opts.Source
	}
	configPath, err := WriteConfig(project, cfg)
	if err != nil {
		return "", "", err
	}
	return configPath, specPath, nil
}

func SyncProject(project string) (string, error) {
	cfg, _, err := ReadConfig(project)
	if err != nil {
		return "", err
	}
	if cfg.UpdateURL == "" {
		return "", fmt.Errorf("project updateUrl is empty; set it with `openapi config set -p %s updateUrl <url>`", project)
	}
	data, err := downloadSpec(cfg.UpdateURL)
	if err != nil {
		return "", err
	}
	projectDir, err := ProjectDir(project)
	if err != nil {
		return "", err
	}
	specPath := filepath.Join(projectDir, "api.json")
	if err := os.WriteFile(specPath, data, 0o644); err != nil {
		return "", fmt.Errorf("write api spec: %w", err)
	}
	return specPath, nil
}

func loadSourceData(source string) ([]byte, bool, error) {
	if source == "" {
		return nil, false, fmt.Errorf("source is required; pass a local file path or an http/https URL")
	}
	if isHTTPURL(source) {
		data, err := downloadSpec(source)
		if err != nil {
			return nil, true, err
		}
		return data, true, nil
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return nil, false, fmt.Errorf("read local spec file: %s; check that the file exists and is readable", err)
	}
	return data, false, nil
}

func downloadSpec(rawURL string) ([]byte, error) {
	resp, err := specHTTPClient.Get(rawURL)
	if err != nil {
		return nil, fmt.Errorf("download api spec: %s; check that the URL is reachable", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("download api spec failed: unexpected status %s from %s", resp.Status, rawURL)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read downloaded api spec: %s", err)
	}
	return data, nil
}

func isHTTPURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return parsed.Scheme == "http" || parsed.Scheme == "https"
}
