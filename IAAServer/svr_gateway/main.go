package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"common/applog"
	"common/config"
	"common/errorcode"
	cservice "common/service"

	"github.com/golang-jwt/jwt/v5"
)

const (
	gatewayConfigPath    = "config.json"
	gatewayConfigPathEnv = "IAA_GATEWAY_CONFIG_PATH"
)

type GatewayConfig struct {
	GatewayPort       int    `json:"gateway_port"`
	JWTSecret         string `json:"jwt_secret"`
	ProxyTimeoutMS    int64  `json:"proxy_timeout_ms"`
	LoginRoutePath    string `json:"login_route_path"`
	LoginTargetPath   string `json:"login_target_path"`
	RegisterPath      string `json:"register_path"`
	HeartbeatPath     string `json:"heartbeat_path"`
	LeaseTimeoutSec   int64  `json:"lease_timeout_sec"`
	SupersedeDelaySec int64  `json:"supersede_delay_sec"`
	GameStickyTTLSec  int64  `json:"game_sticky_ttl_sec"`
}

type APIResp struct {
	ErrMsg uint16 `json:"err_msg"`
}

type HealthResp struct {
	Status  string `json:"status"`
	Service string `json:"service"`
	ErrMsg  uint16 `json:"err_msg,omitempty"`
}

type GatewayServer struct {
	cfg       GatewayConfig
	registry  *cservice.Registry
	transport *http.Transport
	stickyMu  sync.Mutex
	sticky    *stickyStore
}

type stickyBinding struct {
	ServerID string `json:"server_id"`
}

func loadGatewayConfig(path string) (GatewayConfig, error) {
	var cfg GatewayConfig
	if err := config.LoadJSONConfig(path, &cfg); err != nil {
		return GatewayConfig{}, err
	}

	cfg.JWTSecret = strings.TrimSpace(cfg.JWTSecret)
	cfg.LoginRoutePath = normalizePathOrDefault(cfg.LoginRoutePath, "/login")
	cfg.LoginTargetPath = normalizePathOrDefault(cfg.LoginTargetPath, "/wxlogin")
	cfg.RegisterPath = normalizePathOrDefault(cfg.RegisterPath, "/register")
	cfg.HeartbeatPath = normalizePathOrDefault(cfg.HeartbeatPath, "/heartbeat")

	if err := validatePort("gateway_port", cfg.GatewayPort); err != nil {
		return GatewayConfig{}, err
	}
	if cfg.JWTSecret == "" {
		return GatewayConfig{}, errors.New("jwt_secret cannot be empty")
	}
	if cfg.ProxyTimeoutMS <= 0 {
		cfg.ProxyTimeoutMS = 10000
	}
	if cfg.LeaseTimeoutSec <= 0 {
		cfg.LeaseTimeoutSec = 10
	}
	if cfg.SupersedeDelaySec <= 0 {
		cfg.SupersedeDelaySec = 30
	}
	if cfg.GameStickyTTLSec <= 0 {
		cfg.GameStickyTTLSec = 600
	}

	return cfg, nil
}

func validatePort(name string, port int) error {
	if port <= 0 || port > 65535 {
		return fmt.Errorf("%s must be in range 1-65535", name)
	}
	return nil
}

func normalizePathOrDefault(path string, defaultPath string) string {
	p := strings.TrimSpace(path)
	if p == "" {
		p = defaultPath
	}
	p = "/" + strings.Trim(p, "/")
	if p == "//" || p == "/" {
		return "/"
	}
	return p
}

func configPathFromEnv(envName string, defaultPath string) string {
	if value := strings.TrimSpace(os.Getenv(envName)); value != "" {
		return value
	}
	return defaultPath
}

func newGatewayServer(cfg GatewayConfig) *GatewayServer {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          200,
		MaxIdleConnsPerHost:   100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: time.Duration(cfg.ProxyTimeoutMS) * time.Millisecond,
	}

	return &GatewayServer{
		cfg: cfg,
		registry: cservice.NewRegistry(
			time.Duration(cfg.LeaseTimeoutSec)*time.Second,
			time.Duration(cfg.SupersedeDelaySec)*time.Second,
		),
		transport: transport,
		sticky:    newStickyStore(time.Duration(cfg.GameStickyTTLSec) * time.Second),
	}
}

func (g *GatewayServer) Close() {
	if g.transport != nil {
		g.transport.CloseIdleConnections()
	}
	if g.sticky != nil {
		g.sticky.Close()
	}
}

