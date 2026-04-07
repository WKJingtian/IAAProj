package httpapi

import (
	"context"
	"net/http"
	"time"

	"common/applog"
	"common/errorcode"
	"svr_game/game"
)

type Handler struct {
	service *game.Service
}

func NewHandler(service *game.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/player_data", h.playerDataHandler)
	mux.HandleFunc("/room_data", h.roomDataHandler)
	mux.HandleFunc("/debug_val", h.debugValHandler)
	mux.HandleFunc("/debug_val_inc", h.debugValIncHandler)
	mux.HandleFunc("/trigger_event", h.eventTriggerHandler)
	mux.HandleFunc("/upgrade_furniture", h.upgradeFurnitureHandler)
	mux.HandleFunc("/", h.notFoundHandler)
}

func (h *Handler) playerDataHandler(w http.ResponseWriter, r *http.Request) {
	req, ok := preparePlayerRequest(w, r, http.MethodPost)
	if !ok {
		return
	}

	queryCtx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	data, err := h.service.GetOrCreatePlayerData(queryCtx, req.OpenID)
	if err != nil {
		applog.Errorf("get player_data failed, openid=%s: %v", req.OpenID, err)
		writePlayerDataError(w, http.StatusInternalServerError, errorcode.InternalError)
		return
	}

	writePlayerDataJSON(w, http.StatusOK, newPlayerDataReply(data))
}

func (h *Handler) debugValHandler(w http.ResponseWriter, r *http.Request) {
	req, ok := preparePlayerRequest(w, r, http.MethodGet)
	if !ok {
		return
	}

	queryCtx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	doc, err := h.service.GetPlayerData(queryCtx, req.OpenID)
	if err != nil {
		applog.Errorf("query debug_val failed, openid=%s: %v", req.OpenID, err)
		writeDefaultValError(w, http.StatusInternalServerError, errorcode.InternalError)
		return
	}

	writeDefaultValJSON(w, http.StatusOK, newDefaultValReply(doc))
}

func (h *Handler) debugValIncHandler(w http.ResponseWriter, r *http.Request) {
	req, ok := preparePlayerRequest(w, r, http.MethodPost)
	if !ok {
		return
	}

	updateCtx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	doc, err := h.service.IncrementDebugVal(updateCtx, req.OpenID)
	if err != nil {
		applog.Errorf("increment debug_val failed, openid=%s: %v", req.OpenID, err)
		writeDefaultValError(w, http.StatusInternalServerError, errorcode.InternalError)
		return
	}

	writeDefaultValJSON(w, http.StatusOK, newDefaultValReply(doc))
}

func (h *Handler) eventTriggerHandler(w http.ResponseWriter, r *http.Request) {
	req, ok := prepareTriggerEventRequest(w, r)
	if !ok {
		return
	}

	queryCtx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	historyDelta, pirateTargetPlayerIDs, data, err := h.service.TriggerEvent(queryCtx, req.OpenID, req.Multiplier)
	if err != nil {
		status := http.StatusInternalServerError
		code := triggerEventErrorCode(err)
		if code == errorcode.TriggerEventInsufficientEnergy {
			status = http.StatusBadRequest
		}
		writeTriggerEventError(w, status, code)
		return
	}

	writeTriggerEventJSON(w, http.StatusOK, newTriggerEventReply(historyDelta, pirateTargetPlayerIDs, data))
}

func (h *Handler) roomDataHandler(w http.ResponseWriter, r *http.Request) {
	req, ok := preparePlayerRequest(w, r, http.MethodPost)
	if !ok {
		return
	}

	queryCtx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	snapshot, err := h.service.GetRoomData(queryCtx, req.OpenID)
	if err != nil {
		applog.Errorf("get room_data failed, openid=%s: %v", req.OpenID, err)
		writeRoomDataError(w, http.StatusInternalServerError, errorcode.InternalError)
		return
	}

	writeRoomDataJSON(w, http.StatusOK, newRoomDataReply(snapshot))
}

func (h *Handler) upgradeFurnitureHandler(w http.ResponseWriter, r *http.Request) {
	req, ok := prepareUpgradeFurnitureRequest(w, r)
	if !ok {
		return
	}

	queryCtx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	snapshot, playerData, err := h.service.UpgradeFurniture(queryCtx, req.OpenID, req.FurnitureID)
	if err != nil {
		code := upgradeFurnitureErrorCode(err)
		status := http.StatusInternalServerError
		if code != errorcode.InternalError {
			status = http.StatusBadRequest
		}
		writeUpgradeFurnitureError(w, status, code)
		return
	}

	writeUpgradeFurnitureJSON(w, http.StatusOK, newUpgradeFurnitureReply(snapshot, playerData))
}

func (h *Handler) notFoundHandler(w http.ResponseWriter, r *http.Request) {
	if handleOptions(w, r) {
		return
	}

	writeDefaultValError(w, http.StatusNotFound, errorcode.RouteNotFound)
}
