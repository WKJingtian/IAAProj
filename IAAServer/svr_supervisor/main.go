package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"common/applog"
	"common/config"
)

const (
	supervisorConfigPath    = "config.json"
	supervisorConfigPathEnv = "IAA_SUPERVISOR_CONFIG_PATH"
	stateFileName           = "supervisor_state.json"
)

type SupervisorConfig struct {
	ControlHost            string `json:"control_host"`
	ControlPort            int    `json:"control_port"`
	RuntimeRoot            string `json:"runtime_root"`
	HealthHost             string `json:"health_host"`
	HealthCheckIntervalSec int    `json:"health_check_interval_sec"`
	HealthFailureThreshold int    `json:"health_failure_threshold"`
	RestartBackoffSec      int    `json:"restart_backoff_sec"`
	DefaultGameServiceID   string `json:"default_game_service_id"`
}

type GameInstanceState struct {
	Release      string `json:"release"`
	ServiceID    string `json:"service_id"`
	Port         int    `json:"port"`
	InstanceName string `json:"instance_name"`
}

type SupervisorState struct {
	BaseRelease   string               `json:"base_release"`
	Running       bool                 `json:"running"`
	ActiveGame    *GameInstanceState   `json:"active_game,omitempty"`
	RetiringGames []*GameInstanceState `json:"retiring_games,omitempty"`
}

type managedProcess struct {
	Key           string
	ServiceName   string
	Release       string
	RuntimeDir    string
	BinaryPath    string
	ConfigPath    string
	MongoConfig   string
	HealthURL     string
	AppLogPath    string
	StdoutLogPath string
	Env           []string
	Restartable   bool
	Retiring      bool
	Cmd           *exec.Cmd
	StartedAt     time.Time
	FailCount     int
}

type processExit struct {
	Key string
	Err error
}

type bootstrapRequest struct {
	BaseRelease string `json:"base_release"`
}

type deployGameRequest struct {
	Release string `json:"release"`
}

type apiResp struct {
	ErrMsg string `json:"err_msg"`
}

type statusResponse struct {
	ControlPID int                       `json:"control_pid"`
	State     SupervisorState         `json:"state"`
	Processes map[string]processBrief `json:"processes"`
}

type processBrief struct {
	ServiceName string `json:"service_name"`
	Release     string `json:"release"`
	RuntimeDir  string `json:"runtime_dir"`
	HealthURL   string `json:"health_url"`
	Retiring    bool   `json:"retiring"`
	Running     bool   `json:"running"`
}

type Supervisor struct {
	cfg           SupervisorConfig
	state         SupervisorState
	statePath     string
	releasesDir   string
	instancesDir  string
	logsDir       string
	processMu     sync.Mutex
	processes     map[string]*managedProcess
	exitCh        chan processExit
	httpClient    *http.Client
	stoppingAll   bool
	stopControl   func()
}

func loadSupervisorConfig(path string) (SupervisorConfig, error) {
	var cfg SupervisorConfig
	if err := config.LoadJSONConfig(path, &cfg); err != nil {
		return SupervisorConfig{}, err
	}

	cfg.ControlHost = strings.TrimSpace(cfg.ControlHost)
	cfg.RuntimeRoot = strings.TrimSpace(cfg.RuntimeRoot)
	cfg.HealthHost = strings.TrimSpace(cfg.HealthHost)
	cfg.DefaultGameServiceID = strings.TrimSpace(cfg.DefaultGameServiceID)

	if cfg.ControlHost == "" {
		cfg.ControlHost = "127.0.0.1"
	}
	if cfg.HealthHost == "" {
		cfg.HealthHost = "127.0.0.1"
	}
	if cfg.RuntimeRoot == "" {
		cfg.RuntimeRoot = "./linux_runtime"
	}
	if cfg.DefaultGameServiceID == "" {
		cfg.DefaultGameServiceID = "game-1"
	}
	if cfg.ControlPort <= 0 || cfg.ControlPort > 65535 {
		return SupervisorConfig{}, fmt.Errorf("control_port must be in range 1-65535")
	}
	if cfg.HealthCheckIntervalSec <= 0 {
		cfg.HealthCheckIntervalSec = 3
	}
	if cfg.HealthFailureThreshold <= 0 {
		cfg.HealthFailureThreshold = 3
	}
	if cfg.RestartBackoffSec <= 0 {
		cfg.RestartBackoffSec = 2
	}

	return cfg, nil
}

