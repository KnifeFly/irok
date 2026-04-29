package pool

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pelletier/go-toml/v2"
	"orik/internal/config"
)

type File struct {
	Kiro KiroPool `toml:"kiro" json:"kiro"`
}

type KiroPool struct {
	Nodes []Node `toml:"nodes" json:"nodes"`
}

type Node struct {
	ID             string `toml:"id" json:"id"`
	Name           string `toml:"name" json:"name"`
	CredentialPath string `toml:"credential_path" json:"credential_path"`
	Enabled        bool   `toml:"enabled" json:"enabled"`
	Healthy        bool   `toml:"healthy" json:"healthy"`
	FailureCount   int    `toml:"failure_count" json:"failure_count"`
	LastError      string `toml:"last_error,omitempty" json:"last_error,omitempty"`
	LastErrorAt    string `toml:"last_error_at,omitempty" json:"last_error_at,omitempty"`
	RecoveryAt     string `toml:"recovery_at,omitempty" json:"recovery_at,omitempty"`
	LastUsedAt     string `toml:"last_used_at,omitempty" json:"last_used_at,omitempty"`
	UsageCount     int    `toml:"usage_count" json:"usage_count"`
	Note           string `toml:"note,omitempty" json:"note,omitempty"`
	CreatedAt      string `toml:"created_at,omitempty" json:"created_at,omitempty"`
	UpdatedAt      string `toml:"updated_at,omitempty" json:"updated_at,omitempty"`
}

type Store struct {
	mu      sync.RWMutex
	path    string
	data    File
	cursor  int
	maxFail int
}

func NewStore(path string) (*Store, error) {
	s := &Store{path: path, maxFail: 3}
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
			s.data = File{Kiro: KiroPool{Nodes: []Node{}}}
			return config.AtomicWriteTOML(s.path, s.data)
		}
		return err
	}
	var file File
	if err := toml.Unmarshal(data, &file); err != nil {
		return fmt.Errorf("parse pools %s: %w", s.path, err)
	}
	for i := range file.Kiro.Nodes {
		normalizeNode(&file.Kiro.Nodes[i])
	}
	s.data = file
	return nil
}

func (s *Store) Save() error {
	s.mu.RLock()
	data := s.data
	s.mu.RUnlock()
	return config.AtomicWriteTOML(s.path, data)
}

func (s *Store) List() []Node {
	s.mu.RLock()
	defer s.mu.RUnlock()
	nodes := append([]Node(nil), s.data.Kiro.Nodes...)
	sort.SliceStable(nodes, func(i, j int) bool {
		return strings.ToLower(nodes[i].Name) < strings.ToLower(nodes[j].Name)
	})
	return nodes
}

func (s *Store) Get(id string) (Node, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, node := range s.data.Kiro.Nodes {
		if node.ID == id {
			return node, true
		}
	}
	return Node{}, false
}

func (s *Store) Upsert(input Node) (Node, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC().Format(time.RFC3339)
	if input.ID == "" {
		input.ID = newID()
	}
	if input.Name == "" {
		input.Name = "Kiro Node " + input.ID[:8]
	}
	if input.CredentialPath == "" {
		return Node{}, errors.New("credential_path is required")
	}
	if input.CreatedAt == "" {
		input.CreatedAt = now
	}
	input.UpdatedAt = now
	if !input.Enabled && !hasExisting(s.data.Kiro.Nodes, input.ID) {
		input.Enabled = true
	}
	if !input.Healthy && input.FailureCount == 0 && input.LastError == "" {
		input.Healthy = true
	}
	normalizeNode(&input)

	for i, node := range s.data.Kiro.Nodes {
		if node.ID == input.ID {
			if input.CreatedAt == "" {
				input.CreatedAt = node.CreatedAt
			}
			s.data.Kiro.Nodes[i] = input
			return input, config.AtomicWriteTOML(s.path, s.data)
		}
	}
	s.data.Kiro.Nodes = append(s.data.Kiro.Nodes, input)
	return input, config.AtomicWriteTOML(s.path, s.data)
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	nodes := s.data.Kiro.Nodes[:0]
	found := false
	for _, node := range s.data.Kiro.Nodes {
		if node.ID == id {
			found = true
			continue
		}
		nodes = append(nodes, node)
	}
	if !found {
		return os.ErrNotExist
	}
	s.data.Kiro.Nodes = nodes
	return config.AtomicWriteTOML(s.path, s.data)
}

