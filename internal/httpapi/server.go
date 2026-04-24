package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"aiclient2api/internal/assets"
	kiroauth "aiclient2api/internal/auth/kiro"
	"aiclient2api/internal/config"
	"aiclient2api/internal/logtail"
	"aiclient2api/internal/pool"
	"aiclient2api/internal/prompt"
	"aiclient2api/internal/provider"
)

type Server struct {
	cfg       config.Config
	pools     *pool.Store
	prompts   *prompt.Store
	provider  provider.Interface
	auth      *kiroauth.Manager
	logger    *slog.Logger
	startedAt time.Time
	requests  atomic.Int64
	failures  atomic.Int64
	static    http.Handler
}

func New(cfg config.Config, pools *pool.Store, prompts *prompt.Store, provider provider.Interface, auth *kiroauth.Manager, logger *slog.Logger) (*Server, error) {
	dist, err := fs.Sub(assets.Files, "dist")
	if err != nil {
		return nil, err
	}
	return &Server{
		cfg:       cfg,
		pools:     pools,
		prompts:   prompts,
		provider:  provider,
		auth:      auth,
		logger:    logger,
		startedAt: time.Now().UTC(),
		static:    http.FileServer(http.FS(dist)),
	}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handle)
	return mux
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	s.setCommonHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	s.requests.Add(1)
	if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/v1/") {
		if !s.authorized(r) {
			s.failures.Add(1)
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
	}

	var err error
	switch {
	case r.URL.Path == "/health":
		s.handleHealth(w, r)
	case r.URL.Path == "/v1/models" && r.Method == http.MethodGet:
		err = s.handleModels(w, r)
	case r.URL.Path == "/v1/messages" && r.Method == http.MethodPost:
		err = s.handleMessages(w, r)
	case r.URL.Path == "/v1/messages/count_tokens" && r.Method == http.MethodPost:
		err = s.handleCountTokens(w, r)
	case r.URL.Path == "/api/status" && r.Method == http.MethodGet:
		err = s.handleStatus(w, r)
	case r.URL.Path == "/api/logs/tail" && r.Method == http.MethodGet:
		err = s.handleLogTail(w, r)
	case r.URL.Path == "/api/prompts" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, s.prompts.List())
	case r.URL.Path == "/api/prompts" && r.Method == http.MethodPut:
		err = s.handlePutPrompts(w, r)
	case r.URL.Path == "/api/pools/kiro/nodes":
		err = s.handleNodes(w, r)
	case strings.HasPrefix(r.URL.Path, "/api/pools/kiro/nodes/"):
		err = s.handleNode(w, r)
	case r.URL.Path == "/api/kiro/oauth/start" && r.Method == http.MethodPost:
		err = s.handleOAuthStart(w, r)
	case r.URL.Path == s.cfg.Kiro.CallbackPath && r.Method == http.MethodGet:
		err = s.handleOAuthCallback(w, r)
	case r.URL.Path == "/api/kiro/credentials/import" && r.Method == http.MethodPost:
		err = s.handleCredentialImport(w, r)
	default:
		s.serveStatic(w, r)
	}
	if err != nil {
		s.failures.Add(1)
		s.logger.Error("request failed", "method", r.Method, "path", r.URL.Path, "error", err)
		status := http.StatusInternalServerError
		if errors.Is(err, os.ErrNotExist) {
			status = http.StatusNotFound
		}
		writeError(w, status, err.Error())
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeRawJSON(w, http.StatusOK, map[string]any{
		"status":     "healthy",
		"provider":   "kiro",
		"started_at": s.startedAt,
		"timestamp":  time.Now().UTC(),
	})
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) error {
	models, err := s.provider.ListModels(r.Context())
	if err != nil {
		return err
	}
	writeRawJSON(w, http.StatusOK, map[string]any{"data": models})
	return nil
}

func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) error {
	var raw provider.RawMessageRequest
	if err := decodeJSON(r, &raw); err != nil {
		return err
	}
	systemText, err := provider.NormalizeSystem(raw.System)
	if err != nil {
		return err
	}
	systemText = s.prompts.Apply(raw.Model, systemText)
	req := provider.FromRaw(raw, systemText)
	if req.Stream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		flusher, _ := w.(http.Flusher)
		return s.provider.CreateMessageStream(r.Context(), req, func(event provider.StreamEvent) error {
			data, err := json.Marshal(event)
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, data); err != nil {
				return err
			}
			if flusher != nil {
				flusher.Flush()
			}
			return nil
		})
	}
	resp, err := s.provider.CreateMessage(r.Context(), req)
	if err != nil {
		return err
	}
	writeRawJSON(w, http.StatusOK, resp)
	return nil
}

func (s *Server) handleCountTokens(w http.ResponseWriter, r *http.Request) error {
	var raw provider.RawMessageRequest
	if err := decodeJSON(r, &raw); err != nil {
		return err
	}
	systemText, err := provider.NormalizeSystem(raw.System)
	if err != nil {
		return err
	}
	systemText = s.prompts.Apply(raw.Model, systemText)
	count, err := s.provider.CountTokens(r.Context(), provider.FromRaw(raw, systemText))
	if err != nil {
		return err
	}
	writeRawJSON(w, http.StatusOK, count)
	return nil
}

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) error {
	writeJSON(w, http.StatusOK, map[string]any{
		"started_at":      s.startedAt,
		"uptime_seconds":  int(time.Since(s.startedAt).Seconds()),
		"requests_total":  s.requests.Load(),
		"failures_total":  s.failures.Load(),
		"nodes_total":     len(s.pools.List()),
		"prompts_total":   len(s.prompts.List()),
		"models":          modelsForStatus(),
		"config":          s.cfg,
		"config_warnings": configWarnings(s.cfg),
	})
	return nil
}

