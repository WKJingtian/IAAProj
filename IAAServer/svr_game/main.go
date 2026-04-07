package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"common/applog"
	"common/config"
	"common/errorcode"
	"common/idgen"
	cmongo "common/mongo"
	cservice "common/service"
	game "svr_game/game"
	"svr_game/httpapi"
	"svr_game/staticdata"

	"go.mongodb.org/mongo-driver/mongo/readpref"
)

const (
	appConfigPath         = "config.json"
	mongoConfigPath       = "mongo_config.json"
	appConfigPathEnv      = "IAA_GAME_CONFIG_PATH"
	mongoConfigPathEnv    = "IAA_MONGO_CONFIG_PATH"
	heartbeatInterval     = 3 * time.Second
	registerRetryInterval = 2 * time.Second
)

type AppConfig struct {
	GamePort             int    `json:"game_port"`
	MainCollection       string `json:"main_collection"`
	GatewayHost          string `json:"gateway_host"`
	GatewayPort          int    `json:"gateway_port"`
	GatewayRegisterPath  string `json:"gateway_register_path"`
	GatewayHeartbeatPath string `json:"gateway_heartbeat_path"`
	ReportIP             string `json:"report_ip"`
	ServerID             string `json:"server_id"`
	GeneratorID          uint16 `json:"generator_id"`
	PlayerCacheTTLSec    int64  `json:"player_cache_ttl_sec"`
	PlayerFlushSec       int64  `json:"player_flush_interval_sec"`
	DataDir              string `json:"data_dir"`
	CheatModeOn          int    `json:"cheat_mode_on"`
}

type HealthResp struct {
	Status  string `json:"status"`
	Service string `json:"service"`
	ErrMsg  uint16 `json:"err_msg,omitempty"`
}

func loadAppConfig(path string) (AppConfig, error) {
	var cfg AppConfig
	if err := config.LoadJSONConfig(path, &cfg); err != nil {
		return AppConfig{}, err
	}

	cfg.MainCollection = strings.TrimSpace(cfg.MainCollection)
	cfg.GatewayHost = strings.TrimSpace(cfg.GatewayHost)
	cfg.ReportIP = strings.TrimSpace(cfg.ReportIP)
	cfg.ServerID = strings.TrimSpace(cfg.ServerID)
	cfg.GatewayRegisterPath = normalizePathOrDefault(cfg.GatewayRegisterPath, "/register")
	cfg.GatewayHeartbeatPath = normalizePathOrDefault(cfg.GatewayHeartbeatPath, "/heartbeat")
	cfg.DataDir = resolvePathRelativeToConfig(path, cfg.DataDir, ".")

	if cfg.GamePort <= 0 || cfg.GamePort > 65535 {
		return AppConfig{}, fmt.Errorf("game_port must be in range 1-65535")
	}
	if cfg.MainCollection == "" {
		return AppConfig{}, fmt.Errorf("main_collection cannot be empty")
	}
	if cfg.GatewayHost == "" {
		return AppConfig{}, fmt.Errorf("gateway_host cannot be empty")
	}
	if cfg.ReportIP == "" {
		return AppConfig{}, fmt.Errorf("report_ip cannot be empty")
	}
	if cfg.ServerID == "" {
		return AppConfig{}, fmt.Errorf("server_id cannot be empty")
	}
	if cfg.GeneratorID > idgen.MaxGeneratorID {
		return AppConfig{}, fmt.Errorf("generator_id must be in range 0-%d", idgen.MaxGeneratorID)
	}
	if cfg.GatewayPort <= 0 || cfg.GatewayPort > 65535 {
		return AppConfig{}, fmt.Errorf("gateway_port must be in range 1-65535")
	}
	if cfg.PlayerCacheTTLSec <= 0 {
		cfg.PlayerCacheTTLSec = 600
	}
	if cfg.PlayerFlushSec <= 0 {
		cfg.PlayerFlushSec = 5
	}

	return cfg, nil
}

