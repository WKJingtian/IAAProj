package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"common/applog"
	"common/config"

	"github.com/golang-jwt/jwt/v5"
)

const defaultConfigPath = "config.json"

type ServerConfig struct {
	WXAppID     string `json:"wx_app_id"`
	WXAppSecret string `json:"wx_app_secret"`
	LoginPort   int    `json:"login_port"`
	JWTSecret   string `json:"jwt_secret"`
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
	ErrMsg string `json:"errMsg"`
}

func loadServerConfig(path string) (ServerConfig, error) {
	var cfg ServerConfig
	if err := config.LoadJSONConfig(path, &cfg); err != nil {
		return ServerConfig{}, err
	}

	if strings.TrimSpace(cfg.WXAppID) == "" {
		return ServerConfig{}, fmt.Errorf("wx_app_id cannot be empty")
	}
	if strings.TrimSpace(cfg.WXAppSecret) == "" {
		return ServerConfig{}, fmt.Errorf("wx_app_secret cannot be empty")
	}
	if strings.TrimSpace(cfg.JWTSecret) == "" {
		return ServerConfig{}, fmt.Errorf("jwt_secret cannot be empty")
	}
	if cfg.LoginPort <= 0 || cfg.LoginPort > 65535 {
		return ServerConfig{}, fmt.Errorf("login_port must be in range 1-65535")
	}

	return cfg, nil
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
		_ = json.NewEncoder(w).Encode(LoginResp{ErrMsg: "Only support POST request"})
		return
	}

	if err := r.ParseForm(); err != nil {
		applog.Errorf("parse form failed: %v", err)
		_ = json.NewEncoder(w).Encode(LoginResp{ErrMsg: "Fail to parse args: " + err.Error()})
		return
	}

	code := r.PostForm.Get("code")
	appid := r.PostForm.Get("appid")

	if code == "" {
		applog.Errorf("wxlogin request missing code")
		_ = json.NewEncoder(w).Encode(LoginResp{ErrMsg: "code cannot be empty"})
		return
	}
	if appid != appConfig.WXAppID {
		applog.Errorf("appid mismatch: incoming=%s", appid)
		_ = json.NewEncoder(w).Encode(LoginResp{ErrMsg: fmt.Sprintf("AppID error: incoming app id -> %s", appid)})
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
		_ = json.NewEncoder(w).Encode(LoginResp{ErrMsg: "Fail to fetch WX URL: " + err.Error()})
		return
	}
	defer wxResp.Body.Close()

	var wxResult WxCode2SessionResp
	if err := json.NewDecoder(wxResp.Body).Decode(&wxResult); err != nil {
		applog.Errorf("decode wx response failed: %v", err)
		_ = json.NewEncoder(w).Encode(LoginResp{ErrMsg: "Fail to parse json from WX server: " + err.Error()})
		return
	}

	if wxResult.ErrCode != 0 {
		applog.Errorf("wx api returned error: errcode=%d errmsg=%s", wxResult.ErrCode, wxResult.ErrMsg)
		_ = json.NewEncoder(w).Encode(LoginResp{ErrMsg: fmt.Sprintf("WX API error: %d - %s", wxResult.ErrCode, wxResult.ErrMsg)})
		return
	}

	token, err := generateJWT(wxResult.OpenID, appConfig.JWTSecret)
	if err != nil {
		applog.Errorf("jwt generation failed: %v", err)
		_ = json.NewEncoder(w).Encode(LoginResp{ErrMsg: "internal error. JWT generation failed."})
		return
	}

	_ = json.NewEncoder(w).Encode(LoginResp{
		OpenID: wxResult.OpenID,
		Token:  token,
		ErrMsg: "",
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

	cfg, err := loadServerConfig(defaultConfigPath)
	if err != nil {
		applog.Errorf("load config failed: %v", err)
		time.Sleep(1000000)
		return
	}
	appConfig = cfg
	listenAddr := fmt.Sprintf(":%d", appConfig.LoginPort)

	http.HandleFunc("/wxlogin", wxLoginHandler)
	http.HandleFunc("/test", testHandler)
	applog.Infof("login server start, will listen to: http://0.0.0.0%s", listenAddr)
	if err := http.ListenAndServe(listenAddr, nil); err != nil {
		applog.Errorf("server init failed: %v", err)
	}
}
