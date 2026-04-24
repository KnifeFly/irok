package prompt

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"aiclient2api/internal/config"
	"github.com/pelletier/go-toml/v2"
)

const (
	ModePrepend  = "prepend"
	ModeAppend   = "append"
	ModeOverride = "override"
	ModeOff      = "off"
)

type File struct {
	Prompts []Rule `toml:"prompts" json:"prompts"`
}

type Rule struct {
	Model     string `toml:"model" json:"model"`
	Enabled   bool   `toml:"enabled" json:"enabled"`
	Mode      string `toml:"mode" json:"mode"`
	Content   string `toml:"content" json:"content"`
	Note      string `toml:"note,omitempty" json:"note,omitempty"`
	UpdatedAt string `toml:"updated_at,omitempty" json:"updated_at,omitempty"`
}

type Store struct {
	mu   sync.RWMutex
	path string
	data File
}

func NewStore(path string) (*Store, error) {
	s := &Store{path: path}
	if err := s.Load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			s.data = File{Prompts: []Rule{{
				Model:     "*",
				Enabled:   false,
				Mode:      ModePrepend,
				Content:   "",
				Note:      "Default prompt rule",
				UpdatedAt: time.Now().UTC().Format(time.RFC3339),
			}}}
			return config.AtomicWriteTOML(s.path, s.data)
		}
		return err
	}
	var file File
	if err := toml.Unmarshal(data, &file); err != nil {
		return fmt.Errorf("parse prompts %s: %w", s.path, err)
	}
	for i := range file.Prompts {
		normalizeRule(&file.Prompts[i])
	}
	s.data = file
	return nil
}

func (s *Store) List() []Rule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rules := append([]Rule(nil), s.data.Prompts...)
	sort.SliceStable(rules, func(i, j int) bool {
		if rules[i].Model == "*" {
			return true
		}
		if rules[j].Model == "*" {
			return false
		}
		return rules[i].Model < rules[j].Model
	})
	return rules
}

func (s *Store) Replace(rules []Rule) ([]Rule, error) {
	for i := range rules {
		normalizeRule(&rules[i])
	}
	if !hasDefault(rules) {
		rules = append([]Rule{{
			Model:     "*",
			Enabled:   false,
			Mode:      ModePrepend,
			Content:   "",
			Note:      "Default prompt rule",
			UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		}}, rules...)
	}

	s.mu.Lock()
	s.data = File{Prompts: rules}
	data := s.data
	s.mu.Unlock()

	if err := config.AtomicWriteTOML(s.path, data); err != nil {
		return nil, err
	}
	return s.List(), nil
}

func (s *Store) Apply(model string, requestSystem string) string {
	rule, ok := s.match(model)
	if !ok || !rule.Enabled || strings.TrimSpace(rule.Content) == "" || rule.Mode == ModeOff {
		return requestSystem
	}

	content := strings.TrimSpace(rule.Content)
	base := strings.TrimSpace(requestSystem)
	switch rule.Mode {
	case ModeOverride:
		return content
	case ModeAppend:
		if base == "" {
			return content
		}
		return base + "\n\n" + content
	case ModePrepend, "":
		if base == "" {
			return content
		}
		return content + "\n\n" + base
	default:
		if base == "" {
			return content
		}
		return content + "\n\n" + base
	}
}

func (s *Store) match(model string) (Rule, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var fallback *Rule
	for i := range s.data.Prompts {
		rule := s.data.Prompts[i]
		if rule.Model == "*" {
			r := rule
			fallback = &r
			continue
		}
		if rule.Model == model {
			return rule, true
		}
	}
	if fallback != nil {
		return *fallback, true
	}
	return Rule{}, false
}

func normalizeRule(rule *Rule) {
	rule.Model = strings.TrimSpace(rule.Model)
	if rule.Model == "" {
		rule.Model = "*"
	}
	rule.Mode = strings.TrimSpace(strings.ToLower(rule.Mode))
	if rule.Mode == "" {
		rule.Mode = ModePrepend
	}
	switch rule.Mode {
	case ModePrepend, ModeAppend, ModeOverride, ModeOff:
	default:
		rule.Mode = ModePrepend
	}
	if rule.UpdatedAt == "" {
		rule.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
}

func hasDefault(rules []Rule) bool {
	for _, rule := range rules {
		if strings.TrimSpace(rule.Model) == "*" {
			return true
		}
	}
	return false
}
