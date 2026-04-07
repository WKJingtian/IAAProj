package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"common/errorcode"
)

type playerRequest struct {
	OpenID string
}

type triggerEventRequest struct {
	OpenID     string
	Multiplier int
}

type upgradeFurnitureRequest struct {
	OpenID      string
	FurnitureID int
}

func extractOpenID(r *http.Request) (string, error) {
	openid := strings.TrimSpace(r.Header.Get("X-OpenID"))
	if openid == "" {
		return "", errors.New("missing X-OpenID header")
	}
	return openid, nil
}

func setCommonHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-OpenID")
	w.Header().Set("Content-Type", "application/json;charset=utf-8")
}

func handleOptions(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodOptions {
		return false
	}
	setCommonHeaders(w)
	w.WriteHeader(http.StatusOK)
	return true
}

func preparePlayerRequest(w http.ResponseWriter, r *http.Request, method string) (*playerRequest, bool) {
	if handleOptions(w, r) {
		return nil, false
	}
	if r.Method != method {
		writeDefaultValError(w, http.StatusMethodNotAllowed, errorcode.InvalidMethod)
		return nil, false
	}

	openid, err := extractOpenID(r)
	if err != nil {
		writeDefaultValError(w, http.StatusUnauthorized, errorcode.AuthMissingHeader)
		return nil, false
	}

	return &playerRequest{OpenID: openid}, true
}

func prepareTriggerEventRequest(w http.ResponseWriter, r *http.Request) (*triggerEventRequest, bool) {
	req, ok := preparePlayerRequest(w, r, http.MethodPost)
	if !ok {
		return nil, false
	}

	result := &triggerEventRequest{
		OpenID:     req.OpenID,
		Multiplier: 1,
	}

	if r.Body == nil {
		return result, true
	}

	defer r.Body.Close()

	type triggerEventPayload struct {
		Multiplier *int `json:"multiplier"`
	}

	var payload triggerEventPayload
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		if errors.Is(err, io.EOF) {
			return result, true
		}
		writeTriggerEventError(w, http.StatusBadRequest, errorcode.TriggerEventPayloadInvalid)
		return nil, false
	}
	if err := decoder.Decode(&struct{}{}); err != nil && !errors.Is(err, io.EOF) {
		writeTriggerEventError(w, http.StatusBadRequest, errorcode.TriggerEventPayloadInvalid)
		return nil, false
	}

	if payload.Multiplier != nil {
		if *payload.Multiplier <= 0 {
			writeTriggerEventError(w, http.StatusBadRequest, errorcode.TriggerEventMultiplierInvalid)
			return nil, false
		}
		result.Multiplier = *payload.Multiplier
	}

	return result, true
}

func prepareUpgradeFurnitureRequest(w http.ResponseWriter, r *http.Request) (*upgradeFurnitureRequest, bool) {
	req, ok := preparePlayerRequest(w, r, http.MethodPost)
	if !ok {
		return nil, false
	}
	if r.Body == nil {
		writeUpgradeFurnitureError(w, http.StatusBadRequest, errorcode.UpgradeFurniturePayloadInvalid)
		return nil, false
	}

	defer r.Body.Close()

	type upgradeFurniturePayload struct {
		FurnitureID *int `json:"furniture_id"`
	}

	var payload upgradeFurniturePayload
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		if errors.Is(err, io.EOF) {
			writeUpgradeFurnitureError(w, http.StatusBadRequest, errorcode.UpgradeFurniturePayloadInvalid)
			return nil, false
		}
		writeUpgradeFurnitureError(w, http.StatusBadRequest, errorcode.UpgradeFurniturePayloadInvalid)
		return nil, false
	}
	if err := decoder.Decode(&struct{}{}); err != nil && !errors.Is(err, io.EOF) {
		writeUpgradeFurnitureError(w, http.StatusBadRequest, errorcode.UpgradeFurniturePayloadInvalid)
		return nil, false
	}
	if payload.FurnitureID == nil {
		writeUpgradeFurnitureError(w, http.StatusBadRequest, errorcode.UpgradeFurnitureFurnitureIDRequired)
		return nil, false
	}
	if *payload.FurnitureID < 0 {
		writeUpgradeFurnitureError(w, http.StatusBadRequest, errorcode.UpgradeFurnitureFurnitureIDInvalid)
		return nil, false
	}

	return &upgradeFurnitureRequest{
		OpenID:      req.OpenID,
		FurnitureID: *payload.FurnitureID,
	}, true
}
