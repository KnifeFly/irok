package kiroauth

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"aiclient2api/internal/config"
	"aiclient2api/internal/pool"
	kiroprovider "aiclient2api/internal/provider/kiro"
)

type Manager struct {
	cfg    config.Config
	pools  *pool.Store
	client *http.Client
	logger *slog.Logger
	mu     sync.Mutex
	states map[string]pendingSocial
}

type pendingSocial struct {
	Verifier string
	Provider string
	NodeName string
	Created  time.Time
}

type StartRequest struct {
	Method            string `json:"method"`
	Provider          string `json:"provider"`
	NodeName          string `json:"node_name"`
	Region            string `json:"region"`
	BuilderIDStartURL string `json:"builder_id_start_url"`
}

type StartResponse struct {
	AuthURL  string         `json:"auth_url"`
	AuthInfo map[string]any `json:"auth_info"`
}

type ImportRequest struct {
	Name        string          `json:"name"`
	Credentials json.RawMessage `json:"credentials"`
}

func NewManager(cfg config.Config, pools *pool.Store, logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	return &Manager{
		cfg:    cfg,
		pools:  pools,
		client: &http.Client{Timeout: 30 * time.Second},
		logger: logger,
		states: map[string]pendingSocial{},
	}
}

func (m *Manager) Start(ctx context.Context, req StartRequest) (StartResponse, error) {
	method := strings.ToLower(strings.TrimSpace(req.Method))
	if method == "" {
		method = strings.ToLower(strings.TrimSpace(req.Provider))
	}
	switch method {
	case "google", "github":
		return m.startSocial(method, req.NodeName)
	case "builder-id", "builder":
		return m.startBuilder(ctx, req)
	default:
		return StartResponse{}, fmt.Errorf("unsupported kiro auth method: %s", method)
	}
}

func (m *Manager) Callback(ctx context.Context, values url.Values) (pool.Node, error) {
	code := values.Get("code")
	state := values.Get("state")
	if errParam := values.Get("error"); errParam != "" {
		return pool.Node{}, errors.New(errParam)
	}
	if code == "" || state == "" {
		return pool.Node{}, errors.New("code and state are required")
	}

	m.mu.Lock()
	pending, ok := m.states[state]
	if ok {
		delete(m.states, state)
	}
	m.mu.Unlock()
	if !ok {
		return pool.Node{}, errors.New("oauth state not found or expired")
	}
	redirectURI := m.cfg.Server.PublicURL + m.cfg.Kiro.CallbackPath
	payload, _ := json.Marshal(map[string]any{
		"code":          code,
		"code_verifier": pending.Verifier,
		"redirect_uri":  redirectURI,
	})
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(m.cfg.Kiro.AuthServiceURL, "/")+"/oauth/token", bytes.NewReader(payload))
	if err != nil {
		return pool.Node{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "AIClient-2-API/2.0")
	resp, err := m.client.Do(httpReq)
	if err != nil {
		return pool.Node{}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return pool.Node{}, fmt.Errorf("token exchange status %d: %s", resp.StatusCode, string(raw))
	}
	var token struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
		ProfileArn   string `json:"profileArn"`
		ExpiresIn    int    `json:"expiresIn"`
	}
	if err := json.Unmarshal(raw, &token); err != nil {
		return pool.Node{}, err
	}
	if token.AccessToken == "" || token.RefreshToken == "" {
		return pool.Node{}, errors.New("oauth response missing tokens")
	}
	if token.ExpiresIn <= 0 {
		token.ExpiresIn = 3600
	}
	creds := kiroprovider.Credentials{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		ProfileArn:   token.ProfileArn,
		ExpiresAt:    time.Now().UTC().Add(time.Duration(token.ExpiresIn) * time.Second).Format(time.RFC3339),
		AuthMethod:   "social",
		Region:       m.cfg.Kiro.DefaultRegion,
	}
	name := pending.NodeName
	if name == "" {
		name = "Kiro " + strings.Title(pending.Provider)
	}
	return m.saveNode(name, creds)
}

