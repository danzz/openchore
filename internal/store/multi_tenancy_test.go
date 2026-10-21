package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/liftedkilt/openchore/internal/model"
	"github.com/liftedkilt/openchore/internal/store"
)

func TestAccountCRUD(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	acc := &model.Account{
		Email:        "parent@example.com",
		PasswordHash: "hashed_secret",
		DisplayName:  "Alice",
	}

	if err := s.CreateAccount(ctx, acc); err != nil {
		t.Fatalf("CreateAccount failed: %v", err)
	}
	if acc.ID == 0 {
		t.Fatal("expected non-zero account ID")
	}

	byID, err := s.GetAccountByID(ctx, acc.ID)
	if err != nil {
		t.Fatalf("GetAccountByID failed: %v", err)
	}
	if byID.Email != acc.Email || byID.DisplayName != "Alice" {
		t.Fatalf("unexpected account: %+v", byID)
	}

	// Case-insensitive email lookup
	byEmail, err := s.GetAccountByEmail(ctx, "PARENT@example.com")
	if err != nil {
		t.Fatalf("GetAccountByEmail failed: %v", err)
	}
	if byEmail.ID != acc.ID {
		t.Fatalf("expected ID %d, got %d", acc.ID, byEmail.ID)
	}

	// Not found
	_, err = s.GetAccountByEmail(ctx, "nonexistent@example.com")
	if err != store.ErrAccountNotFound {
		t.Fatalf("expected ErrAccountNotFound, got %v", err)
	}
}

func TestHouseholdCRUD(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	owner := &model.Account{
		Email:        "bob@example.com",
		PasswordHash: "hash123",
		DisplayName:  "Bob",
	}
	if err := s.CreateAccount(ctx, owner); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	h := &model.Household{
		Name:           "Bob's Family",
		OwnerAccountID: owner.ID,
		InviteCode:     "BOB-123",
	}
	if err := s.CreateHousehold(ctx, h); err != nil {
		t.Fatalf("CreateHousehold: %v", err)
	}
	if h.ID == 0 {
		t.Fatal("expected non-zero household ID")
	}

	byID, err := s.GetHouseholdByID(ctx, h.ID)
	if err != nil {
		t.Fatalf("GetHouseholdByID: %v", err)
	}
	if byID.Name != "Bob's Family" {
		t.Fatalf("unexpected name: %s", byID.Name)
	}

	byCode, err := s.GetHouseholdByInviteCode(ctx, "BOB-123")
	if err != nil {
		t.Fatalf("GetHouseholdByInviteCode: %v", err)
	}
	if byCode.ID != h.ID {
		t.Fatalf("expected ID %d, got %d", h.ID, byCode.ID)
	}

	list, err := s.ListHouseholdsByAccountID(ctx, owner.ID)
	if err != nil {
		t.Fatalf("ListHouseholdsByAccountID: %v", err)
	}
	if len(list) != 1 || list[0].ID != h.ID {
		t.Fatalf("unexpected households: %+v", list)
	}

	active, err := s.ListActiveHouseholds(ctx)
	if err != nil {
		t.Fatalf("ListActiveHouseholds: %v", err)
	}
	// Default household (ID 1) + newly created household
	if len(active) < 2 {
		t.Fatalf("expected at least 2 active households, got %d", len(active))
	}
}

