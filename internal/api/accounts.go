package api

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"

	"github.com/liftedkilt/openchore/internal/model"
	"github.com/liftedkilt/openchore/internal/store"
	"golang.org/x/crypto/bcrypt"
)

type AccountHandler struct {
	store    *store.Store
	sessions *SessionManager
}

func NewAccountHandler(s *store.Store, sm *SessionManager) *AccountHandler {
	return &AccountHandler{store: s, sessions: sm}
}

type registerRequest struct {
	Email         string `json:"email"`
	Password      string `json:"password"`
	DisplayName   string `json:"display_name"`
	HouseholdName string `json:"household_name"`
}

type loginAccountRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func generateInviteCode(name string) string {
	runes := []rune(strings.ToUpper(strings.ReplaceAll(name, " ", "")))
	if len(runes) > 6 {
		runes = runes[:6]
	}
	clean := string(runes)
	if clean == "" {
		clean = "HOME"
	}
	b := make([]byte, 3)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s-%s", clean, hex.EncodeToString(b))
}

// Register creates a new platform account, an initial household, and the parent user profile.
func (h *AccountHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	req.HouseholdName = strings.TrimSpace(req.HouseholdName)

	if req.Email == "" || !strings.Contains(req.Email, "@") {
		writeError(w, http.StatusBadRequest, "valid email address is required")
		return
	}
	if len(req.Password) < 6 {
		writeError(w, http.StatusBadRequest, "password must be at least 6 characters")
		return
	}
	if req.DisplayName == "" {
		req.DisplayName = "Parent"
	}
	if req.HouseholdName == "" {
		req.HouseholdName = req.DisplayName + "'s Household"
	}

	// Check if email already registered
	existing, err := h.store.GetAccountByEmail(r.Context(), req.Email)
	if err == nil && existing != nil {
		writeError(w, http.StatusConflict, "an account with this email already exists")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	acc := &model.Account{
		Email:        req.Email,
		PasswordHash: string(hash),
		DisplayName:  req.DisplayName,
	}
	if err := h.store.CreateAccount(r.Context(), acc); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create account")
		return
	}

	household := &model.Household{
		Name:           req.HouseholdName,
		OwnerAccountID: acc.ID,
		InviteCode:     generateInviteCode(req.HouseholdName),
	}
	if err := h.store.CreateHousehold(r.Context(), household); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create household")
		return
	}

	// Create initial parent user in this household
	user := &model.User{
		HouseholdID: household.ID,
		AccountID:   &acc.ID,
		Name:        req.DisplayName,
		Role:        model.RoleAdmin,
	}
	if err := h.store.CreateUser(r.Context(), user); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create parent profile")
		return
	}

	// Issue session for the created user
	token, issued := h.sessions.Issue(SessionClaims{
		UserID:  user.ID,
		Version: user.SessionVersion,
		Method:  SessionMethodPin,
	})
	setSessionCookie(w, r, token, issued)

	writeJSON(w, http.StatusCreated, map[string]any{
		"account": map[string]any{
			"id":           acc.ID,
			"email":        acc.Email,
			"display_name": acc.DisplayName,
		},
		"household": household,
		"user":      user,
		"token":     token,
	})
}

// LoginAccount authenticates an email/password platform account.
func (h *AccountHandler) LoginAccount(w http.ResponseWriter, r *http.Request) {
	var req loginAccountRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	acc, err := h.store.GetAccountByEmail(r.Context(), req.Email)
	if err != nil || acc == nil {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(acc.PasswordHash), []byte(req.Password)); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	households, err := h.store.ListHouseholdsByAccountID(r.Context(), acc.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list households")
		return
	}

	var activeHousehold *model.Household
	if len(households) > 0 {
		activeHousehold = &households[0]
	}

	// Find user profile for the active household
	var activeUser *model.User
	if activeHousehold != nil {
		users, err := h.store.ListUsersByHousehold(r.Context(), activeHousehold.ID)
		if err == nil {
			for i := range users {
				if users[i].AccountID != nil && *users[i].AccountID == acc.ID {
					activeUser = &users[i]
					break
				}
			}
			// If no profile specifically linked to account, pick first admin
			if activeUser == nil {
				for i := range users {
					if users[i].Role == model.RoleAdmin {
						activeUser = &users[i]
						break
					}
				}
			}
		}
	}

	var token string
	if activeUser != nil {
		var issued *SessionClaims
		token, issued = h.sessions.Issue(SessionClaims{
			UserID:  activeUser.ID,
			Version: activeUser.SessionVersion,
			Method:  SessionMethodPin,
		})
		setSessionCookie(w, r, token, issued)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"account": map[string]any{
			"id":           acc.ID,
			"email":        acc.Email,
			"display_name": acc.DisplayName,
		},
		"households": households,
		"user":       activeUser,
		"token":      token,
	})
}

// MyHouseholds lists households associated with the authenticated user/account.
func (h *AccountHandler) MyHouseholds(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "sign in required")
		return
	}

	var households []model.Household
	if u.AccountID != nil {
		var err error
		households, err = h.store.ListHouseholdsByAccountID(r.Context(), *u.AccountID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list households")
			return
		}
	} else if u.HouseholdID > 0 {
		currentH, err := h.store.GetHouseholdByID(r.Context(), u.HouseholdID)
		if err == nil && currentH != nil {
			households = []model.Household{*currentH}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"households": households,
	})
}