func configPathFromEnv(envName string, defaultPath string) string {
	if value := strings.TrimSpace(os.Getenv(envName)); value != "" {
		return value
	}
	return defaultPath
}

func newSupervisor(cfg SupervisorConfig) (*Supervisor, error) {
	runtimeRoot, err := filepath.Abs(cfg.RuntimeRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve runtime root failed: %w", err)
	}

	stateDir := filepath.Join(runtimeRoot, "state")
	releasesDir := filepath.Join(runtimeRoot, "releases")
	instancesDir := filepath.Join(runtimeRoot, "instances")
	logsDir := filepath.Join(runtimeRoot, "logs")

	for _, dir := range []string{runtimeRoot, stateDir, releasesDir, instancesDir, logsDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create runtime dir failed: %w", err)
		}
	}

	supervisor := &Supervisor{
		cfg:          cfg,
		statePath:    filepath.Join(stateDir, stateFileName),
		releasesDir:  releasesDir,
		instancesDir: instancesDir,
		logsDir:      logsDir,
		processes:    make(map[string]*managedProcess),
		exitCh:       make(chan processExit, 32),
		httpClient:   &http.Client{Timeout: 2 * time.Second},
	}

	if err := supervisor.loadState(); err != nil {
		return nil, err
	}
	return supervisor, nil
}

func (s *Supervisor) loadState() error {
	if _, err := os.Stat(s.statePath); errors.Is(err, os.ErrNotExist) {
		s.state = SupervisorState{}
		return nil
	}
	return config.LoadJSONConfig(s.statePath, &s.state)
}

func (s *Supervisor) saveStateLocked() error {
	data, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state failed: %w", err)
	}
	if err := os.WriteFile(s.statePath, data, 0644); err != nil {
		return fmt.Errorf("write state failed: %w", err)
	}
	return nil
}

func (s *Supervisor) releaseServiceDir(release string, service string) string {
	return filepath.Join(s.releasesDir, release, service)
}

func (s *Supervisor) releaseBinary(release string, service string) string {
	return filepath.Join(s.releaseServiceDir(release, service), service)
}

func (s *Supervisor) releaseConfig(release string, service string) string {
	return filepath.Join(s.releaseServiceDir(release, service), "config.json")
}

func (s *Supervisor) releaseMongoConfig(release string) string {
	return filepath.Join(s.releaseServiceDir(release, "svr_game"), "mongo_config.json")
}

func (s *Supervisor) ensureReleaseExists(release string) error {
	if strings.TrimSpace(release) == "" {
		return errors.New("release cannot be empty")
	}
	if _, err := os.Stat(filepath.Join(s.releasesDir, release)); err != nil {
		return fmt.Errorf("release %q not found under %s", release, s.releasesDir)
	}
	return nil
}

func (s *Supervisor) loadGatewayConfig(release string) (GatewayConfig, error) {
	var cfg GatewayConfig
	if err := config.LoadJSONConfig(s.releaseConfig(release, "svr_gateway"), &cfg); err != nil {
		return GatewayConfig{}, err
	}
	return cfg, nil
}

func (s *Supervisor) loadLoginConfig(release string) (LoginConfig, error) {
	var cfg LoginConfig
	if err := config.LoadJSONConfig(s.releaseConfig(release, "svr_login"), &cfg); err != nil {
		return LoginConfig{}, err
	}
	return cfg, nil
}

func (s *Supervisor) loadGameConfig(release string) (GameConfig, error) {
	var cfg GameConfig
	if err := config.LoadJSONConfig(s.releaseConfig(release, "svr_game"), &cfg); err != nil {
		return GameConfig{}, err
	}
	return cfg, nil
}

func (s *Supervisor) writeGameRuntimeConfig(state *GameInstanceState) (string, error) {
	baseCfg, err := s.loadGameConfig(state.Release)
	if err != nil {
		return "", err
	}
	baseCfg.GamePort = state.Port
	baseCfg.ServerID = state.ServiceID

	runtimeDir := filepath.Join(s.instancesDir, "svr_game", state.InstanceName)
	if err := os.MkdirAll(runtimeDir, 0755); err != nil {
		return "", fmt.Errorf("create game runtime dir failed: %w", err)
	}
	if err := s.copyGameStaticDataFiles(state.Release, runtimeDir); err != nil {
		return "", err
	}

	configPath := filepath.Join(runtimeDir, "config.json")
	data, err := json.MarshalIndent(baseCfg, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal runtime game config failed: %w", err)
	}
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return "", fmt.Errorf("write runtime game config failed: %w", err)
	}
	return configPath, nil
}

