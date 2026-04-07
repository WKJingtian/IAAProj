package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	mrand "math/rand"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Type string

const (
	TypeLogin Type = "login"
	TypeGame  Type = "game"
)

type LeaseState string

const (
	LeaseStateActive     LeaseState = "active"
	LeaseStateSuperseded LeaseState = "superseded"
	LeaseStateUnknown    LeaseState = "unknown"
)

type Instance struct {
	Type Type   `json:"type"`
	ID   string `json:"id"`
	Host string `json:"host"`
	Port int    `json:"port"`
}

type RegisterResponse struct {
	ErrMsg            string `json:"err_msg"`
	Replaced          bool   `json:"replaced"`
	LeaseID           string `json:"lease_id"`
	SupersedeDelaySec int    `json:"supersede_delay_sec"`
}

type HeartbeatRequest struct {
	Type    Type   `json:"type"`
	ID      string `json:"id"`
	LeaseID string `json:"lease_id"`
}

type HeartbeatResponse struct {
	ErrMsg            string     `json:"err_msg"`
	State             LeaseState `json:"state"`
	LeaseID           string     `json:"lease_id,omitempty"`
	TerminateAfterSec int        `json:"terminate_after_sec,omitempty"`
}

type Lease struct {
	Instance         Instance
	LeaseID          string
	RegisteredAt     time.Time
	LastHeartbeatAt  time.Time
	SupersededAt     time.Time
	TerminateAfterAt time.Time
}

type serviceSlot struct {
	Active   Lease
	Retiring map[string]Lease
}

type Registry struct {
	mu             sync.Mutex
	byType         map[Type]map[string]*serviceSlot
	rng            *mrand.Rand
	leaseTimeout   time.Duration
	supersedeDelay time.Duration
}

func NormalizeType(raw string) (Type, error) {
	value := Type(strings.ToLower(strings.TrimSpace(raw)))
	switch value {
	case TypeLogin, TypeGame:
		return value, nil
	default:
		return "", fmt.Errorf("unsupported service type: %q", raw)
	}
}

func (i Instance) Validate() error {
	if _, err := NormalizeType(string(i.Type)); err != nil {
		return err
	}
	if strings.TrimSpace(i.ID) == "" {
		return errors.New("service id cannot be empty")
	}
	if strings.TrimSpace(i.Host) == "" {
		return errors.New("service host cannot be empty")
	}
	if i.Port <= 0 || i.Port > 65535 {
		return errors.New("service port must be in range 1-65535")
	}
	return nil
}

func (i Instance) Normalized() (Instance, error) {
	if err := i.Validate(); err != nil {
		return Instance{}, err
	}
	serviceType, _ := NormalizeType(string(i.Type))
	i.Type = serviceType
	i.ID = strings.TrimSpace(i.ID)
	i.Host = strings.TrimSpace(i.Host)
	return i, nil
}

func (i Instance) BaseURL() string {
	return fmt.Sprintf("http://%s:%d", i.Host, i.Port)
}

func (h HeartbeatRequest) Normalized() (HeartbeatRequest, error) {
	serviceType, err := NormalizeType(string(h.Type))
	if err != nil {
		return HeartbeatRequest{}, err
	}
	h.Type = serviceType
	h.ID = strings.TrimSpace(h.ID)
	h.LeaseID = strings.TrimSpace(h.LeaseID)
	if h.ID == "" {
		return HeartbeatRequest{}, errors.New("heartbeat id cannot be empty")
	}
	if h.LeaseID == "" {
		return HeartbeatRequest{}, errors.New("heartbeat lease_id cannot be empty")
	}
	return h, nil
}

func NewRegistry(leaseTimeout time.Duration, supersedeDelay time.Duration) *Registry {
	if leaseTimeout <= 0 {
		leaseTimeout = 10 * time.Second
	}
	if supersedeDelay <= 0 {
		supersedeDelay = 30 * time.Second
	}

	return &Registry{
		byType:         make(map[Type]map[string]*serviceSlot),
		rng:            mrand.New(mrand.NewSource(time.Now().UnixNano())),
		leaseTimeout:   leaseTimeout,
		supersedeDelay: supersedeDelay,
	}
}