func (m *Manager) Import(ctx context.Context, req ImportRequest) ([]pool.Node, error) {
	if len(req.Credentials) == 0 {
		return nil, errors.New("credentials is required")
	}
	var list []kiroprovider.Credentials
	if req.Credentials[0] == '[' {
		if err := json.Unmarshal(req.Credentials, &list); err != nil {
			return nil, err
		}
	} else {
		var cred kiroprovider.Credentials
		if err := json.Unmarshal(req.Credentials, &cred); err != nil {
			return nil, err
		}
		list = []kiroprovider.Credentials{cred}
	}
	nodes := make([]pool.Node, 0, len(list))
	for i, cred := range list {
		if cred.AccessToken == "" || cred.RefreshToken == "" {
			return nil, errors.New("credentials require accessToken and refreshToken")
		}
		if cred.AuthMethod == "" {
			if cred.ClientID != "" && cred.ClientSecret != "" {
				cred.AuthMethod = "builder-id"
			} else {
				cred.AuthMethod = "social"
			}
		}
		if cred.Region == "" {
			cred.Region = m.cfg.Kiro.DefaultRegion
		}
		if cred.IDCRegion == "" {
			cred.IDCRegion = m.cfg.Kiro.DefaultIDCRegion
		}
		name := req.Name
		if len(list) > 1 || name == "" {
			name = fmt.Sprintf("Kiro Imported %d", i+1)
		}
		node, err := m.saveNode(name, cred)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

func (m *Manager) startSocial(provider string, nodeName string) (StartResponse, error) {
	verifier := randomURLToken(32)
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	state := randomURLToken(24)

	m.mu.Lock()
	m.states[state] = pendingSocial{
		Verifier: verifier,
		Provider: provider,
		NodeName: nodeName,
		Created:  time.Now(),
	}
	m.mu.Unlock()

	idp := "Google"
	if provider == "github" {
		idp = "Github"
	}
	redirectURI := m.cfg.Server.PublicURL + m.cfg.Kiro.CallbackPath
	authURL := strings.TrimRight(m.cfg.Kiro.AuthServiceURL, "/") + "/login?" + url.Values{
		"idp":                   []string{idp},
		"redirect_uri":          []string{redirectURI},
		"code_challenge":        []string{challenge},
		"code_challenge_method": []string{"S256"},
		"state":                 []string{state},
	}.Encode()
	return StartResponse{
		AuthURL: authURL,
		AuthInfo: map[string]any{
			"method":       provider,
			"state":        state,
			"redirect_uri": redirectURI,
		},
	}, nil
}

func (m *Manager) startBuilder(ctx context.Context, req StartRequest) (StartResponse, error) {
	region := req.Region
	if region == "" {
		region = m.cfg.Kiro.DefaultIDCRegion
	}
	startURL := req.BuilderIDStartURL
	if startURL == "" {
		startURL = "https://view.awsapps.com/start"
	}
	base := strings.ReplaceAll(m.cfg.Kiro.OidcURLTemplate, "{{region}}", region)
	regPayload, _ := json.Marshal(map[string]any{
		"clientName": "Kiro IDE",
		"clientType": "public",
		"scopes": []string{
			"codewhisperer:completions",
			"codewhisperer:analysis",
			"codewhisperer:conversations",
		},
	})
	regData, err := m.postJSON(ctx, base+"/client/register", regPayload)
	if err != nil {
		return StartResponse{}, err
	}
	clientID, _ := regData["clientId"].(string)
	clientSecret, _ := regData["clientSecret"].(string)
	if clientID == "" || clientSecret == "" {
		return StartResponse{}, errors.New("builder-id client registration missing client credentials")
	}
	authPayload, _ := json.Marshal(map[string]any{
		"clientId":     clientID,
		"clientSecret": clientSecret,
		"startUrl":     startURL,
	})
	device, err := m.postJSON(ctx, base+"/device_authorization", authPayload)
	if err != nil {
		return StartResponse{}, err
	}
	deviceCode, _ := device["deviceCode"].(string)
	if deviceCode == "" {
		return StartResponse{}, errors.New("builder-id device authorization missing deviceCode")
	}
	interval := intFromAny(device["interval"], 5)
	expiresIn := intFromAny(device["expiresIn"], 300)
	nodeName := req.NodeName
	go m.pollBuilder(context.Background(), base, clientID, clientSecret, deviceCode, interval, expiresIn, nodeName, region)

	authURL, _ := device["verificationUriComplete"].(string)
	return StartResponse{AuthURL: authURL, AuthInfo: device}, nil
}

func (m *Manager) pollBuilder(ctx context.Context, baseURL, clientID, clientSecret, deviceCode string, interval, expiresIn int, nodeName string, region string) {
	deadline := time.Now().Add(time.Duration(expiresIn) * time.Second)
	for time.Now().Before(deadline) {
		payload, _ := json.Marshal(map[string]any{
			"clientId":     clientID,
			"clientSecret": clientSecret,
			"deviceCode":   deviceCode,
			"grantType":    "urn:ietf:params:oauth:grant-type:device_code",
		})
		data, err := m.postJSON(ctx, baseURL+"/token", payload)
		if err == nil {
			accessToken, _ := data["accessToken"].(string)
			refreshToken, _ := data["refreshToken"].(string)
			if accessToken != "" && refreshToken != "" {
				expires := intFromAny(data["expiresIn"], 3600)
				if nodeName == "" {
					nodeName = "Kiro Builder ID"
				}
				_, saveErr := m.saveNode(nodeName, kiroprovider.Credentials{
					AccessToken:  accessToken,
					RefreshToken: refreshToken,
					ClientID:     clientID,
					ClientSecret: clientSecret,
					AuthMethod:   "builder-id",
					IDCRegion:    region,
					ExpiresAt:    time.Now().UTC().Add(time.Duration(expires) * time.Second).Format(time.RFC3339),
				})
				if saveErr != nil {
					m.logger.Error("save builder-id credentials failed", "error", saveErr)
				}
				return
			}
		}
		if err != nil && !strings.Contains(err.Error(), "authorization_pending") && !strings.Contains(err.Error(), "slow_down") {
			m.logger.Warn("builder-id token polling failed", "error", err)
		}
		time.Sleep(time.Duration(interval) * time.Second)
	}
	m.logger.Warn("builder-id token polling expired")
}

func (m *Manager) postJSON(ctx context.Context, endpoint string, payload []byte) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "KiroIDE")
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var data map[string]any
	_ = json.Unmarshal(raw, &data)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if code, _ := data["error"].(string); code != "" {
			return nil, errors.New(code)
		}
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(raw))
	}
	return data, nil
}

func (m *Manager) saveNode(name string, creds kiroprovider.Credentials) (pool.Node, error) {
	id := randomID()
	path := filepath.Join(m.cfg.Files.CredentialsDir, id+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return pool.Node{}, err
	}
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return pool.Node{}, err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return pool.Node{}, err
	}
	return m.pools.Upsert(pool.Node{
		ID:             id,
		Name:           name,
		CredentialPath: path,
		Enabled:        true,
		Healthy:        true,
	})
}

func intFromAny(value any, fallback int) int {
	switch v := value.(type) {
	case float64:
		return int(v)
	case int:
		return v
	default:
		return fallback
	}
}

func randomURLToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func randomID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return hex.EncodeToString(b[:4]) + "-" + hex.EncodeToString(b[4:6]) + "-" + hex.EncodeToString(b[6:8]) + "-" + hex.EncodeToString(b[8:10]) + "-" + hex.EncodeToString(b[10:])
}