func (s *Supervisor) copyGameStaticDataFiles(release string, runtimeDir string) error {
	pattern := filepath.Join(s.releaseServiceDir(release, "svr_game"), "*.csv")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("glob game static data files failed: %w", err)
	}

	for _, sourcePath := range matches {
		data, err := os.ReadFile(sourcePath)
		if err != nil {
			return fmt.Errorf("read game static data file %s failed: %w", sourcePath, err)
		}

		targetPath := filepath.Join(runtimeDir, filepath.Base(sourcePath))
		if err := os.WriteFile(targetPath, data, 0644); err != nil {
			return fmt.Errorf("write game static data file %s failed: %w", targetPath, err)
		}
	}

	return nil
}

func (s *Supervisor) processKeyForGame(state *GameInstanceState, retiring bool) string {
	if retiring {
		return "svr_game:retiring:" + state.InstanceName
	}
	return "svr_game:active"
}

func (s *Supervisor) gatewaySpec(release string) (*managedProcess, error) {
	cfg, err := s.loadGatewayConfig(release)
	if err != nil {
		return nil, err
	}
	runtimeDir := filepath.Join(s.instancesDir, "svr_gateway")
	_ = os.MkdirAll(runtimeDir, 0755)
	return &managedProcess{
		Key:           "svr_gateway",
		ServiceName:   "svr_gateway",
		Release:       release,
		RuntimeDir:    runtimeDir,
		BinaryPath:    s.releaseBinary(release, "svr_gateway"),
		ConfigPath:    s.releaseConfig(release, "svr_gateway"),
		HealthURL:     fmt.Sprintf("http://%s:%d/readyz", s.cfg.HealthHost, cfg.GatewayPort),
		AppLogPath:    filepath.Join(s.logsDir, "svr_gateway.log"),
		StdoutLogPath: filepath.Join(s.logsDir, "svr_gateway.stdout.log"),
		Env:           []string{"IAA_GATEWAY_CONFIG_PATH=" + s.releaseConfig(release, "svr_gateway")},
		Restartable:   true,
	}, nil
}

func (s *Supervisor) loginSpec(release string) (*managedProcess, error) {
	cfg, err := s.loadLoginConfig(release)
	if err != nil {
		return nil, err
	}
	runtimeDir := filepath.Join(s.instancesDir, "svr_login")
	_ = os.MkdirAll(runtimeDir, 0755)
	return &managedProcess{
		Key:           "svr_login",
		ServiceName:   "svr_login",
		Release:       release,
		RuntimeDir:    runtimeDir,
		BinaryPath:    s.releaseBinary(release, "svr_login"),
		ConfigPath:    s.releaseConfig(release, "svr_login"),
		HealthURL:     fmt.Sprintf("http://%s:%d/readyz", s.cfg.HealthHost, cfg.LoginPort),
		AppLogPath:    filepath.Join(s.logsDir, "svr_login.log"),
		StdoutLogPath: filepath.Join(s.logsDir, "svr_login.stdout.log"),
		Env:           []string{"IAA_LOGIN_CONFIG_PATH=" + s.releaseConfig(release, "svr_login")},
		Restartable:   true,
	}, nil
}

func (s *Supervisor) gameSpec(state *GameInstanceState, retiring bool) (*managedProcess, error) {
	configPath, err := s.writeGameRuntimeConfig(state)
	if err != nil {
		return nil, err
	}
	runtimeDir := filepath.Dir(configPath)
	prefix := "svr_game.active"
	if retiring {
		prefix = "svr_game.retiring." + state.InstanceName
	}
	return &managedProcess{
		Key:           s.processKeyForGame(state, retiring),
		ServiceName:   "svr_game",
		Release:       state.Release,
		RuntimeDir:    runtimeDir,
		BinaryPath:    s.releaseBinary(state.Release, "svr_game"),
		ConfigPath:    configPath,
		MongoConfig:   s.releaseMongoConfig(state.Release),
		HealthURL:     fmt.Sprintf("http://%s:%d/readyz", s.cfg.HealthHost, state.Port),
		AppLogPath:    filepath.Join(s.logsDir, prefix+".log"),
		StdoutLogPath: filepath.Join(s.logsDir, prefix+".stdout.log"),
		Env: []string{
			"IAA_GAME_CONFIG_PATH=" + configPath,
			"IAA_MONGO_CONFIG_PATH=" + s.releaseMongoConfig(state.Release),
		},
		Restartable: true,
		Retiring:    retiring,
	}, nil
}