func proxyErrorHandler(w http.ResponseWriter, _ *http.Request, err error) {
	applog.Errorf("proxy upstream failed: %v", err)
	writeJSON(w, http.StatusBadGateway, APIResp{ErrMsg: uint16(errorcode.UpstreamProxyFailed)})
}

func setCommonHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, PATCH")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
	w.Header().Set("Content-Type", "application/json;charset=utf-8")
}

func writeJSON(w http.ResponseWriter, status int, resp APIResp) {
	setCommonHeaders(w)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}

func writeHealthJSON(w http.ResponseWriter, status int, resp HealthResp) {
	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}

func extractBearerToken(r *http.Request) (string, error) {
	raw := strings.TrimSpace(r.Header.Get("Authorization"))
	if raw == "" {
		return "", errors.New("Authorization header is required")
	}

	const prefix = "Bearer "
	if len(raw) <= len(prefix) || !strings.EqualFold(raw[:len(prefix)], prefix) {
		return "", errors.New("Authorization must be Bearer token")
	}

	token := strings.TrimSpace(raw[len(prefix):])
	if token == "" {
		return "", errors.New("Bearer token cannot be empty")
	}
	return token, nil
}

func authErrorCode(err error) errorcode.Code {
	switch err.Error() {
	case "Authorization header is required":
		return errorcode.AuthMissingHeader
	case "Authorization must be Bearer token":
		return errorcode.AuthInvalidBearer
	case "Bearer token cannot be empty":
		return errorcode.AuthEmptyBearerToken
	case "openid claim is missing":
		return errorcode.AuthMissingOpenID
	default:
		return errorcode.AuthInvalidToken
	}
}

func parseOpenIDFromJWT(tokenString string, jwtSecret string) (string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(jwtSecret), nil
	})
	if err != nil {
		return "", err
	}
	if !token.Valid {
		return "", errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", errors.New("invalid token claims")
	}

	openid, ok := claims["openid"].(string)
	if !ok || strings.TrimSpace(openid) == "" {
		return "", errors.New("openid claim is missing")
	}
	return openid, nil
}

func (g *GatewayServer) isLoginRoute(path string) bool {
	normalized := normalizePathOrDefault(path, "/")
	if normalized == g.cfg.LoginRoutePath {
		return true
	}
	return normalized == "/wxlogin"
}

func (g *GatewayServer) handleHealthRoute(w http.ResponseWriter, r *http.Request) bool {
	switch normalizePathOrDefault(r.URL.Path, "/") {
	case "/livez", "/readyz":
		if r.Method != http.MethodGet {
			writeHealthJSON(w, http.StatusMethodNotAllowed, HealthResp{
				Status:  "error",
				Service: "svr_gateway",
				ErrMsg:  uint16(errorcode.InvalidMethod),
			})
			return true
		}

		writeHealthJSON(w, http.StatusOK, HealthResp{
			Status:  "ok",
			Service: "svr_gateway",
		})
		return true
	default:
		return false
	}
}

func (g *GatewayServer) registerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		setCommonHeaders(w)
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, APIResp{ErrMsg: uint16(errorcode.InvalidMethod)})
		return
	}

	var request cservice.Instance
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, APIResp{ErrMsg: uint16(errorcode.InvalidRequest)})
		return
	}

	instance, err := request.Normalized()
	if err != nil {
		writeJSON(w, http.StatusBadRequest, APIResp{ErrMsg: uint16(errorcode.InvalidRequest)})
		return
	}

	response, err := g.registry.Upsert(instance)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, APIResp{ErrMsg: uint16(errorcode.InvalidRequest)})
		return
	}

	if response.Replaced {
		applog.Infof("service replaced: type=%s id=%s addr=%s:%d lease=%s",
			instance.Type, instance.ID, instance.Host, instance.Port, response.LeaseID)
	} else {
		applog.Infof("service registered: type=%s id=%s addr=%s:%d lease=%s",
			instance.Type, instance.ID, instance.Host, instance.Port, response.LeaseID)
	}

	setCommonHeaders(w)
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}

func (g *GatewayServer) heartbeatHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		setCommonHeaders(w)
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, APIResp{ErrMsg: uint16(errorcode.InvalidMethod)})
		return
	}

	var request cservice.HeartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, APIResp{ErrMsg: uint16(errorcode.InvalidRequest)})
		return
	}

	response, err := g.registry.Heartbeat(request)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, APIResp{ErrMsg: uint16(errorcode.InvalidRequest)})
		return
	}

	setCommonHeaders(w)
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}

