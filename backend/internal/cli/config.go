package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"testforge/internal/client"
)

// Config is what `testforge login` stores.
type Config struct {
	URL       string `json:"url"`
	APIKey    string `json:"apiKey"`
	Workspace string `json:"workspace"`
}

func (e *Env) configPath() (string, error) {
	dir := e.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home := e.Getenv("HOME")
		if home == "" {
			return "", errors.New("cannot find the config directory: set HOME or XDG_CONFIG_HOME")
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "testforge", "config.json"), nil
}

func (e *Env) loadConfig() (Config, error) {
	var c Config
	p, err := e.configPath()
	if err != nil {
		return c, err
	}
	data, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return c, fmt.Errorf("invalid config %s: %w", p, err)
	}
	return c, nil
}

// saveConfig writes the config readable only by the user, atomically.
func (e *Env) saveConfig(c Config) (string, error) {
	p, err := e.configPath()
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return "", err
	}
	data, _ := json.MarshalIndent(c, "", "  ")
	tmp, err := os.CreateTemp(dir, ".config-*.json")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return "", err
	}
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	return p, os.Rename(tmp.Name(), p)
}

func (e *Env) removeConfig() (bool, error) {
	p, err := e.configPath()
	if err != nil {
		return false, err
	}
	err = os.Remove(p)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

// target is the resolved API, key and workspace for one command.
type target struct {
	mu        sync.Mutex // guards workspace for concurrent MCP calls
	client    *client.Client
	workspace string
}

// resolve applies flags over env over the config file. The stored key is only
// used with the stored URL, so `--url` can never send it to another server.
func (e *Env) resolve(g *globals) (*target, error) {
	url := firstNonEmpty(g.url, e.Getenv("TESTFORGE_URL"))
	key := strings.TrimSpace(e.Getenv("TESTFORGE_API_KEY"))
	// With both from the environment (CI) the stored login is not needed.
	cfg, err := e.loadConfig()
	if err != nil && (url == "" || key == "") {
		return nil, err
	}
	if url == "" {
		url = cfg.URL
	}
	if url == "" {
		return nil, errors.New("not logged in: run `testforge login --url <api-url>` or set TESTFORGE_URL and TESTFORGE_API_KEY")
	}
	if key == "" {
		if !sameURL(url, cfg.URL) {
			return nil, fmt.Errorf("no API key for %s: set TESTFORGE_API_KEY or run `testforge login --url %s`", url, url)
		}
		key = cfg.APIKey
	}
	if key == "" {
		return nil, errors.New("no API key: run `testforge login` or set TESTFORGE_API_KEY")
	}
	c, err := client.New(url, key)
	if err != nil {
		return nil, err
	}
	c.UserAgent = "testforge-cli/" + Version
	t := &target{client: c, workspace: firstNonEmpty(g.workspace, e.Getenv("TESTFORGE_WORKSPACE"))}
	if t.workspace == "" && sameURL(url, cfg.URL) {
		t.workspace = cfg.Workspace
	}
	return t, nil
}

func sameURL(a, b string) bool {
	ua, err1 := client.ParseURL(a)
	ub, err2 := client.ParseURL(b)
	return err1 == nil && err2 == nil && ua.String() == ub.String()
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s = strings.TrimSpace(s); s != "" {
			return s
		}
	}
	return ""
}