func (s *Supervisor) startProcessLocked(spec *managedProcess) error {
	if existing, ok := s.processes[spec.Key]; ok && existing.Cmd != nil && existing.Cmd.Process != nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(spec.AppLogPath), 0755); err != nil {
		return err
	}
	if err := os.Chmod(spec.BinaryPath, 0755); err != nil {
		return fmt.Errorf("ensure executable bit for %s failed: %w", spec.BinaryPath, err)
	}

	stdoutFile, err := os.OpenFile(spec.StdoutLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("open stdout log failed: %w", err)
	}

	cmd := exec.Command(spec.BinaryPath)
	cmd.Dir = spec.RuntimeDir
	cmd.Stdout = stdoutFile
	cmd.Stderr = stdoutFile
	cmd.Env = append(os.Environ(), spec.Env...)
	cmd.Env = append(cmd.Env, "APP_LOG_PATH="+spec.AppLogPath)

	if err := cmd.Start(); err != nil {
		_ = stdoutFile.Close()
		return fmt.Errorf("start %s failed: %w", spec.ServiceName, err)
	}
	_ = stdoutFile.Close()

	spec.Cmd = cmd
	spec.StartedAt = time.Now().UTC()
	spec.FailCount = 0
	s.processes[spec.Key] = spec

	go func(key string, waitCmd *exec.Cmd) {
		err := waitCmd.Wait()
		s.exitCh <- processExit{Key: key, Err: err}
	}(spec.Key, cmd)

	applog.Infof("process started: key=%s release=%s pid=%d", spec.Key, spec.Release, cmd.Process.Pid)
	return nil
}

func (s *Supervisor) stopProcessLocked(key string) {
	process, ok := s.processes[key]
	if !ok || process.Cmd == nil || process.Cmd.Process == nil {
		delete(s.processes, key)
		return
	}
	_ = process.Cmd.Process.Kill()
	delete(s.processes, key)
}

