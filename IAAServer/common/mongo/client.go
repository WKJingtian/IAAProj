package mongo

import (
	"common/config"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	driverMongo "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

var ErrNotInitialized = errors.New("mongo client is not initialized")

type Config struct {
	URI                      string `json:"uri"`
	Database                 string `json:"database"`
	User                     string `json:"user"`
	Pwd                      string `json:"pwd"`
	AuthSource               string `json:"auth_source"`
	AuthMechanism            string `json:"auth_mechanism"`
	AppName                  string `json:"app_name"`
	MaxPoolSize              uint64 `json:"max_pool_size"`
	MinPoolSize              uint64 `json:"min_pool_size"`
	MaxConnecting            uint64 `json:"max_connecting"`
	MaxConnIdleMS            int64  `json:"max_conn_idle_ms"`
	ConnectTimeoutMS         int64  `json:"connect_timeout_ms"`
	ServerSelectionTimeoutMS int64  `json:"server_selection_timeout_ms"`
	SocketTimeoutMS          int64  `json:"socket_timeout_ms"`
	PingTimeoutMS            int64  `json:"ping_timeout_ms"`
}

type managerState struct {
	mu        sync.RWMutex
	client    *driverMongo.Client
	defaultDB string
}

var global managerState

func InitFromJSON(ctx context.Context, configPath string) (*driverMongo.Client, error) {
	var cfg Config
	if err := config.LoadJSONConfig(configPath, &cfg); err != nil {
		return nil, fmt.Errorf("load mongo config failed: %w", err)
	}
	return Init(ctx, cfg)
}

func Init(ctx context.Context, cfg Config) (*driverMongo.Client, error) {
	if client, ok := currentClient(); ok {
		return client, nil
	}

	cfg, err := normalizeConfig(cfg)
	if err != nil {
		return nil, err
	}

	global.mu.Lock()
	defer global.mu.Unlock()

	if global.client != nil {
		return global.client, nil
	}

	clientOpts := options.Client().
		ApplyURI(cfg.URI).
		SetAppName(cfg.AppName).
		SetRetryWrites(true).
		SetRetryReads(true).
		SetMaxPoolSize(cfg.MaxPoolSize).
		SetMinPoolSize(cfg.MinPoolSize).
		SetMaxConnecting(cfg.MaxConnecting).
		SetMaxConnIdleTime(time.Duration(cfg.MaxConnIdleMS) * time.Millisecond).
		SetConnectTimeout(time.Duration(cfg.ConnectTimeoutMS) * time.Millisecond).
		SetServerSelectionTimeout(time.Duration(cfg.ServerSelectionTimeoutMS) * time.Millisecond).
		SetSocketTimeout(time.Duration(cfg.SocketTimeoutMS) * time.Millisecond)
	if cfg.User != "" {
		clientOpts.SetAuth(buildCredential(cfg))
	}

	client, err := connectAndPing(ctx, cfg, clientOpts)
	if err != nil {
		if shouldRetryWithSCRAMSHA256(cfg, err) {
			retryOpts := cloneClientOptions(clientOpts)
			retryCfg := cfg
			retryCfg.AuthMechanism = "SCRAM-SHA-256"
			retryOpts.SetAuth(buildCredential(retryCfg))
			client, err = connectAndPing(ctx, cfg, retryOpts)
		}
		if err != nil {
			return nil, err
		}
	}

	global.client = client
	global.defaultDB = cfg.Database
	return client, nil
}

func Client() (*driverMongo.Client, error) {
	if client, ok := currentClient(); ok {
		return client, nil
	}
	return nil, ErrNotInitialized
}

func Database(name ...string) (*driverMongo.Database, error) {
	client, err := Client()
	if err != nil {
		return nil, err
	}

	dbName := ""
	if len(name) > 0 {
		dbName = strings.TrimSpace(name[0])
	}
	if dbName == "" {
		global.mu.RLock()
		dbName = global.defaultDB
		global.mu.RUnlock()
	}
	if dbName == "" {
		return nil, errors.New("database name cannot be empty")
	}
	return client.Database(dbName), nil
}

func Disconnect(ctx context.Context) error {
	global.mu.Lock()
	client := global.client
	global.client = nil
	global.defaultDB = ""
	global.mu.Unlock()

	if client == nil {
		return nil
	}
	return client.Disconnect(defaultContext(ctx))
}

func currentClient() (*driverMongo.Client, bool) {
	global.mu.RLock()
	defer global.mu.RUnlock()
	if global.client == nil {
		return nil, false
	}
	return global.client, true
}

func defaultContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func normalizeConfig(cfg Config) (Config, error) {
	cfg.URI = strings.TrimSpace(cfg.URI)
	cfg.Database = strings.TrimSpace(cfg.Database)
	cfg.User = strings.TrimSpace(cfg.User)
	cfg.Pwd = strings.TrimSpace(cfg.Pwd)
	cfg.AuthSource = strings.TrimSpace(cfg.AuthSource)
	cfg.AuthMechanism = strings.TrimSpace(cfg.AuthMechanism)
	cfg.AppName = strings.TrimSpace(cfg.AppName)

	if cfg.URI == "" {
		return Config{}, errors.New("mongo uri cannot be empty")
	}
	if (cfg.User == "" && cfg.Pwd != "") || (cfg.User != "" && cfg.Pwd == "") {
		return Config{}, errors.New("mongo user and pwd must be provided together")
	}
	if cfg.AppName == "" {
		cfg.AppName = "iaa-service"
	}
	if cfg.MaxPoolSize == 0 {
		cfg.MaxPoolSize = 300
	}
	if cfg.MinPoolSize == 0 {
		cfg.MinPoolSize = 20
	}
	if cfg.MaxConnecting == 0 {
		cfg.MaxConnecting = 32
	}
	if cfg.MaxConnIdleMS <= 0 {
		cfg.MaxConnIdleMS = 120000
	}
	if cfg.ConnectTimeoutMS <= 0 {
		cfg.ConnectTimeoutMS = 5000
	}
	if cfg.ServerSelectionTimeoutMS <= 0 {
		cfg.ServerSelectionTimeoutMS = 5000
	}
	if cfg.SocketTimeoutMS <= 0 {
		cfg.SocketTimeoutMS = 30000
	}
	if cfg.PingTimeoutMS <= 0 {
		cfg.PingTimeoutMS = 3000
	}
	if cfg.MinPoolSize > cfg.MaxPoolSize {
		return Config{}, errors.New("min_pool_size cannot be greater than max_pool_size")
	}
	return cfg, nil
}

func buildCredential(cfg Config) options.Credential {
	cred := options.Credential{
		Username: cfg.User,
		Password: cfg.Pwd,
	}
	if cfg.AuthSource != "" {
		cred.AuthSource = cfg.AuthSource
	}
	if cfg.AuthMechanism != "" {
		cred.AuthMechanism = cfg.AuthMechanism
	}
	return cred
}

func cloneClientOptions(base *options.ClientOptions) *options.ClientOptions {
	clone := options.Client()
	if base != nil {
		*clone = *base
	}
	return clone
}

func connectAndPing(ctx context.Context, cfg Config, clientOpts *options.ClientOptions) (*driverMongo.Client, error) {
	client, err := driverMongo.Connect(defaultContext(ctx), clientOpts)
	if err != nil {
		return nil, fmt.Errorf("connect mongo failed: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(defaultContext(ctx), time.Duration(cfg.PingTimeoutMS)*time.Millisecond)
	defer cancel()
	if err := client.Ping(pingCtx, readpref.Primary()); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, fmt.Errorf("ping mongo failed: %w", err)
	}

	return client, nil
}

func shouldRetryWithSCRAMSHA256(cfg Config, err error) bool {
	if cfg.User == "" || cfg.AuthMechanism != "" || err == nil {
		return false
	}
	return strings.Contains(err.Error(), `unable to authenticate using mechanism "SCRAM-SHA-1"`)
}