func (r *Registry) Upsert(instance Instance) (RegisterResponse, error) {
	normalized, err := instance.Normalized()
	if err != nil {
		return RegisterResponse{}, err
	}

	now := time.Now().UTC()
	leaseID, err := newLeaseID()
	if err != nil {
		return RegisterResponse{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.cleanupExpiredLocked(now)

	typeEntries, ok := r.byType[normalized.Type]
	if !ok {
		typeEntries = make(map[string]*serviceSlot)
		r.byType[normalized.Type] = typeEntries
	}

	slot, ok := typeEntries[normalized.ID]
	if !ok {
		slot = &serviceSlot{Retiring: make(map[string]Lease)}
		typeEntries[normalized.ID] = slot
	}

	replaced := slot.Active.LeaseID != ""
	if replaced {
		previous := slot.Active
		previous.SupersededAt = now
		previous.TerminateAfterAt = now.Add(r.supersedeDelay)
		slot.Retiring[previous.LeaseID] = previous
	}

	slot.Active = Lease{
		Instance:        normalized,
		LeaseID:         leaseID,
		RegisteredAt:    now,
		LastHeartbeatAt: now,
	}

	return RegisterResponse{
		ErrMsg:            "",
		Replaced:          replaced,
		LeaseID:           leaseID,
		SupersedeDelaySec: int(r.supersedeDelay / time.Second),
	}, nil
}

func (r *Registry) Heartbeat(request HeartbeatRequest) (HeartbeatResponse, error) {
	normalized, err := request.Normalized()
	if err != nil {
		return HeartbeatResponse{}, err
	}

	now := time.Now().UTC()

	r.mu.Lock()
	defer r.mu.Unlock()

	r.cleanupExpiredLocked(now)

	typeEntries, ok := r.byType[normalized.Type]
	if !ok {
		return HeartbeatResponse{State: LeaseStateUnknown}, nil
	}
	slot, ok := typeEntries[normalized.ID]
	if !ok {
		return HeartbeatResponse{State: LeaseStateUnknown}, nil
	}

	if slot.Active.LeaseID == normalized.LeaseID {
		slot.Active.LastHeartbeatAt = now
		return HeartbeatResponse{
			ErrMsg:  "",
			State:   LeaseStateActive,
			LeaseID: normalized.LeaseID,
		}, nil
	}

	if retiring, ok := slot.Retiring[normalized.LeaseID]; ok {
		remaining := int(time.Until(retiring.TerminateAfterAt).Seconds())
		if remaining < 0 {
			remaining = 0
		}
		return HeartbeatResponse{
			ErrMsg:            "",
			State:             LeaseStateSuperseded,
			LeaseID:           normalized.LeaseID,
			TerminateAfterSec: remaining,
		}, nil
	}

	return HeartbeatResponse{State: LeaseStateUnknown}, nil
}

func (r *Registry) RandomByType(serviceType Type) (Instance, bool) {
	normalizedType, err := NormalizeType(string(serviceType))
	if err != nil {
		return Instance{}, false
	}

	now := time.Now().UTC()

	r.mu.Lock()
	defer r.mu.Unlock()

	r.cleanupExpiredLocked(now)

	typeEntries, ok := r.byType[normalizedType]
	if !ok || len(typeEntries) == 0 {
		return Instance{}, false
	}

	instances := make([]Instance, 0, len(typeEntries))
	for _, slot := range typeEntries {
		if slot.Active.LeaseID == "" {
			continue
		}
		if now.Sub(slot.Active.LastHeartbeatAt) > r.leaseTimeout {
			continue
		}
		instances = append(instances, slot.Active.Instance)
	}
	if len(instances) == 0 {
		return Instance{}, false
	}

	index := r.rng.Intn(len(instances))
	return instances[index], true
}

func (r *Registry) GetByTypeAndID(serviceType Type, serviceID string) (Instance, bool) {
	normalizedType, err := NormalizeType(string(serviceType))
	if err != nil {
		return Instance{}, false
	}

	normalizedID := strings.TrimSpace(serviceID)
	if normalizedID == "" {
		return Instance{}, false
	}

	now := time.Now().UTC()

	r.mu.Lock()
	defer r.mu.Unlock()

	r.cleanupExpiredLocked(now)

	typeEntries, ok := r.byType[normalizedType]
	if !ok {
		return Instance{}, false
	}

	slot, ok := typeEntries[normalizedID]
	if !ok {
		return Instance{}, false
	}

	if slot.Active.LeaseID == "" {
		return Instance{}, false
	}
	if now.Sub(slot.Active.LastHeartbeatAt) > r.leaseTimeout {
		return Instance{}, false
	}

	return slot.Active.Instance, true
}

func (r *Registry) cleanupExpiredLocked(now time.Time) {
	for serviceType, typeEntries := range r.byType {
		for serviceID, slot := range typeEntries {
			for leaseID, retiring := range slot.Retiring {
				if now.After(retiring.TerminateAfterAt.Add(r.leaseTimeout)) {
					delete(slot.Retiring, leaseID)
				}
			}

			if slot.Active.LeaseID != "" && now.Sub(slot.Active.LastHeartbeatAt) > r.leaseTimeout {
				slot.Active = Lease{}
			}

			if slot.Active.LeaseID == "" && len(slot.Retiring) == 0 {
				delete(typeEntries, serviceID)
			}
		}
		if len(typeEntries) == 0 {
			delete(r.byType, serviceType)
		}
	}
}

func newLeaseID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate lease id failed: %w", err)
	}
	return hex.EncodeToString(raw[:]), nil
}

