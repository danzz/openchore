package api_test

import (
	"fmt"
	"net/http"
	"testing"
)

func TestAccountRegisterAndLoginAPI(t *testing.T) {
	env := setupTest(t)

	// 1. Register a new parent account and household
	regPayload := map[string]any{
		"email":          "smith@example.com",
		"password":       "password123",
		"display_name":   "John Smith",
		"household_name": "Smith Family",
	}

	resp := env.request(t, http.MethodPost, "/api/auth/register", regPayload, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d", resp.StatusCode)
	}

	var regResult struct {
		Account struct {
			ID          int64  `json:"id"`
			Email       string `json:"email"`
			DisplayName string `json:"display_name"`
		} `json:"account"`
		Household struct {
			ID         int64  `json:"id"`
			Name       string `json:"name"`
			InviteCode string `json:"invite_code"`
		} `json:"household"`
		Token string `json:"token"`
	}
	decodeBody(t, resp, &regResult)

	if regResult.Account.Email != "smith@example.com" {
		t.Fatalf("unexpected account email: %s", regResult.Account.Email)
	}
	if regResult.Household.Name != "Smith Family" {
		t.Fatalf("unexpected household name: %s", regResult.Household.Name)
	}
	if regResult.Token == "" {
		t.Fatal("expected token to be issued")
	}

	// 2. Duplicate registration rejected
	dupResp := env.request(t, http.MethodPost, "/api/auth/register", regPayload, nil)
	if dupResp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 Conflict for duplicate email, got %d", dupResp.StatusCode)
	}

	// 3. Login with correct password
	loginPayload := map[string]any{
		"email":    "smith@example.com",
		"password": "password123",
	}
	loginResp := env.request(t, http.MethodPost, "/api/auth/login-account", loginPayload, nil)
	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for login, got %d", loginResp.StatusCode)
	}

	var loginResult struct {
		Account struct {
			Email string `json:"email"`
		} `json:"account"`
		Households []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"households"`
		Token string `json:"token"`
	}
	decodeBody(t, loginResp, &loginResult)
	if len(loginResult.Households) != 1 || loginResult.Households[0].Name != "Smith Family" {
		t.Fatalf("unexpected households: %+v", loginResult.Households)
	}

	// 4. Login with incorrect password
	badLoginResp := env.request(t, http.MethodPost, "/api/auth/login-account", map[string]any{
		"email":    "smith@example.com",
		"password": "wrongpassword",
	}, nil)
	if badLoginResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d", badLoginResp.StatusCode)
	}
}

func TestKioskPairingFlow(t *testing.T) {
	env := setupTest(t)

	// 1. Create a parent admin with household
	regResp := env.request(t, http.MethodPost, "/api/auth/register", map[string]any{
		"email":          "alice@example.com",
		"password":       "alicepass123",
		"display_name":   "Alice",
		"household_name": "Alice Household",
	}, nil)
	if regResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d", regResp.StatusCode)
	}

	var regResult struct {
		Household struct {
			ID int64 `json:"id"`
		} `json:"household"`
		User struct {
			ID int64 `json:"id"`
		} `json:"user"`
		Token string `json:"token"`
	}
	decodeBody(t, regResp, &regResult)
	adminHeaders := map[string]string{
		"Authorization": "Bearer " + regResult.Token,
	}

	// Add a kid to Alice's household
	kidResp := env.request(t, http.MethodPost, "/api/users", map[string]any{
		"name": "Alice Junior",
		"role": "child",
	}, adminHeaders)
	if kidResp.StatusCode != http.StatusCreated {
		t.Fatalf("failed to create kid: %d", kidResp.StatusCode)
	}

	// 2. Wall tablet initiates pairing
	initResp := env.request(t, http.MethodPost, "/api/kiosk/pair/init", nil, nil)
	if initResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /kiosk/pair/init, got %d", initResp.StatusCode)
	}
	var initResult struct {
		PairCode string `json:"pair_code"`
	}
	decodeBody(t, initResp, &initResult)
	if len(initResult.PairCode) != 6 {
		t.Fatalf("expected 6-digit pair code, got %s", initResult.PairCode)
	}

	// 3. Wall tablet polls before claim -> paired is false
	poll1 := env.request(t, http.MethodGet, "/api/kiosk/pair/poll?code="+initResult.PairCode, nil, nil)
	var poll1Result struct {
		Paired bool `json:"paired"`
	}
	decodeBody(t, poll1, &poll1Result)
	if poll1Result.Paired {
		t.Fatal("expected paired to be false before claim")
	}

	// 4. Parent claims device with pair code
	claimResp := env.request(t, http.MethodPost, "/api/kiosk/pair/claim", map[string]any{
		"code":         initResult.PairCode,
		"device_name":  "Kitchen Wall Display",
		"household_id": regResult.Household.ID,
	}, adminHeaders)
	if claimResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /kiosk/pair/claim, got %d", claimResp.StatusCode)
	}

	// 5. Tablet polls again -> paired is true and returns device_token
	poll2 := env.request(t, http.MethodGet, "/api/kiosk/pair/poll?code="+initResult.PairCode, nil, nil)
	var poll2Result struct {
		Paired      bool   `json:"paired"`
		DeviceToken string `json:"device_token"`
		HouseholdID int64  `json:"household_id"`
		DeviceName  string `json:"device_name"`
	}
	decodeBody(t, poll2, &poll2Result)
	if !poll2Result.Paired || poll2Result.DeviceToken == "" {
		t.Fatalf("expected paired with device_token, got %+v", poll2Result)
	}
	if poll2Result.HouseholdID != regResult.Household.ID {
		t.Fatalf("expected household_id %d, got %d", regResult.Household.ID, poll2Result.HouseholdID)
	}

	// 6. Wall tablet uses X-Device-Token to list users
	deviceHeaders := map[string]string{
		"X-Device-Token": poll2Result.DeviceToken,
	}
	usersResp := env.request(t, http.MethodGet, "/api/users", nil, deviceHeaders)
	if usersResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/users with X-Device-Token, got %d", usersResp.StatusCode)
	}

	var users []struct {
		ID          int64  `json:"id"`
		Name        string `json:"name"`
		HouseholdID int64  `json:"household_id"`
	}
	decodeBody(t, usersResp, &users)

	// Should contain Alice and Alice Junior, all scoped to Alice's household!
	if len(users) < 2 {
		t.Fatalf("expected at least 2 users for Alice's household, got %d", len(users))
	}
	for _, u := range users {
		if u.HouseholdID != regResult.Household.ID {
			t.Fatalf("expected user to belong to household %d, got %d (user %s)", regResult.Household.ID, u.HouseholdID, u.Name)
		}
	}

	// 7. Admin lists paired devices
	devicesResp := env.request(t, http.MethodGet, "/api/admin/kiosk/devices", nil, adminHeaders)
	if devicesResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /admin/kiosk/devices, got %d", devicesResp.StatusCode)
	}
	var devList struct {
		Devices []struct {
			ID         int64  `json:"id"`
			DeviceName string `json:"device_name"`
		} `json:"devices"`
	}
	decodeBody(t, devicesResp, &devList)
	if len(devList.Devices) != 1 || devList.Devices[0].DeviceName != "Kitchen Wall Display" {
		t.Fatalf("unexpected devices: %+v", devList.Devices)
	}

	// 8. Admin deletes the device
	delResp := env.request(t, http.MethodDelete, fmt.Sprintf("/api/admin/kiosk/devices/%d", devList.Devices[0].ID), nil, adminHeaders)
	if delResp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204 No Content on delete device, got %d", delResp.StatusCode)
	}
}

func TestHouseholdRewardsIsolation(t *testing.T) {
	env := setupTest(t)

	// 1. Household A
	regAResp := env.request(t, http.MethodPost, "/api/auth/register", map[string]any{
		"email":          "family.a@example.com",
		"password":       "password123",
		"display_name":   "Parent A",
		"household_name": "Household A",
	}, nil)
	var regA struct {
		Token string `json:"token"`
	}
	decodeBody(t, regAResp, &regA)
	headersA := map[string]string{"Authorization": "Bearer " + regA.Token}

	// Create Reward A in Household A
	createRewardAResp := env.request(t, http.MethodPost, "/api/rewards", map[string]any{
		"name": "Reward A Ice Cream",
		"cost": 50,
	}, headersA)
	if createRewardAResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 for create reward A, got %d", createRewardAResp.StatusCode)
	}

	// 2. Household B
	regBResp := env.request(t, http.MethodPost, "/api/auth/register", map[string]any{
		"email":          "family.b@example.com",
		"password":       "password123",
		"display_name":   "Parent B",
		"household_name": "Household B",
	}, nil)
	var regB struct {
		Token string `json:"token"`
	}
	decodeBody(t, regBResp, &regB)
	headersB := map[string]string{"Authorization": "Bearer " + regB.Token}

	// Create Reward B in Household B
	createRewardBResp := env.request(t, http.MethodPost, "/api/rewards", map[string]any{
		"name": "Reward B Lego Set",
		"cost": 200,
	}, headersB)
	if createRewardBResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 for create reward B, got %d", createRewardBResp.StatusCode)
	}

	// 3. User Reward List isolation (GET /api/rewards)
	var rewardsA []struct {
		Name string `json:"name"`
	}
	listAResp := env.request(t, http.MethodGet, "/api/rewards", nil, headersA)
	decodeBody(t, listAResp, &rewardsA)
	if len(rewardsA) != 1 || rewardsA[0].Name != "Reward A Ice Cream" {
		t.Fatalf("Household A should only see Reward A, got %+v", rewardsA)
	}

	var rewardsB []struct {
		Name string `json:"name"`
	}
	listBResp := env.request(t, http.MethodGet, "/api/rewards", nil, headersB)
	decodeBody(t, listBResp, &rewardsB)
	if len(rewardsB) != 1 || rewardsB[0].Name != "Reward B Lego Set" {
		t.Fatalf("Household B should only see Reward B, got %+v", rewardsB)
	}

	// 4. Admin Reward List isolation (GET /api/rewards/all)
	var adminRewardsA []struct {
		Name string `json:"name"`
	}
	listAllAResp := env.request(t, http.MethodGet, "/api/rewards/all", nil, headersA)
	decodeBody(t, listAllAResp, &adminRewardsA)
	if len(adminRewardsA) != 1 || adminRewardsA[0].Name != "Reward A Ice Cream" {
		t.Fatalf("Admin Household A should only see Reward A in listAll, got %+v", adminRewardsA)
	}

	var adminRewardsB []struct {
		Name string `json:"name"`
	}
	listAllBResp := env.request(t, http.MethodGet, "/api/rewards/all", nil, headersB)
	decodeBody(t, listAllBResp, &adminRewardsB)
	if len(adminRewardsB) != 1 || adminRewardsB[0].Name != "Reward B Lego Set" {
		t.Fatalf("Admin Household B should only see Reward B in listAll, got %+v", adminRewardsB)
	}

	// 5. Streak Rewards isolation
	env.request(t, http.MethodPost, "/api/admin/streak-rewards", map[string]any{
		"streak_days":  3,
		"bonus_points": 15,
		"label":        "3-day bonus for A",
	}, headersA)

	env.request(t, http.MethodPost, "/api/admin/streak-rewards", map[string]any{
		"streak_days":  7,
		"bonus_points": 50,
		"label":        "7-day bonus for B",
	}, headersB)

	// Household B should also be able to create a 3-day milestone without conflict with Household A
	respB3 := env.request(t, http.MethodPost, "/api/admin/streak-rewards", map[string]any{
		"streak_days":  3,
		"bonus_points": 25,
		"label":        "3-day bonus for B",
	}, headersB)
	if respB3.StatusCode != http.StatusCreated {
		t.Fatalf("Household B should be able to create 3-day milestone even if Household A already has one, got status %d", respB3.StatusCode)
	}

	// Creating duplicate milestone in Household B should return 409 Conflict
	respBDup := env.request(t, http.MethodPost, "/api/admin/streak-rewards", map[string]any{
		"streak_days":  3,
		"bonus_points": 30,
		"label":        "duplicate 3-day bonus for B",
	}, headersB)
	if respBDup.StatusCode != http.StatusConflict {
		t.Fatalf("Duplicate milestone for same days in same household should return 409 Conflict, got status %d", respBDup.StatusCode)
	}

	var streaksA []struct {
		Label string `json:"label"`
	}
	streaksAResp := env.request(t, http.MethodGet, "/api/admin/streak-rewards", nil, headersA)
	decodeBody(t, streaksAResp, &streaksA)
	if len(streaksA) != 1 || streaksA[0].Label != "3-day bonus for A" {
		t.Fatalf("Household A should only see its own streak rewards, got %+v", streaksA)
	}

	var streaksB []struct {
		Label string `json:"label"`
	}
	streaksBResp := env.request(t, http.MethodGet, "/api/admin/streak-rewards", nil, headersB)
	decodeBody(t, streaksBResp, &streaksB)
	if len(streaksB) != 2 {
		t.Fatalf("Household B should see its 2 streak rewards, got %+v", streaksB)
	}
}

