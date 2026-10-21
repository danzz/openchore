package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/liftedkilt/openchore/internal/model"
	"github.com/liftedkilt/openchore/internal/store"
)

const (
	householdContextKey     contextKey = "household_id"
	householdDeviceContextKey contextKey = "household_device"
)

// ResolveHouseholdMiddleware attaches the resolved household_id and optional
// household_device to the request context.
// Resolution order:
// 1. User Session (from Cookie or Bearer token) -> scopes to user's HouseholdID.
// 2. Device Token header (X-Device-Token) from paired kiosk / wall tablet.
// 3. X-Household-ID header or ?household_id= query param (explicit selection).
// 4. Fallback default household ID 1 (for single-family compatibility).
func ResolveHouseholdMiddleware(s *store.Store, sm *SessionManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			var resolvedHouseholdID int64
			var matchedDevice *model.HouseholdDevice

			// 1. Check if user is logged in via session cookie / token
			token := sessionTokenFromRequest(r)
			if token != "" && sm != nil {
				if user, claims, err := loadSessionUser(ctx, s, sm, token); err == nil && user != nil {
					ctx = context.WithValue(ctx, userContextKey, user)
					ctx = context.WithValue(ctx, sessionClaimsContextKey, claims)
					resolvedHouseholdID = user.HouseholdID
				}
			}

			// 2. Check X-Device-Token header (paired kiosk tablet)
			if resolvedHouseholdID == 0 {
				deviceToken := strings.TrimSpace(r.Header.Get("X-Device-Token"))
				if deviceToken != "" {
					dev, err := s.GetHouseholdDeviceByToken(ctx, deviceToken)
					if err == nil && dev != nil && dev.PairedAt != nil {
						resolvedHouseholdID = dev.HouseholdID
						matchedDevice = dev
						// Asynchronously touch last_seen_at
						go func(tok string) {
							_ = s.TouchHouseholdDevice(context.Background(), tok)
						}(deviceToken)
					}
				}
			}

			// 2. Check X-Household-ID header or query param
			if resolvedHouseholdID == 0 {
				hHeader := strings.TrimSpace(r.Header.Get("X-Household-ID"))
				if hHeader == "" {
					hHeader = strings.TrimSpace(r.URL.Query().Get("household_id"))
				}
				if hHeader != "" {
					if id, err := strconv.ParseInt(hHeader, 10, 64); err == nil && id > 0 {
						resolvedHouseholdID = id
					}
				}
			}

			if resolvedHouseholdID > 0 {
				ctx = context.WithValue(ctx, householdContextKey, resolvedHouseholdID)
			}
			if matchedDevice != nil {
				ctx = context.WithValue(ctx, householdDeviceContextKey, matchedDevice)
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// HouseholdIDFromContext returns the resolved household ID for the request,
// or 1 if not set. User session takes priority, followed by device/header, then default 1.
func HouseholdIDFromContext(ctx context.Context) int64 {
	if u := UserFromContext(ctx); u != nil && u.HouseholdID > 0 {
		return u.HouseholdID
	}
	if val, ok := ctx.Value(householdContextKey).(int64); ok && val > 0 {
		return val
	}
	return 1
}

// HouseholdDeviceFromContext returns the matched wall tablet device, or nil.
func HouseholdDeviceFromContext(ctx context.Context) *model.HouseholdDevice {
	if dev, ok := ctx.Value(householdDeviceContextKey).(*model.HouseholdDevice); ok {
		return dev
	}
	return nil
}