func Register(ctx context.Context, client *http.Client, registerURL string, instance Instance) (RegisterResponse, error) {
	if strings.TrimSpace(registerURL) == "" {
		return RegisterResponse{}, errors.New("register url cannot be empty")
	}

	normalized, err := instance.Normalized()
	if err != nil {
		return RegisterResponse{}, err
	}

	body, err := json.Marshal(normalized)
	if err != nil {
		return RegisterResponse{}, fmt.Errorf("marshal register request failed: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, registerURL, bytes.NewReader(body))
	if err != nil {
		return RegisterResponse{}, fmt.Errorf("create register request failed: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")

	httpClient := client
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 3 * time.Second}
	}

	response, err := httpClient.Do(request)
	if err != nil {
		return RegisterResponse{}, fmt.Errorf("register request failed: %w", err)
	}
	defer response.Body.Close()

	var registerResp RegisterResponse
	_ = json.NewDecoder(response.Body).Decode(&registerResp)
	if response.StatusCode != http.StatusOK {
		if strings.TrimSpace(registerResp.ErrMsg) != "" {
			return RegisterResponse{}, fmt.Errorf("register failed: %s", registerResp.ErrMsg)
		}
		return RegisterResponse{}, fmt.Errorf("register failed with status %d", response.StatusCode)
	}
	if strings.TrimSpace(registerResp.ErrMsg) != "" {
		return RegisterResponse{}, fmt.Errorf("register failed: %s", registerResp.ErrMsg)
	}
	if strings.TrimSpace(registerResp.LeaseID) == "" {
		return RegisterResponse{}, errors.New("register response missing lease_id")
	}
	return registerResp, nil
}

func Heartbeat(ctx context.Context, client *http.Client, heartbeatURL string, request HeartbeatRequest) (HeartbeatResponse, error) {
	if strings.TrimSpace(heartbeatURL) == "" {
		return HeartbeatResponse{}, errors.New("heartbeat url cannot be empty")
	}

	normalized, err := request.Normalized()
	if err != nil {
		return HeartbeatResponse{}, err
	}

	body, err := json.Marshal(normalized)
	if err != nil {
		return HeartbeatResponse{}, fmt.Errorf("marshal heartbeat request failed: %w", err)
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, heartbeatURL, bytes.NewReader(body))
	if err != nil {
		return HeartbeatResponse{}, fmt.Errorf("create heartbeat request failed: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")

	httpClient := client
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 3 * time.Second}
	}

	response, err := httpClient.Do(httpRequest)
	if err != nil {
		return HeartbeatResponse{}, fmt.Errorf("heartbeat request failed: %w", err)
	}
	defer response.Body.Close()

	var heartbeatResp HeartbeatResponse
	_ = json.NewDecoder(response.Body).Decode(&heartbeatResp)
	if response.StatusCode != http.StatusOK {
		if strings.TrimSpace(heartbeatResp.ErrMsg) != "" {
			return HeartbeatResponse{}, fmt.Errorf("heartbeat failed: %s", heartbeatResp.ErrMsg)
		}
		return HeartbeatResponse{}, fmt.Errorf("heartbeat failed with status %d", response.StatusCode)
	}
	if strings.TrimSpace(heartbeatResp.ErrMsg) != "" {
		return HeartbeatResponse{}, fmt.Errorf("heartbeat failed: %s", heartbeatResp.ErrMsg)
	}
	return heartbeatResp, nil
}