func (g *GatewayServer) selectGameInstance(openid string) (cservice.Instance, bool) {
	g.stickyMu.Lock()
	defer g.stickyMu.Unlock()

	if binding, ok, err := g.sticky.Get(openid); err != nil {
		applog.Errorf("load sticky binding failed: %v", err)
	} else if ok {
		if instance, healthy := g.registry.GetByTypeAndID(cservice.TypeGame, binding.ServerID); healthy {
			if err := g.sticky.Set(openid, stickyBinding{ServerID: instance.ID}); err != nil {
				applog.Errorf("refresh sticky binding failed: %v", err)
			}
			return instance, true
		}
		g.sticky.Delete(openid)
	}

	instance, ok := g.registry.RandomByType(cservice.TypeGame)
	if !ok {
		return cservice.Instance{}, false
	}

	// Sticky routing only needs a local TTL cache because each gateway instance owns its in-process bindings.
	// TODO: Consider consistent hashing if scale-out requires lower remap rates during game node changes.
	if err := g.sticky.Set(openid, stickyBinding{
		ServerID: instance.ID,
	}); err != nil {
		applog.Errorf("store sticky binding failed: %v", err)
	}
	return instance, true
}

func (g *GatewayServer) proxyToType(w http.ResponseWriter, r *http.Request, serviceType cservice.Type, targetPath string, openid string) {
	var (
		instance cservice.Instance
		ok       bool
	)

	if serviceType == cservice.TypeGame && strings.TrimSpace(openid) != "" {
		instance, ok = g.selectGameInstance(openid)
	} else {
		instance, ok = g.registry.RandomByType(serviceType)
	}
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, APIResp{ErrMsg: uint16(errorcode.UpstreamUnavailable)})
		return
	}

	targetURL, err := url.Parse(instance.BaseURL())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, APIResp{ErrMsg: uint16(errorcode.InternalError)})
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	proxy.Transport = g.transport
	proxy.ErrorHandler = proxyErrorHandler

	request := r.Clone(r.Context())
	if targetPath != "" {
		request.URL.Path = targetPath
		request.URL.RawPath = ""
	}
	if strings.TrimSpace(openid) != "" {
		request.Header.Del("Authorization")
		request.Header.Set("X-OpenID", openid)
	}

	setCommonHeaders(w)
	proxy.ServeHTTP(w, request)
}

func (g *GatewayServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if g.handleHealthRoute(w, r) {
		return
	}

	normalizedPath := normalizePathOrDefault(r.URL.Path, "/")
	switch normalizedPath {
	case g.cfg.RegisterPath:
		g.registerHandler(w, r)
		return
	case g.cfg.HeartbeatPath:
		g.heartbeatHandler(w, r)
		return
	}

	if r.Method == http.MethodOptions {
		setCommonHeaders(w)
		w.WriteHeader(http.StatusOK)
		return
	}

	if g.isLoginRoute(r.URL.Path) {
		g.proxyToType(w, r, cservice.TypeLogin, g.cfg.LoginTargetPath, "")
		return
	}

	token, err := extractBearerToken(r)
	if err != nil {
		applog.Errorf("auth failed: %v", err)
		writeJSON(w, http.StatusUnauthorized, APIResp{ErrMsg: uint16(authErrorCode(err))})
		return
	}

	openid, err := parseOpenIDFromJWT(token, g.cfg.JWTSecret)
	if err != nil {
		applog.Errorf("jwt verify failed: %v", err)
		writeJSON(w, http.StatusUnauthorized, APIResp{ErrMsg: uint16(authErrorCode(err))})
		return
	}

	g.proxyToType(w, r, cservice.TypeGame, "", openid)
}

func main() {
	if err := applog.Init("svr_gateway"); err != nil {
		fmt.Printf("init logger failed: %s\n", err.Error())
	}
	defer func() {
		if err := applog.Close(); err != nil {
			fmt.Printf("close logger failed: %s\n", err.Error())
		}
	}()
	defer applog.CatchPanic()

	cfg, err := loadGatewayConfig(configPathFromEnv(gatewayConfigPathEnv, gatewayConfigPath))
	if err != nil {
		applog.Errorf("load gateway config failed: %v", err)
		return
	}

	listenAddr := fmt.Sprintf(":%d", cfg.GatewayPort)
	server := newGatewayServer(cfg)
	defer server.Close()

	applog.Infof("svr_gateway start, will listen to: http://0.0.0.0%s", listenAddr)
	if err := http.ListenAndServe(listenAddr, server); err != nil {
		applog.Errorf("gateway init failed: %v", err)
	}
}
