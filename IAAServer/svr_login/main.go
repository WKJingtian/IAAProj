package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"common/applog"
	"common/config"
	"common/errorcode"
	cservice "common/service"

	"github.com/golang-jwt/jwt/v5"
)

const (
	defaultConfigPath     = "config.json"
	loginConfigPathEnv    = "IAA_LOGIN_CONFIG_PATH"
	heartbeatInterval     = 3 * time.Second
	registerRetryInterval = 2 * time.Second
)

type ServerConfig struct {
	WXAppID              string `json:"wx_app_id"`
	WXAppSecret          string `json:"wx_app_secret"`
	LoginPort            int    `json:"login_port"`
	JWTSecret            string `json:"jwt_secret"`
	GatewayHost          string `json:"gateway_host"`
	GatewayPort          int    `json:"gateway_port"`
	GatewayRegisterPath  string `json:"gateway_register_path"`
	GatewayHeartbeatPath string `json:"gateway_heartbeat_path"`
	ReportIP             string `json:"report_ip"`
	ServerID             string `json:"server_id"`
}

var appConfig ServerConfig

type WxCode2SessionResp struct {
	OpenID     string `json:"openid"`
	SessionKey string `json:"session_key"`
	UnionID    string `json:"unionid"`
	ErrCode    int    `json:"errcode"`
	ErrMsg     string `json:"errmsg"`
}

type TestResp struct {
	TestRespContentHeader string `json:"content"`
}

type LoginResp struct {
	OpenID string `json:"openid"`
	Token  string `json:"token"`
	ErrMsg uint16 `json:"err_msg"`
}

type HealthResp struct {
	Status  string `json:"status"`
	Service string `json:"service"`
	ErrMsg  uint16 `json:"err_msg,omitempty"`
}