func (s *Supervisor) waitReady(healthURL string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		response, err := s.httpClient.Get(healthURL)
		if err == nil {
			io.Copy(io.Discard, response.Body)
			response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("health check timeout: %s", healthURL)
}

func (s *Supervisor) ensureBaseProcessesLocked() error {
	if strings.TrimSpace(s.state.BaseRelease) == "" {
		return errors.New("base release is not configured")
	}
	if err := s.ensureReleaseExists(s.state.BaseRelease); err != nil {
		return err
	}

	gatewaySpec, err := s.gatewaySpec(s.state.BaseRelease)
	if err != nil {
		return err
	}
	if err := s.startProcessLocked(gatewaySpec); err != nil {
		return err
	}

	loginSpec, err := s.loginSpec(s.state.BaseRelease)
	if err != nil {
		return err
	}
	if err := s.startProcessLocked(loginSpec); err != nil {
		return err
	}
	return nil
}

func (s *Supervisor) ensureActiveGameStateLocked() error {
	if s.state.ActiveGame != nil {
		return nil
	}
	baseCfg, err := s.loadGameConfig(s.state.BaseRelease)
	if err != nil {
		return err
	}
	serviceID := baseCfg.ServerID
	if strings.TrimSpace(serviceID) == "" {
		serviceID = s.cfg.DefaultGameServiceID
	}
	s.state.ActiveGame = &GameInstanceState{
		Release:      s.state.BaseRelease,
		ServiceID:    serviceID,
		Port:         baseCfg.GamePort,
		InstanceName: fmt.Sprintf("%s-active", s.state.BaseRelease),
	}
	return s.saveStateLocked()
}

func (s *Supervisor) ensureActiveGameProcessLocked() error {
	if err := s.ensureActiveGameStateLocked(); err != nil {
		return err
	}
	spec, err := s.gameSpec(s.state.ActiveGame, false)
	if err != nil {
		return err
	}
	return s.startProcessLocked(spec)
}

func (s *Supervisor) startAllLocked() error {
	if err := s.ensureBaseProcessesLocked(); err != nil {
		return err
	}
	if err := s.waitReady(s.processes["svr_gateway"].HealthURL, 30*time.Second); err != nil {
		s.stopAllLocked()
		return err
	}
	if err := s.waitReady(s.processes["svr_login"].HealthURL, 30*time.Second); err != nil {
		s.stopAllLocked()
		return err
	}
	if err := s.ensureActiveGameProcessLocked(); err != nil {
		s.stopAllLocked()
		return err
	}
	if err := s.waitReady(s.processes["svr_game:active"].HealthURL, 30*time.Second); err != nil {
		s.stopAllLocked()
		return err
	}
	return nil
}

func (s *Supervisor) stopAllLocked() {
	s.stoppingAll = true
	for key := range s.processes {
		s.stopProcessLocked(key)
	}
	s.stoppingAll = false
}

func (s *Supervisor) pickGamePortLocked() (int, error) {
	startPort := 8082
	if s.state.ActiveGame != nil && s.state.ActiveGame.Port >= startPort {
		startPort = s.state.ActiveGame.Port + 1
	} else if strings.TrimSpace(s.state.BaseRelease) != "" {
		cfg, err := s.loadGameConfig(s.state.BaseRelease)
		if err == nil && cfg.GamePort > 0 {
			startPort = cfg.GamePort
		}
	}

	for port := startPort; port < 65535; port++ {
		address := fmt.Sprintf("%s:%d", s.cfg.HealthHost, port)
		listener, err := net.Listen("tcp", address)
		if err == nil {
			_ = listener.Close()
			return port, nil
		}
	}
	return 0, errors.New("no available port for game cutover")
}

func (s *Supervisor) deployGameLocked(release string) error {
	if err := s.ensureReleaseExists(release); err != nil {
		return err
	}
	if strings.TrimSpace(s.state.BaseRelease) == "" {
		return errors.New("base release is not configured")
	}
	if err := s.ensureBaseProcessesLocked(); err != nil {
		return err
	}

	serviceID := s.cfg.DefaultGameServiceID
	if s.state.ActiveGame != nil && strings.TrimSpace(s.state.ActiveGame.ServiceID) != "" {
		serviceID = s.state.ActiveGame.ServiceID
	} else {
		cfg, err := s.loadGameConfig(s.state.BaseRelease)
		if err == nil && strings.TrimSpace(cfg.ServerID) != "" {
			serviceID = cfg.ServerID
		}
	}

	port, err := s.pickGamePortLocked()
	if err != nil {
		return err
	}

	next := &GameInstanceState{
		Release:      release,
		ServiceID:    serviceID,
		Port:         port,
		InstanceName: fmt.Sprintf("%s-%d", release, time.Now().Unix()),
	}
	spec, err := s.gameSpec(next, false)
	if err != nil {
		return err
	}
	tempKey := "svr_game:staging:" + next.InstanceName
	spec.Key = tempKey
	if err := s.startProcessLocked(spec); err != nil {
		return err
	}
	if err := s.waitReady(spec.HealthURL, 30*time.Second); err != nil {
		s.stopProcessLocked(tempKey)
		return err
	}

	oldActive := s.state.ActiveGame
	var oldProc *managedProcess
	if existing, ok := s.processes["svr_game:active"]; ok {
		oldProc = existing
	}
	s.state.ActiveGame = next
	if oldActive != nil && oldProc != nil {
		s.state.RetiringGames = append(s.state.RetiringGames, oldActive)
		oldProc.Key = s.processKeyForGame(oldActive, true)
		oldProc.Retiring = true
		s.processes[oldProc.Key] = oldProc
		delete(s.processes, "svr_game:active")
	}

	activeProc := s.processes[tempKey]
	delete(s.processes, tempKey)
	activeProc.Key = "svr_game:active"
	s.processes["svr_game:active"] = activeProc

	return s.saveStateLocked()
}

func (s *Supervisor) removeRetiringStateLocked(instanceName string) {
	filtered := make([]*GameInstanceState, 0, len(s.state.RetiringGames))
	for _, item := range s.state.RetiringGames {
		if item.InstanceName != instanceName {
			filtered = append(filtered, item)
		}
	}
	s.state.RetiringGames = filtered
	_ = s.saveStateLocked()
}

func (s *Supervisor) buildStatusLocked() statusResponse {
	processes := make(map[string]processBrief, len(s.processes))
	for key, process := range s.processes {
		processes[key] = processBrief{
			ServiceName: process.ServiceName,
			Release:     process.Release,
			RuntimeDir:  process.RuntimeDir,
			HealthURL:   process.HealthURL,
			Retiring:    process.Retiring,
			Running:     process.Cmd != nil && process.Cmd.Process != nil,
		}
	}
	return statusResponse{
		ControlPID: os.Getpid(),
		State:     s.state,
		Processes: processes,
	}
}

func (s *Supervisor) handleProcessExit(event processExit) {
	s.processMu.Lock()
	defer s.processMu.Unlock()

	process, ok := s.processes[event.Key]
	if !ok {
		return
	}
	delete(s.processes, event.Key)

	applog.Infof("process exited: key=%s err=%v", event.Key, event.Err)

	if process.Retiring {
		for _, item := range s.state.RetiringGames {
			if s.processKeyForGame(item, true) == event.Key {
				s.removeRetiringStateLocked(item.InstanceName)
				break
			}
		}
		return
	}
	if s.stoppingAll {
		return
	}
}

func (s *Supervisor) monitorLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(s.cfg.HealthCheckIntervalSec) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case event := <-s.exitCh:
			s.handleProcessExit(event)
		case <-ticker.C:
			s.processMu.Lock()
			s.ensureProcessesHealthyLocked()
			s.processMu.Unlock()
		}
	}
}

