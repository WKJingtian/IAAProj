package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"common/applog"
	"common/config"

	"github.com/golang-jwt/jwt/v5"
)

const gatewayConfigPath = "config.json"

type GatewayConfig struct {
	GatewayPort     int    `json:"gateway_port"`
	LoginHost       string `json:"login_host"`
	LoginPort       int    `json:"login_port"`
	GameHost        string `json:"game_host"`
	GamePort        int    `json:"game_port"`
	JWTSecret       string `json:"jwt_secret"`
	ProxyTimeoutMS  int64  `json:"proxy_timeout_ms"`
	LoginRoutePath  string `json:"login_route_path"`
	LoginTargetPath string `json:"login_target_path"`
}

type APIResp struct {
	ErrMsg string `json:"errMsg"`
}

type GatewayServer struct {
	cfg        GatewayConfig
	loginProxy *httputil.ReverseProxy
	gameProxy  *httputil.ReverseProxy
}

func loadGatewayConfig(path string) (GatewayConfig, error) {
	var cfg GatewayConfig
	if err := config.LoadJSONConfig(path, &cfg); err != nil {
		return GatewayConfig{}, err
	}

	cfg.LoginHost = strings.TrimSpace(cfg.LoginHost)
	cfg.GameHost = strings.TrimSpace(cfg.GameHost)
	cfg.JWTSecret = strings.TrimSpace(cfg.JWTSecret)
	cfg.LoginRoutePath = normalizePathOrDefault(cfg.LoginRoutePath, "/login")
	cfg.LoginTargetPath = normalizePathOrDefault(cfg.LoginTargetPath, "/wxlogin")

	if err := validatePort("gateway_port", cfg.GatewayPort); err != nil {
		return GatewayConfig{}, err
	}
	if err := validatePort("login_port", cfg.LoginPort); err != nil {
		return GatewayConfig{}, err
	}
	if err := validatePort("game_port", cfg.GamePort); err != nil {
		return GatewayConfig{}, err
	}
	if cfg.LoginHost == "" {
		return GatewayConfig{}, errors.New("login_host cannot be empty")
	}
	if cfg.GameHost == "" {
		return GatewayConfig{}, errors.New("game_host cannot be empty")
	}
	if cfg.JWTSecret == "" {
		return GatewayConfig{}, errors.New("jwt_secret cannot be empty")
	}
	if cfg.ProxyTimeoutMS <= 0 {
		cfg.ProxyTimeoutMS = 10000
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

func newGatewayServer(cfg GatewayConfig) (*GatewayServer, error) {
	loginURL := &url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(cfg.LoginHost, strconv.Itoa(cfg.LoginPort)),
	}
	gameURL := &url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(cfg.GameHost, strconv.Itoa(cfg.GamePort)),
	}

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

	loginProxy := httputil.NewSingleHostReverseProxy(loginURL)
	gameProxy := httputil.NewSingleHostReverseProxy(gameURL)
	loginProxy.Transport = transport
	gameProxy.Transport = transport
	loginProxy.ErrorHandler = proxyErrorHandler
	gameProxy.ErrorHandler = proxyErrorHandler

	return &GatewayServer{
		cfg:        cfg,
		loginProxy: loginProxy,
		gameProxy:  gameProxy,
	}, nil
}

func proxyErrorHandler(w http.ResponseWriter, _ *http.Request, err error) {
	applog.Errorf("proxy upstream failed: %v", err)
	writeJSON(w, http.StatusBadGateway, APIResp{ErrMsg: "proxy upstream failed: " + err.Error()})
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
	// Compatible with historical path used by svr_login.
	return normalized == "/wxlogin"
}

func (g *GatewayServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		setCommonHeaders(w)
		w.WriteHeader(http.StatusOK)
		return
	}

	if g.isLoginRoute(r.URL.Path) {
		setCommonHeaders(w)
		loginReq := r.Clone(r.Context())
		loginReq.URL.Path = g.cfg.LoginTargetPath
		loginReq.URL.RawPath = ""
		g.loginProxy.ServeHTTP(w, loginReq)
		return
	}

	token, err := extractBearerToken(r)
	if err != nil {
		applog.Errorf("auth failed: %v", err)
		writeJSON(w, http.StatusUnauthorized, APIResp{ErrMsg: err.Error()})
		return
	}

	openid, err := parseOpenIDFromJWT(token, g.cfg.JWTSecret)
	if err != nil {
		applog.Errorf("jwt verify failed: %v", err)
		writeJSON(w, http.StatusUnauthorized, APIResp{ErrMsg: "JWT verify failed: " + err.Error()})
		return
	}

	setCommonHeaders(w)
	gameReq := r.Clone(r.Context())
	gameReq.Header.Del("Authorization")
	gameReq.Header.Set("X-OpenID", openid)
	g.gameProxy.ServeHTTP(w, gameReq)
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

	cfg, err := loadGatewayConfig(gatewayConfigPath)
	if err != nil {
		applog.Errorf("load gateway config failed: %v", err)
		time.Sleep(1000000)
		return
	}

	listenAddr := fmt.Sprintf(":%d", cfg.GatewayPort)

	server, err := newGatewayServer(cfg)
	if err != nil {
		applog.Errorf("init gateway failed: %v", err)
		time.Sleep(1000000)
		return
	}

	applog.Infof("svr_gateway start, will listen to: http://0.0.0.0%s", listenAddr)
	if err := http.ListenAndServe(listenAddr, server); err != nil {
		applog.Errorf("gateway init failed: %v", err)
	}
}