func resolvePathRelativeToConfig(configPath string, rawPath string, defaultPath string) string {
	path := strings.TrimSpace(rawPath)
	if path == "" {
		path = defaultPath
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}

	configDir := filepath.Dir(configPath)
	if strings.TrimSpace(configDir) == "" {
		configDir = "."
	}

	return filepath.Clean(filepath.Join(configDir, path))
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

func registerURL(cfg AppConfig) string {
	return fmt.Sprintf("http://%s:%d%s", cfg.GatewayHost, cfg.GatewayPort, cfg.GatewayRegisterPath)
}

func heartbeatURL(cfg AppConfig) string {
	return fmt.Sprintf("http://%s:%d%s", cfg.GatewayHost, cfg.GatewayPort, cfg.GatewayHeartbeatPath)
}

func maintainGatewayLease(ctx context.Context, cfg AppConfig, shutdownCh chan<- string, flushCh chan<- struct{}) {
	instance := cservice.Instance{
		Type: cservice.TypeGame,
		ID:   cfg.ServerID,
		Host: cfg.ReportIP,
		Port: cfg.GamePort,
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

	triggerFlush := func() {
		select {
		case flushCh <- struct{}{}:
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
				triggerFlush()
				triggerShutdown("superseded")
				return
			}
			if terminateTimer == nil {
				applog.Infof("lease superseded, shutdown in %ds", response.TerminateAfterSec)
				triggerFlush()
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

func livenessHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeHealthJSON(w, http.StatusMethodNotAllowed, HealthResp{
			Status:  "error",
			Service: "svr_game",
			ErrMsg:  uint16(errorcode.InvalidMethod),
		})
		return
	}

	writeHealthJSON(w, http.StatusOK, HealthResp{
		Status:  "ok",
		Service: "svr_game",
	})
}

func readinessHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeHealthJSON(w, http.StatusMethodNotAllowed, HealthResp{
			Status:  "error",
			Service: "svr_game",
			ErrMsg:  uint16(errorcode.InvalidMethod),
		})
		return
	}

	client, err := cmongo.Client()
	if err != nil {
		writeHealthJSON(w, http.StatusServiceUnavailable, HealthResp{
			Status:  "not_ready",
			Service: "svr_game",
			ErrMsg:  uint16(errorcode.InternalError),
		})
		return
	}

	pingCtx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := client.Ping(pingCtx, readpref.Primary()); err != nil {
		writeHealthJSON(w, http.StatusServiceUnavailable, HealthResp{
			Status:  "not_ready",
			Service: "svr_game",
			ErrMsg:  uint16(errorcode.InternalError),
		})
		return
	}

	writeHealthJSON(w, http.StatusOK, HealthResp{
		Status:  "ok",
		Service: "svr_game",
	})
}

func main() {
	if err := applog.Init("svr_game"); err != nil {
		fmt.Printf("init logger failed: %s\n", err.Error())
	}
	defer func() {
		if err := applog.Close(); err != nil {
			fmt.Printf("close logger failed: %s\n", err.Error())
		}
	}()
	defer applog.CatchPanic()

	resolvedAppConfigPath := configPathFromEnv(appConfigPathEnv, appConfigPath)
	cfg, err := loadAppConfig(resolvedAppConfigPath)
	if err != nil {
		applog.Errorf("load app config failed: %v", err)
		return
	}

	listenAddr := fmt.Sprintf(":%d", cfg.GamePort)

	staticData, err := staticdata.LoadStaticData(cfg.DataDir)
	if err != nil {
		applog.Errorf("load static data failed: %v", err)
		return
	}

	if _, err := cmongo.InitFromJSON(context.Background(), configPathFromEnv(mongoConfigPathEnv, mongoConfigPath)); err != nil {
		applog.Errorf("init mongo failed: %v", err)
		return
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = cmongo.Disconnect(ctx)
	}()

	playerIDGenerator, err := idgen.NewSnowflakeGenerator(idgen.SnowflakeConfig{
		GeneratorID: cfg.GeneratorID,
	})
	if err != nil {
		applog.Errorf("init player id generator failed: %v", err)
		return
	}

	gameService, err := game.NewService(
		cfg.MainCollection,
		time.Duration(cfg.PlayerCacheTTLSec)*time.Second,
		time.Duration(cfg.PlayerFlushSec)*time.Second,
		staticData,
		cfg.CheatModeOn,
		playerIDGenerator,
	)
	if err != nil {
		applog.Errorf("init game service failed: %v", err)
		return
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := gameService.Close(ctx); err != nil {
			applog.Errorf("close game service failed: %v", err)
		}
	}()
	httpHandler := httpapi.NewHandler(gameService)

	if err := gameService.EnsureIndexes(context.Background()); err != nil {
		applog.Errorf("ensure indexes failed: %v", err)
		return
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/livez", livenessHandler)
	mux.HandleFunc("/readyz", readinessHandler)
	httpHandler.RegisterRoutes(mux)

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
	flushCh := make(chan struct{}, 1)
	leaseCtx, cancelLease := context.WithCancel(context.Background())
	defer cancelLease()
	go maintainGatewayLease(leaseCtx, cfg, shutdownCh, flushCh)

	applog.Infof("svr_game start, will listen to: http://0.0.0.0%s, generator_id=%d", listenAddr, cfg.GeneratorID)

	for {
		select {
		case err := <-errCh:
			cancelLease()
			if err != nil {
				applog.Errorf("server init failed: %v", err)
			}
			return
		case <-flushCh:
			flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := gameService.FlushNow(flushCtx); err != nil {
				applog.Errorf("flush player cache on supersede failed: %v", err)
			}
			cancel()
		case reason := <-shutdownCh:
			applog.Infof("shutdown requested: %s", reason)
			cancelLease()
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := server.Shutdown(shutdownCtx); err != nil {
				applog.Errorf("server shutdown failed: %v", err)
			}
			cancel()
			flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := gameService.FlushNow(flushCtx); err != nil {
				applog.Errorf("flush player cache on shutdown failed: %v", err)
			}
			cancel()
			if err := <-errCh; err != nil {
				applog.Errorf("server close failed: %v", err)
			}
			return
		}
	}
}