func (s *Store) Select() (Node, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.data.Kiro.Nodes) == 0 {
		return Node{}, errors.New("no kiro nodes configured")
	}
	now := time.Now().UTC()
	candidates := make([]int, 0, len(s.data.Kiro.Nodes))
	for i := range s.data.Kiro.Nodes {
		node := &s.data.Kiro.Nodes[i]
		if !node.Enabled || !node.Healthy {
			continue
		}
		if node.RecoveryAt != "" {
			recovery, err := time.Parse(time.RFC3339, node.RecoveryAt)
			if err == nil && recovery.After(now) {
				continue
			}
			node.RecoveryAt = ""
		}
		candidates = append(candidates, i)
	}
	if len(candidates) == 0 {
		return Node{}, errors.New("no healthy kiro nodes available")
	}
	s.cursor = (s.cursor + 1) % len(candidates)
	node := &s.data.Kiro.Nodes[candidates[s.cursor]]
	node.LastUsedAt = now.Format(time.RFC3339)
	node.UsageCount++
	node.UpdatedAt = node.LastUsedAt
	_ = config.AtomicWriteTOML(s.path, s.data)
	return *node, nil
}

func (s *Store) MarkSuccess(id string) {
	s.updateStatus(id, func(node *Node) {
		node.Healthy = true
		node.FailureCount = 0
		node.LastError = ""
		node.LastErrorAt = ""
		node.RecoveryAt = ""
	})
}

func (s *Store) MarkFailure(id string, reason string, recovery *time.Time) {
	s.updateStatus(id, func(node *Node) {
		node.FailureCount++
		node.LastError = reason
		node.LastErrorAt = time.Now().UTC().Format(time.RFC3339)
		if recovery != nil {
			node.Healthy = false
			node.RecoveryAt = recovery.UTC().Format(time.RFC3339)
			return
		}
		if node.FailureCount >= s.maxFail {
			node.Healthy = false
		}
	})
}

func (s *Store) SetEnabled(id string, enabled bool) (Node, error) {
	var out Node
	err := s.updateStatus(id, func(node *Node) {
		node.Enabled = enabled
		if enabled && node.LastError == "" {
			node.Healthy = true
		}
		out = *node
	})
	return out, err
}

func (s *Store) updateStatus(id string, fn func(*Node)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.data.Kiro.Nodes {
		if s.data.Kiro.Nodes[i].ID == id {
			fn(&s.data.Kiro.Nodes[i])
			s.data.Kiro.Nodes[i].UpdatedAt = time.Now().UTC().Format(time.RFC3339)
			return config.AtomicWriteTOML(s.path, s.data)
		}
	}
	return os.ErrNotExist
}

func normalizeNode(node *Node) {
	if node.ID == "" {
		node.ID = newID()
	}
	if node.Name == "" {
		node.Name = "Kiro Node " + node.ID[:8]
	}
	if !node.Enabled && node.CreatedAt == "" {
		node.Enabled = true
	}
	if !node.Healthy && node.FailureCount == 0 && node.LastError == "" {
		node.Healthy = true
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if node.CreatedAt == "" {
		node.CreatedAt = now
	}
	if node.UpdatedAt == "" {
		node.UpdatedAt = node.CreatedAt
	}
}

func hasExisting(nodes []Node, id string) bool {
	for _, node := range nodes {
		if node.ID == id {
			return true
		}
	}
	return false
}

func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return hex.EncodeToString(b[:4]) + "-" + hex.EncodeToString(b[4:6]) + "-" + hex.EncodeToString(b[6:8]) + "-" + hex.EncodeToString(b[8:10]) + "-" + hex.EncodeToString(b[10:])
}