func (s *Server) handleLogTail(w http.ResponseWriter, r *http.Request) error {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	lines, err := logtail.Lines(filepath.Join(s.cfg.Logging.Dir, "app.log"), limit)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"lines": lines})
	return nil
}

func (s *Server) handlePutPrompts(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		Prompts []prompt.Rule `json:"prompts"`
	}
	if err := decodeJSON(r, &body); err != nil {
		var direct []prompt.Rule
		if err2 := decodeBodyAgain(r, &direct); err2 != nil {
			return err
		}
		body.Prompts = direct
	}
	rules, err := s.prompts.Replace(body.Prompts)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, rules)
	return nil
}

func (s *Server) handleNodes(w http.ResponseWriter, r *http.Request) error {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.pools.List())
		return nil
	case http.MethodPost:
		var node pool.Node
		if err := decodeJSON(r, &node); err != nil {
			return err
		}
		out, err := s.pools.Upsert(node)
		if err != nil {
			return err
		}
		writeJSON(w, http.StatusOK, out)
		return nil
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return nil
	}
}

func (s *Server) handleNode(w http.ResponseWriter, r *http.Request) error {
	rest := strings.TrimPrefix(r.URL.Path, "/api/pools/kiro/nodes/")
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		return os.ErrNotExist
	}
	id := parts[0]
	if len(parts) == 2 && parts[1] == "refresh" && r.Method == http.MethodPost {
		if err := s.provider.RefreshCredential(r.Context(), id); err != nil {
			return err
		}
		writeJSON(w, http.StatusOK, map[string]any{"refreshed": true})
		return nil
	}
	switch r.Method {
	case http.MethodPut:
		var node pool.Node
		if err := decodeJSON(r, &node); err != nil {
			return err
		}
		node.ID = id
		out, err := s.pools.Upsert(node)
		if err != nil {
			return err
		}
		writeJSON(w, http.StatusOK, out)
	case http.MethodDelete:
		if err := s.pools.Delete(id); err != nil {
			return err
		}
		writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
	return nil
}

func (s *Server) handleOAuthStart(w http.ResponseWriter, r *http.Request) error {
	var req kiroauth.StartRequest
	if err := decodeJSON(r, &req); err != nil {
		return err
	}
	resp, err := s.auth.Start(r.Context(), req)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, resp)
	return nil
}

func (s *Server) handleOAuthCallback(w http.ResponseWriter, r *http.Request) error {
	node, err := s.auth.Callback(r.Context(), r.URL.Query())
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprintf(w, "<!doctype html><title>Kiro OAuth</title><h1>Authorization failed</h1><p>%s</p>", err.Error())
		return nil
	}
	_, _ = fmt.Fprintf(w, "<!doctype html><title>Kiro OAuth</title><h1>Authorization complete</h1><p>Saved node %s. You can close this page.</p>", node.Name)
	return nil
}

func (s *Server) handleCredentialImport(w http.ResponseWriter, r *http.Request) error {
	var req kiroauth.ImportRequest
	if err := decodeJSON(r, &req); err != nil {
		return err
	}
	nodes, err := s.auth.Import(r.Context(), req)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, nodes)
	return nil
}

func (s *Server) serveStatic(w http.ResponseWriter, r *http.Request) {
	clean := path.Clean(r.URL.Path)
	if clean == "." || clean == "/" {
		r.URL.Path = "/"
		s.static.ServeHTTP(w, r)
		return
	}
	filePath := strings.TrimPrefix(clean, "/")
	if _, err := assets.Files.Open("dist/" + filePath); err == nil {
		s.static.ServeHTTP(w, r)
		return
	}
	r.URL.Path = "/"
	s.static.ServeHTTP(w, r)
}

func (s *Server) authorized(r *http.Request) bool {
	if s.cfg.Server.AdminAPIKey == "" {
		return true
	}
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") && strings.TrimSpace(auth[7:]) == s.cfg.Server.AdminAPIKey {
		return true
	}
	for _, key := range []string{"x-api-key", "x-admin-api-key", "x-goog-api-key"} {
		if r.Header.Get(key) == s.cfg.Server.AdminAPIKey {
			return true
		}
	}
	return false
}

func (s *Server) setCommonHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, x-api-key, x-admin-api-key, x-goog-api-key")
}

func decodeJSON(r *http.Request, out any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(r.Body, 20<<20))
	decoder.UseNumber()
	return decoder.Decode(out)
}

func decodeBodyAgain(_ *http.Request, _ any) error {
	return errors.New("invalid request body")
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	writeRawJSON(w, status, map[string]any{"success": true, "data": data})
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeRawJSON(w, status, map[string]any{"success": false, "message": message})
}

func writeRawJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func modelsForStatus() []string {
	return []string{
		"claude-haiku-4-5",
		"claude-sonnet-4-5",
		"claude-opus-4-5",
		"claude-sonnet-4-6",
		"claude-opus-4-6",
	}
}

func configWarnings(cfg config.Config) []string {
	var warnings []string
	if cfg.Server.AdminAPIKey == "change-me" {
		warnings = append(warnings, "admin_api_key is still the default value")
	}
	return warnings
}
