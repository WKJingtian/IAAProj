package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"common/applog"
	"common/config"
	cmongo "common/mongo"

	"go.mongodb.org/mongo-driver/bson"
	driverMongo "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	appConfigPath   = "config.json"
	mongoConfigPath = "mongo_config.json"
)

type AppConfig struct {
	GamePort       int    `json:"game_port"`
	MainCollection string "main_collection"
}

type APIResp struct {
	OpenID   string `json:"openid,omitempty"`
	DebugVal int64  `json:"debug_val,omitempty"`
	ErrMsg   string `json:"errMsg"`
}

type PlayerDoc struct {
	OpenID    string    `bson:"openid"`
	DebugVal  int64     `bson:"debug_val"`
	CreatedAt time.Time `bson:"created_at,omitempty"`
	UpdatedAt time.Time `bson:"updated_at,omitempty"`
}

var defaultCollectionName string = ""

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
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-OpenID")
	w.Header().Set("Content-Type", "application/json;charset=utf-8")
}

func writeJSON(w http.ResponseWriter, status int, resp APIResp) {
	setCommonHeaders(w)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}

func handleOptions(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodOptions {
		return false
	}
	setCommonHeaders(w)
	w.WriteHeader(http.StatusOK)
	return true
}

func extractOpenID(r *http.Request) (string, error) {
	openid := strings.TrimSpace(r.Header.Get("X-OpenID"))
	if openid == "" {
		return "", errors.New("missing X-OpenID header")
	}
	return openid, nil
}

func getPlayersCollection() (*driverMongo.Collection, error) {
	db, err := cmongo.Database()
	if err != nil {
		return nil, err
	}
	return db.Collection(defaultCollectionName), nil
}

func ensurePlayerIndexes(ctx context.Context) error {
	coll, err := getPlayersCollection()
	if err != nil {
		return err
	}

	indexCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err = coll.Indexes().CreateOne(indexCtx, driverMongo.IndexModel{
		Keys: bson.D{{Key: "openid", Value: 1}},
		Options: options.Index().
			SetName("idx_openid_unique").
			SetUnique(true),
	})
	if err != nil {
		return fmt.Errorf("create player index failed: %w", err)
	}
	return nil
}

func debugValHandler(w http.ResponseWriter, r *http.Request) {
	if handleOptions(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, APIResp{ErrMsg: "Only support GET request"})
		return
	}

	openid, err := extractOpenID(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, APIResp{ErrMsg: err.Error()})
		return
	}

	coll, err := getPlayersCollection()
	if err != nil {
		applog.Errorf("get players collection failed(debug_val): %v", err)
		writeJSON(w, http.StatusInternalServerError, APIResp{ErrMsg: "get players collection failed: " + err.Error()})
		return
	}

	queryCtx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	var doc PlayerDoc
	err = coll.FindOne(queryCtx, bson.M{"openid": openid}).Decode(&doc)
	if err != nil {
		if errors.Is(err, driverMongo.ErrNoDocuments) {
			writeJSON(w, http.StatusOK, APIResp{
				OpenID:   openid,
				DebugVal: 0,
				ErrMsg:   "",
			})
			return
		}
		applog.Errorf("query debug_val failed, openid=%s: %v", openid, err)
		writeJSON(w, http.StatusInternalServerError, APIResp{ErrMsg: "query debug_val failed: " + err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, APIResp{
		OpenID:   openid,
		DebugVal: doc.DebugVal,
		ErrMsg:   "",
	})
}

func debugValIncHandler(w http.ResponseWriter, r *http.Request) {
	if handleOptions(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, APIResp{ErrMsg: "Only support POST request"})
		return
	}

	openid, err := extractOpenID(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, APIResp{ErrMsg: err.Error()})
		return
	}

	coll, err := getPlayersCollection()
	if err != nil {
		applog.Errorf("get players collection failed(debug_val_inc): %v", err)
		writeJSON(w, http.StatusInternalServerError, APIResp{ErrMsg: "get players collection failed: " + err.Error()})
		return
	}

	now := time.Now().UTC()
	update := bson.M{
		"$inc": bson.M{
			"debug_val": 1,
		},
		"$set": bson.M{
			"updated_at": now,
		},
		"$setOnInsert": bson.M{
			"openid":     openid,
			"created_at": now,
		},
	}

	updateCtx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	var doc PlayerDoc
	err = coll.FindOneAndUpdate(
		updateCtx,
		bson.M{"openid": openid},
		update,
		options.FindOneAndUpdate().
			SetUpsert(true).
			SetReturnDocument(options.After),
	).Decode(&doc)
	if err != nil {
		applog.Errorf("increment debug_val failed, openid=%s: %v", openid, err)
		writeJSON(w, http.StatusInternalServerError, APIResp{ErrMsg: "increment debug_val failed: " + err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, APIResp{
		OpenID:   openid,
		DebugVal: doc.DebugVal,
		ErrMsg:   "",
	})
}

func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	if handleOptions(w, r) {
		return
	}
	writeJSON(w, http.StatusNotFound, APIResp{ErrMsg: "route not found"})
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

	cfg, err := loadAppConfig(appConfigPath)
	if err != nil {
		applog.Errorf("load app config failed: %v", err)
		time.Sleep(1000000)
		return
	}

	defaultCollectionName = cfg.MainCollection
	listenAddr := fmt.Sprintf(":%d", cfg.GamePort)

	if _, err := cmongo.InitFromJSON(context.Background(), mongoConfigPath); err != nil {
		applog.Errorf("init mongo failed: %v", err)
		time.Sleep(1000000)
		return
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = cmongo.Disconnect(ctx)
	}()

	if err := ensurePlayerIndexes(context.Background()); err != nil {
		applog.Errorf("ensure player indexes failed: %v", err)
		time.Sleep(1000000)
		return
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/debug_val", debugValHandler)
	mux.HandleFunc("/debug_val_inc", debugValIncHandler)
	mux.HandleFunc("/", notFoundHandler)

	applog.Infof("svr_game start, will listen to: http://0.0.0.0%s", listenAddr)
	if err := http.ListenAndServe(listenAddr, mux); err != nil {
		applog.Errorf("server init failed: %v", err)
	}
}
