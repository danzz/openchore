package api

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/liftedkilt/openchore/internal/model"
	"github.com/liftedkilt/openchore/internal/store"
)

type KioskHandler struct {
	store *store.Store
}

func NewKioskHandler(s *store.Store) *KioskHandler {
	return &KioskHandler{store: s}
}

type pairClaimRequest struct {
	Code        string `json:"code"`
	DeviceName  string `json:"device_name"`
	HouseholdID int64  `json:"household_id"`
}

func generateRandomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func generatePairCode() string {
	n, err := rand.Int(rand.Reader, big.NewInt(900000))
	if err != nil {
		return "123456"
	}
	return fmt.Sprintf("%06d", n.Int64()+100000)
}

// PairInit is called by an un-paired wall tablet to request a 6-digit pairing code.
func (h *KioskHandler) PairInit(w http.ResponseWriter, r *http.Request) {
	code := generatePairCode()
	initToken := "init_" + generateRandomHex(16)
	exp := time.Now().UTC().Add(10 * time.Minute)

	dev := &model.HouseholdDevice{
		HouseholdID:       1, // placeholder until claimed
		DeviceToken:       initToken,
		DeviceName:        "Wall Tablet",
		PairCode:          code,
		PairCodeExpiresAt: &exp,
	}

	if err := h.store.CreateHouseholdDevice(r.Context(), dev); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to initialize pairing")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"pair_code":    code,
		"expires_in":   600,
		"device_token": initToken,
	})
}

// PairPoll is polled by the tablet waiting for a parent to claim the pair code.
func (h *KioskHandler) PairPoll(w http.ResponseWriter, r *http.Request) {
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	token := strings.TrimSpace(r.URL.Query().Get("token"))

	var dev *model.HouseholdDevice
	var err error

	if code != "" {
		dev, err = h.store.GetHouseholdDeviceByPairCode(r.Context(), code)
	} else if token != "" {
		dev, err = h.store.GetHouseholdDeviceByToken(r.Context(), token)
	} else {
		writeError(w, http.StatusBadRequest, "code or token parameter required")
		return
	}

	if err != nil || dev == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"paired":  false,
			"expired": true,
		})
		return
	}

	if dev.PairedAt != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"paired":       true,
			"device_token": dev.DeviceToken,
			"household_id": dev.HouseholdID,
			"device_name":  dev.DeviceName,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"paired":  false,
		"expired": false,
	})
}

// PairClaim is called by an authenticated parent to approve a tablet's pairing code.
func (h *KioskHandler) PairClaim(w http.ResponseWriter, r *http.Request) {
	var req pairClaimRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Code = strings.TrimSpace(req.Code)
	if req.Code == "" {
		writeError(w, http.StatusBadRequest, "pairing code is required")
		return
	}

	householdID := req.HouseholdID
	if householdID <= 0 {
		householdID = HouseholdIDFromContext(r.Context())
	}
	if req.DeviceName == "" {
		req.DeviceName = "Wall Tablet"
	}

	permToken := "ocd_" + generateRandomHex(32)
	claimed, err := h.store.ClaimHouseholdDevice(r.Context(), req.Code, householdID, permToken, req.DeviceName)
	if err != nil {
		if err == store.ErrHouseholdDeviceNotFound {
			writeError(w, http.StatusNotFound, "pairing code not found or already claimed")
			return
		}
		if err == store.ErrPairCodeExpired {
			writeError(w, http.StatusBadRequest, "pairing code has expired")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to claim device")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"device":  claimed,
	})
}

// ListDevices returns all paired wall tablet devices for the household.
func (h *KioskHandler) ListDevices(w http.ResponseWriter, r *http.Request) {
	householdID := HouseholdIDFromContext(r.Context())
	devices, err := h.store.ListHouseholdDevices(r.Context(), householdID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list devices")
		return
	}
	if devices == nil {
		devices = []model.HouseholdDevice{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"devices": devices,
	})
}

// DeleteDevice revokes a paired wall tablet device.
func (h *KioskHandler) DeleteDevice(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid device id")
		return
	}

	householdID := HouseholdIDFromContext(r.Context())
	if err := h.store.DeleteHouseholdDevice(r.Context(), id, householdID); err != nil {
		if err == store.ErrHouseholdDeviceNotFound {
			writeError(w, http.StatusNotFound, "device not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete device")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