func TestHouseholdDevicePairing(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	owner := &model.Account{
		Email:        "charlie@example.com",
		PasswordHash: "hash456",
		DisplayName:  "Charlie",
	}
	if err := s.CreateAccount(ctx, owner); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	h := &model.Household{
		Name:           "Charlie Family",
		OwnerAccountID: owner.ID,
		InviteCode:     "CHARLIE-456",
	}
	if err := s.CreateHousehold(ctx, h); err != nil {
		t.Fatalf("CreateHousehold: %v", err)
	}

	// 1. Tablet requests pairing code
	exp := time.Now().UTC().Add(10 * time.Minute)
	dev := &model.HouseholdDevice{
		HouseholdID:       h.ID,
		DeviceToken:       "init-token-123",
		DeviceName:        "Living Room iPad",
		PairCode:          "123456",
		PairCodeExpiresAt: &exp,
	}
	if err := s.CreateHouseholdDevice(ctx, dev); err != nil {
		t.Fatalf("CreateHouseholdDevice: %v", err)
	}

	foundDev, err := s.GetHouseholdDeviceByPairCode(ctx, "123456")
	if err != nil {
		t.Fatalf("GetHouseholdDeviceByPairCode: %v", err)
	}
	if foundDev.ID != dev.ID {
		t.Fatalf("expected device ID %d, got %d", dev.ID, foundDev.ID)
	}

	// 2. Parent claims device with code and issues persistent token
	claimed, err := s.ClaimHouseholdDevice(ctx, "123456", h.ID, "perm-token-999", "Living Room iPad")
	if err != nil {
		t.Fatalf("ClaimHouseholdDevice: %v", err)
	}
	if claimed.DeviceToken != "perm-token-999" || claimed.PairedAt == nil {
		t.Fatalf("unexpected claimed device: %+v", claimed)
	}

	// Verify pair code resolves to paired device
	afterClaim, err := s.GetHouseholdDeviceByPairCode(ctx, "123456")
	if err != nil || afterClaim.PairedAt == nil {
		t.Fatalf("expected paired device, got %+v, err: %v", afterClaim, err)
	}

	// Verify token lookup & touch
	byToken, err := s.GetHouseholdDeviceByToken(ctx, "perm-token-999")
	if err != nil {
		t.Fatalf("GetHouseholdDeviceByToken: %v", err)
	}
	if byToken.DeviceName != "Living Room iPad" {
		t.Fatalf("unexpected device name: %s", byToken.DeviceName)
	}

	if err := s.TouchHouseholdDevice(ctx, "perm-token-999"); err != nil {
		t.Fatalf("TouchHouseholdDevice: %v", err)
	}

	// List devices
	devices, err := s.ListHouseholdDevices(ctx, h.ID)
	if err != nil {
		t.Fatalf("ListHouseholdDevices: %v", err)
	}
	if len(devices) != 1 || devices[0].DeviceToken != "perm-token-999" {
		t.Fatalf("unexpected device list: %+v", devices)
	}

	// Delete device
	if err := s.DeleteHouseholdDevice(ctx, claimed.ID, h.ID); err != nil {
		t.Fatalf("DeleteHouseholdDevice: %v", err)
	}
	devicesAfter, err := s.ListHouseholdDevices(ctx, h.ID)
	if err != nil {
		t.Fatalf("ListHouseholdDevices: %v", err)
	}
	if len(devicesAfter) != 0 {
		t.Fatalf("expected 0 devices after deletion, got %d", len(devicesAfter))
	}
}

func TestHouseholdSettings(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	// Default household setting fallback
	if _, err := s.GetHouseholdSetting(ctx, 1, "timezone"); err != nil {
		// might not be set yet, which is fine
	}

	// Create household 2
	owner := &model.Account{
		Email:        "david@example.com",
		PasswordHash: "pass789",
		DisplayName:  "David",
	}
	if err := s.CreateAccount(ctx, owner); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	h2 := &model.Household{
		Name:           "David Family",
		OwnerAccountID: owner.ID,
		InviteCode:     "DAVID-789",
	}
	if err := s.CreateHousehold(ctx, h2); err != nil {
		t.Fatalf("CreateHousehold: %v", err)
	}

	// Set household 2 specific setting
	if err := s.SetHouseholdSetting(ctx, h2.ID, "discord_webhook_url", "https://discord.com/api/webhooks/222"); err != nil {
		t.Fatalf("SetHouseholdSetting: %v", err)
	}

	got2, err := s.GetHouseholdSetting(ctx, h2.ID, "discord_webhook_url")
	if err != nil {
		t.Fatalf("GetHouseholdSetting: %v", err)
	}
	if got2 != "https://discord.com/api/webhooks/222" {
		t.Fatalf("unexpected setting value: %s", got2)
	}

	// Verify non-existent household 999 doesn't have it
	_, err3 := s.GetHouseholdSetting(ctx, 999, "discord_webhook_url")
	if err3 == nil {
		t.Fatal("expected error for non-existent setting in household 999")
	}

	all2, err := s.ListHouseholdSettings(ctx, h2.ID)
	if err != nil {
		t.Fatalf("ListHouseholdSettings: %v", err)
	}
	if all2["discord_webhook_url"] != "https://discord.com/api/webhooks/222" {
		t.Fatalf("unexpected map: %+v", all2)
	}
}