func (s *Supervisor) ensureProcessesHealthyLocked() {
	if s.state.BaseRelease == "" || !s.state.Running {
		return
	}

	if err := s.ensureBaseProcessesLocked(); err != nil {
		applog.Errorf("ensure base processes failed: %v", err)
	}
	if err := s.ensureActiveGameProcessLocked(); err != nil {
		applog.Errorf("ensure active game failed: %v", err)
	}

	for key, process := range s.processes {
		response, err := s.httpClient.Get(process.HealthURL)
		if err == nil {
			io.Copy(io.Discard, response.Body)
			response.Body.Close()
		}
		if err == nil && response.StatusCode == http.StatusOK {
			process.FailCount = 0
			continue
		}

		process.FailCount++
		if process.FailCount < s.cfg.HealthFailureThreshold {
			continue
		}

		if process.Retiring {
			continue
		}

		applog.Errorf("health check failed, restarting process: key=%s", key)
		s.stopProcessLocked(key)

		switch key {
		case "svr_gateway":
			if spec, err := s.gatewaySpec(s.state.BaseRelease); err == nil {
				time.Sleep(time.Duration(s.cfg.RestartBackoffSec) * time.Second)
				_ = s.startProcessLocked(spec)
			}
		case "svr_login":
			if spec, err := s.loginSpec(s.state.BaseRelease); err == nil {
				time.Sleep(time.Duration(s.cfg.RestartBackoffSec) * time.Second)
				_ = s.startProcessLocked(spec)
			}
		case "svr_game:active":
			if s.state.ActiveGame != nil {
				if spec, err := s.gameSpec(s.state.ActiveGame, false); err == nil {
					time.Sleep(time.Duration(s.cfg.RestartBackoffSec) * time.Second)
					_ = s.startProcessLocked(spec)
				}
			}
		}
	}
}

func decodeJSONBody[T any](r *http.Request, target *T) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(target)
}

func writeAPIResp(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func (s *Supervisor) bootstrapHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIResp(w, http.StatusMethodNotAllowed, apiResp{ErrMsg: "Only support POST request"})
		return
	}

	var request bootstrapRequest
	if err := decodeJSONBody(r, &request); err != nil {
		writeAPIResp(w, http.StatusBadRequest, apiResp{ErrMsg: "decode bootstrap request failed: " + err.Error()})
		return
	}

	s.processMu.Lock()
	defer s.processMu.Unlock()

	if err := s.ensureReleaseExists(request.BaseRelease); err != nil {
		writeAPIResp(w, http.StatusBadRequest, apiResp{ErrMsg: err.Error()})
		return
	}

	s.state.BaseRelease = strings.TrimSpace(request.BaseRelease)
	s.state.Running = false
	s.state.ActiveGame = nil
	s.state.RetiringGames = nil
	if err := s.ensureActiveGameStateLocked(); err != nil {
		writeAPIResp(w, http.StatusInternalServerError, apiResp{ErrMsg: err.Error()})
		return
	}
	if err := s.saveStateLocked(); err != nil {
		writeAPIResp(w, http.StatusInternalServerError, apiResp{ErrMsg: err.Error()})
		return
	}

	writeAPIResp(w, http.StatusOK, apiResp{})
}