func loadServerConfig(path string) (ServerConfig, error) {
	var cfg ServerConfig
	if err := config.LoadJSONConfig(path, &cfg); err != nil {
		return ServerConfig{}, err
	}

	cfg.WXAppID = strings.TrimSpace(cfg.WXAppID)
	cfg.WXAppSecret = strings.TrimSpace(cfg.WXAppSecret)
	cfg.JWTSecret = strings.TrimSpace(cfg.JWTSecret)
	cfg.GatewayHost = strings.TrimSpace(cfg.GatewayHost)
	cfg.ReportIP = strings.TrimSpace(cfg.ReportIP)
	cfg.ServerID = strings.TrimSpace(cfg.ServerID)
	cfg.GatewayRegisterPath = normalizePathOrDefault(cfg.GatewayRegisterPath, "/register")
	cfg.GatewayHeartbeatPath = normalizePathOrDefault(cfg.GatewayHeartbeatPath, "/heartbeat")

	if cfg.WXAppID == "" {
		return ServerConfig{}, fmt.Errorf("wx_app_id cannot be empty")
	}
	if cfg.WXAppSecret == "" {
		return ServerConfig{}, fmt.Errorf("wx_app_secret cannot be empty")
	}
	if cfg.JWTSecret == "" {
		return ServerConfig{}, fmt.Errorf("jwt_secret cannot be empty")
	}
	if cfg.GatewayHost == "" {
		return ServerConfig{}, fmt.Errorf("gateway_host cannot be empty")
	}
	if cfg.ReportIP == "" {
		return ServerConfig{}, fmt.Errorf("report_ip cannot be empty")
	}
	if cfg.ServerID == "" {
		return ServerConfig{}, fmt.Errorf("server_id cannot be empty")
	}
	if cfg.LoginPort <= 0 || cfg.LoginPort > 65535 {
		return ServerConfig{}, fmt.Errorf("login_port must be in range 1-65535")
	}
	if cfg.GatewayPort <= 0 || cfg.GatewayPort > 65535 {
		return ServerConfig{}, fmt.Errorf("gateway_port must be in range 1-65535")
	}

	return cfg, nil
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

func generateJWT(openid string, secret string) (string, error) {
	claims := jwt.MapClaims{
		"openid": openid,
		"exp":    time.Now().Add(7 * 24 * time.Hour).Unix(),
		"iat":    time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func configPathFromEnv(envName string, defaultPath string) string {
	if value := strings.TrimSpace(os.Getenv(envName)); value != "" {
		return value
	}
	return defaultPath
}

func registerURL(cfg ServerConfig) string {
	return fmt.Sprintf("http://%s:%d%s", cfg.GatewayHost, cfg.GatewayPort, cfg.GatewayRegisterPath)
}

func heartbeatURL(cfg ServerConfig) string {
	return fmt.Sprintf("http://%s:%d%s", cfg.GatewayHost, cfg.GatewayPort, cfg.GatewayHeartbeatPath)
}

func maintainGatewayLease(ctx context.Context, cfg ServerConfig, shutdownCh chan<- string) {
	instance := cservice.Instance{
		Type: cservice.TypeLogin,
		ID:   cfg.ServerID,
		Host: cfg.ReportIP,
		Port: cfg.LoginPort,
	}
	client := &http.Client{Timeout: 3 * time.Second}
	registerEndpoint := registerURL(cfg)
	heartbeatEndpoint := heartbeatURL(cfg)

	leaseID := ""
	var terminateTimer *time.Timer
	superseded := false

	stopTimer := func() {
		if terminateTimer != nil {
			terminateTimer.Stop()
			terminateTimer = nil
		}
	}

	triggerShutdown := func(reason string) {
		select {
		case shutdownCh <- reason:
		default:
		}
	}

	for {
		select {
		case <-ctx.Done():
			stopTimer()
			return
		default:
		}

		if leaseID == "" {
			registerCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			response, err := cservice.Register(registerCtx, client, registerEndpoint, instance)
			cancel()
			if err != nil {
				applog.Errorf("register to gateway failed: %v", err)
				select {
				case <-ctx.Done():
					return
				case <-time.After(registerRetryInterval):
					continue
				}
			}

			leaseID = response.LeaseID
			superseded = false
			stopTimer()
			applog.Infof("registered to gateway: type=%s id=%s addr=%s:%d lease=%s gateway=%s:%d",
				instance.Type, instance.ID, instance.Host, instance.Port, leaseID, cfg.GatewayHost, cfg.GatewayPort)
		}

		heartbeatCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		response, err := cservice.Heartbeat(heartbeatCtx, client, heartbeatEndpoint, cservice.HeartbeatRequest{
			Type:    instance.Type,
			ID:      instance.ID,
			LeaseID: leaseID,
		})
		cancel()
		if err != nil {
			applog.Errorf("heartbeat to gateway failed: %v", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(heartbeatInterval):
				continue
			}
		}

		switch response.State {
		case cservice.LeaseStateActive:
			if superseded {
				break
			}
			stopTimer()
		case cservice.LeaseStateSuperseded:
			superseded = true
			delay := time.Duration(response.TerminateAfterSec) * time.Second
			if response.TerminateAfterSec <= 0 {
				applog.Infof("lease superseded, shutdown immediately")
				triggerShutdown("superseded")
				return
			}
			if terminateTimer == nil {
				applog.Infof("lease superseded, shutdown in %ds", response.TerminateAfterSec)
				terminateTimer = time.AfterFunc(delay, func() {
					triggerShutdown("superseded")
				})
			}
		case cservice.LeaseStateUnknown:
			if superseded {
				break
			}
			applog.Infof("lease unknown, will re-register")
			leaseID = ""
			stopTimer()
		default:
			applog.Errorf("unexpected lease state: %s", response.State)
		}

		select {
		case <-ctx.Done():
			stopTimer()
			return
		case <-time.After(heartbeatInterval):
		}
	}
}

func writeHealthJSON(w http.ResponseWriter, status int, resp HealthResp) {
	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}

func healthHandler(serviceName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeHealthJSON(w, http.StatusMethodNotAllowed, HealthResp{
				Status:  "error",
				Service: serviceName,
				ErrMsg:  uint16(errorcode.InvalidMethod),
			})
			return
		}

		writeHealthJSON(w, http.StatusOK, HealthResp{
			Status:  "ok",
			Service: serviceName,
		})
	}
}

func testHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json;charset=utf-8")

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(TestResp{TestRespContentHeader: r.Method})
}

func wxLoginHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json;charset=utf-8")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		applog.Errorf("invalid method for /wxlogin: %s", r.Method)
		_ = json.NewEncoder(w).Encode(LoginResp{ErrMsg: uint16(errorcode.InvalidMethod)})
		return
	}

	if err := r.ParseForm(); err != nil {
		applog.Errorf("parse form failed: %v", err)
		_ = json.NewEncoder(w).Encode(LoginResp{ErrMsg: uint16(errorcode.InvalidRequest)})
		return
	}

	code := r.PostForm.Get("code")
	appid := r.PostForm.Get("appid")

	if code == "" {
		applog.Errorf("wxlogin request missing code")
		_ = json.NewEncoder(w).Encode(LoginResp{ErrMsg: uint16(errorcode.LoginCodeEmpty)})
		return
	}
	if appid != appConfig.WXAppID {
		applog.Errorf("appid mismatch: incoming=%s", appid)
		_ = json.NewEncoder(w).Encode(LoginResp{ErrMsg: uint16(errorcode.LoginAppIDMismatch)})
		return
	}

	wxURL := fmt.Sprintf(
		"https://api.weixin.qq.com/sns/jscode2session?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code",
		url.QueryEscape(appConfig.WXAppID),
		url.QueryEscape(appConfig.WXAppSecret),
		url.QueryEscape(code),
	)

	wxResp, err := http.Get(wxURL)
	if err != nil {
		applog.Errorf("fetch wx url failed: %v", err)
		_ = json.NewEncoder(w).Encode(LoginResp{ErrMsg: uint16(errorcode.LoginWXRequestFailed)})
		return
	}
	defer wxResp.Body.Close()

	var wxResult WxCode2SessionResp
	if err := json.NewDecoder(wxResp.Body).Decode(&wxResult); err != nil {
		applog.Errorf("decode wx response failed: %v", err)
		_ = json.NewEncoder(w).Encode(LoginResp{ErrMsg: uint16(errorcode.LoginWXResponseInvalid)})
		return
	}

	if wxResult.ErrCode != 0 {
		applog.Errorf("wx api returned error: errcode=%d errmsg=%s", wxResult.ErrCode, wxResult.ErrMsg)
		_ = json.NewEncoder(w).Encode(LoginResp{ErrMsg: uint16(errorcode.LoginWXAPIError)})
		return
	}

	token, err := generateJWT(wxResult.OpenID, appConfig.JWTSecret)
	if err != nil {
		applog.Errorf("jwt generation failed: %v", err)
		_ = json.NewEncoder(w).Encode(LoginResp{ErrMsg: uint16(errorcode.LoginJWTGenerationFailed)})
		return
	}

	_ = json.NewEncoder(w).Encode(LoginResp{
		OpenID: wxResult.OpenID,
		Token:  token,
		ErrMsg: uint16(errorcode.OK),
	})
}

func main() {
	if err := applog.Init("svr_login"); err != nil {
		fmt.Printf("init logger failed: %s\n", err.Error())
	}
	defer func() {
		if err := applog.Close(); err != nil {
			fmt.Printf("close logger failed: %s\n", err.Error())
		}
	}()
	defer applog.CatchPanic()

	cfg, err := loadServerConfig(configPathFromEnv(loginConfigPathEnv, defaultConfigPath))
	if err != nil {
		applog.Errorf("load config failed: %v", err)
		return
	}
	appConfig = cfg
	listenAddr := fmt.Sprintf(":%d", appConfig.LoginPort)

	mux := http.NewServeMux()
	mux.HandleFunc("/wxlogin", wxLoginHandler)
	mux.HandleFunc("/test", testHandler)
	mux.HandleFunc("/livez", healthHandler("svr_login"))
	mux.HandleFunc("/readyz", healthHandler("svr_login"))

	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		applog.Errorf("listen failed: %v", err)
		return
	}

	server := &http.Server{Handler: mux}
	errCh := make(chan error, 1)
	go func() {
		err := server.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	shutdownCh := make(chan string, 1)
	leaseCtx, cancelLease := context.WithCancel(context.Background())
	defer cancelLease()
	go maintainGatewayLease(leaseCtx, appConfig, shutdownCh)

	applog.Infof("login server start, will listen to: http://0.0.0.0%s", listenAddr)

	select {
	case err := <-errCh:
		cancelLease()
		if err != nil {
			applog.Errorf("server init failed: %v", err)
		}
	case reason := <-shutdownCh:
		applog.Infof("shutdown requested: %s", reason)
		cancelLease()
		_ = server.Close()
		if err := <-errCh; err != nil {
			applog.Errorf("server close failed: %v", err)
		}
	}
}
