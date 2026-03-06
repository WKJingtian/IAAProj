package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"common/config"
	cmongo "common/mongo"
)

const (
	appConfigPath   = "config.json"
	mongoConfigPath = "mongo_config.json"
)

type AppConfig struct {
	GamePort int `json:"game_port"`
}

type APIResp struct {
	Method string `json:"method,omitempty"`
	ErrMsg string `json:"errMsg"`
}

func loadAppConfig(path string) (AppConfig, error) {
	var cfg AppConfig
	if err := config.LoadJSONConfig(path, &cfg); err != nil {
		return AppConfig{}, err
	}

	if cfg.GamePort <= 0 || cfg.GamePort > 65535 {
		return AppConfig{}, fmt.Errorf("game_port must be in range 1-65535")
	}
	return cfg, nil
}

func setCommonHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, PATCH")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-OpenID")
	w.Header().Set("Content-Type", "application/json;charset=utf-8")
}

func writeJSON(w http.ResponseWriter, status int, resp APIResp) {
	setCommonHeaders(w)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}

func requestHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		setCommonHeaders(w)
		w.WriteHeader(http.StatusOK)
		return
	}

	openid := strings.TrimSpace(r.Header.Get("X-OpenID"))
	if openid == "" {
		writeJSON(w, http.StatusUnauthorized, APIResp{ErrMsg: "missing X-OpenID header"})
		return
	}

	db, err := cmongo.Database()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, APIResp{ErrMsg: "get mongo database failed: " + err.Error()})
		return
	}

	insertCtx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	_, err = db.Collection("request_logs").InsertOne(insertCtx, map[string]any{
		"method":     r.Method,
		"path":       r.URL.Path,
		"openid":     openid,
		"created_at": time.Now().UTC(),
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, APIResp{ErrMsg: "insert request log failed: " + err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, APIResp{
		Method: r.Method,
		ErrMsg: "",
	})
}

func main() {
	cfg, err := loadAppConfig(appConfigPath)
	if err != nil {
		fmt.Printf("load app config failed: %s\n", err.Error())
		return
	}

	listenAddr := fmt.Sprintf(":%d", cfg.GamePort)

	if _, err := cmongo.InitFromJSON(context.Background(), mongoConfigPath); err != nil {
		fmt.Printf("init mongo failed: %s\n", err.Error())
		return
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = cmongo.Disconnect(ctx)
	}()

	http.HandleFunc("/", requestHandler)
	fmt.Printf("svr_game start, will listen to: http://0.0.0.0%s\n", listenAddr)
	if err := http.ListenAndServe(listenAddr, nil); err != nil {
		fmt.Printf("server init failed: %s\n", err.Error())
	}
}