func (s *Supervisor) startHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIResp(w, http.StatusMethodNotAllowed, apiResp{ErrMsg: "Only support POST request"})
		return
	}

	s.processMu.Lock()
	defer s.processMu.Unlock()

	if err := s.startAllLocked(); err != nil {
		s.state.Running = false
		_ = s.saveStateLocked()
		writeAPIResp(w, http.StatusInternalServerError, apiResp{ErrMsg: err.Error()})
		return
	}
	s.state.Running = true
	if err := s.saveStateLocked(); err != nil {
		s.state.Running = false
		s.stopAllLocked()
		writeAPIResp(w, http.StatusInternalServerError, apiResp{ErrMsg: err.Error()})
		return
	}
	writeAPIResp(w, http.StatusOK, apiResp{})
}

func (s *Supervisor) stopHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIResp(w, http.StatusMethodNotAllowed, apiResp{ErrMsg: "Only support POST request"})
		return
	}

	s.processMu.Lock()
	defer s.processMu.Unlock()
	s.state.Running = false
	_ = s.saveStateLocked()
	s.stopAllLocked()
	writeAPIResp(w, http.StatusOK, apiResp{})
	if s.stopControl != nil {
		go s.stopControl()
	}
}

func (s *Supervisor) statusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIResp(w, http.StatusMethodNotAllowed, apiResp{ErrMsg: "Only support GET request"})
		return
	}

	s.processMu.Lock()
	defer s.processMu.Unlock()
	writeAPIResp(w, http.StatusOK, s.buildStatusLocked())
}

func (s *Supervisor) deployGameHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIResp(w, http.StatusMethodNotAllowed, apiResp{ErrMsg: "Only support POST request"})
		return
	}

	var request deployGameRequest
	if err := decodeJSONBody(r, &request); err != nil {
		writeAPIResp(w, http.StatusBadRequest, apiResp{ErrMsg: "decode deploy request failed: " + err.Error()})
		return
	}

	s.processMu.Lock()
	defer s.processMu.Unlock()

	if err := s.deployGameLocked(strings.TrimSpace(request.Release)); err != nil {
		applog.Errorf("deploy game failed, release=%s: %v", strings.TrimSpace(request.Release), err)
		writeAPIResp(w, http.StatusInternalServerError, apiResp{ErrMsg: err.Error()})
		return
	}
	writeAPIResp(w, http.StatusOK, apiResp{})
}

func (s *Supervisor) serveControl(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/bootstrap", s.bootstrapHandler)
	mux.HandleFunc("/start", s.startHandler)
	mux.HandleFunc("/stop", s.stopHandler)
	mux.HandleFunc("/status", s.statusHandler)
	mux.HandleFunc("/deploy/game", s.deployGameHandler)
	mux.HandleFunc("/livez", func(w http.ResponseWriter, _ *http.Request) {
		writeAPIResp(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", s.cfg.ControlHost, s.cfg.ControlPort),
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	applog.Infof("svr_supervisor control listen on http://%s:%d", s.cfg.ControlHost, s.cfg.ControlPort)
	err := server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func main() {
	if err := applog.Init("svr_supervisor"); err != nil {
		fmt.Printf("init logger failed: %s\n", err.Error())
	}
	defer func() {
		if err := applog.Close(); err != nil {
			fmt.Printf("close logger failed: %s\n", err.Error())
		}
	}()
	defer applog.CatchPanic()

	cfg, err := loadSupervisorConfig(configPathFromEnv(supervisorConfigPathEnv, supervisorConfigPath))
	if err != nil {
		applog.Errorf("load config failed: %v", err)
		return
	}

	supervisor, err := newSupervisor(cfg)
	if err != nil {
		applog.Errorf("init supervisor failed: %v", err)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	supervisor.stopControl = cancel

	go supervisor.monitorLoop(ctx)

	if err := supervisor.serveControl(ctx); err != nil {
		applog.Errorf("control server failed: %v", err)
	}
}
