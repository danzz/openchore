package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/liftedkilt/openchore/internal/model"
)

var (
	ErrAccountNotFound         = errors.New("account not found")
	ErrHouseholdNotFound       = errors.New("household not found")
	ErrHouseholdDeviceNotFound = errors.New("household device not found")
	ErrPairCodeExpired         = errors.New("pair code expired")
)

// --- Accounts ---

func (s *Store) CreateAccount(ctx context.Context, a *model.Account) error {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO accounts (email, password_hash, display_name, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		a.Email, a.PasswordHash, a.DisplayName, now, now)
	if err != nil {
		return err
	}
	a.ID, _ = res.LastInsertId()
	a.CreatedAt = now
	a.UpdatedAt = now
	return nil
}

func (s *Store) GetAccountByID(ctx context.Context, id int64) (*model.Account, error) {
	a := &model.Account{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, email, password_hash, display_name, created_at, updated_at FROM accounts WHERE id = ?`, id).
		Scan(&a.ID, &a.Email, &a.PasswordHash, &a.DisplayName, &a.CreatedAt, &a.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrAccountNotFound
	}
	return a, err
}

func (s *Store) GetAccountByEmail(ctx context.Context, email string) (*model.Account, error) {
	a := &model.Account{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, email, password_hash, display_name, created_at, updated_at FROM accounts WHERE email = ? COLLATE NOCASE`, email).
		Scan(&a.ID, &a.Email, &a.PasswordHash, &a.DisplayName, &a.CreatedAt, &a.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrAccountNotFound
	}
	return a, err
}

// --- Households ---

func (s *Store) CreateHousehold(ctx context.Context, h *model.Household) error {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO households (name, owner_account_id, invite_code, created_at) VALUES (?, ?, ?, ?)`,
		h.Name, h.OwnerAccountID, h.InviteCode, now)
	if err != nil {
		return err
	}
	h.ID, _ = res.LastInsertId()
	h.CreatedAt = now
	return nil
}

func (s *Store) GetHouseholdByID(ctx context.Context, id int64) (*model.Household, error) {
	h := &model.Household{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, owner_account_id, invite_code, created_at FROM households WHERE id = ?`, id).
		Scan(&h.ID, &h.Name, &h.OwnerAccountID, &h.InviteCode, &h.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrHouseholdNotFound
	}
	return h, err
}

func (s *Store) GetHouseholdByInviteCode(ctx context.Context, code string) (*model.Household, error) {
	h := &model.Household{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, owner_account_id, invite_code, created_at FROM households WHERE invite_code = ?`, code).
		Scan(&h.ID, &h.Name, &h.OwnerAccountID, &h.InviteCode, &h.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrHouseholdNotFound
	}
	return h, err
}

func (s *Store) ListHouseholdsByAccountID(ctx context.Context, accountID int64) ([]model.Household, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT DISTINCT h.id, h.name, h.owner_account_id, h.invite_code, h.created_at
		 FROM households h
		 LEFT JOIN users u ON u.household_id = h.id
		 WHERE h.owner_account_id = ? OR u.account_id = ?
		 ORDER BY h.id ASC`, accountID, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.Household
	for rows.Next() {
		var h model.Household
		if err := rows.Scan(&h.ID, &h.Name, &h.OwnerAccountID, &h.InviteCode, &h.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func (s *Store) ListActiveHouseholds(ctx context.Context) ([]model.Household, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, owner_account_id, invite_code, created_at FROM households ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.Household
	for rows.Next() {
		var h model.Household
		if err := rows.Scan(&h.ID, &h.Name, &h.OwnerAccountID, &h.InviteCode, &h.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// --- Household Devices (Kiosk / Tablet) ---

func (s *Store) CreateHouseholdDevice(ctx context.Context, d *model.HouseholdDevice) error {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO household_devices (household_id, device_token, device_name, pair_code, pair_code_expires_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		d.HouseholdID, d.DeviceToken, d.DeviceName, d.PairCode, d.PairCodeExpiresAt, now)
	if err != nil {
		return err
	}
	d.ID, _ = res.LastInsertId()
	d.CreatedAt = now
	return nil
}

func (s *Store) GetHouseholdDeviceByToken(ctx context.Context, token string) (*model.HouseholdDevice, error) {
	d := &model.HouseholdDevice{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, household_id, device_token, device_name, COALESCE(pair_code, ''), pair_code_expires_at, paired_at, last_seen_at, created_at
		 FROM household_devices WHERE device_token = ?`, token).
		Scan(&d.ID, &d.HouseholdID, &d.DeviceToken, &d.DeviceName, &d.PairCode, &d.PairCodeExpiresAt, &d.PairedAt, &d.LastSeenAt, &d.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrHouseholdDeviceNotFound
	}
	return d, err
}

func (s *Store) GetHouseholdDeviceByPairCode(ctx context.Context, code string) (*model.HouseholdDevice, error) {
	d := &model.HouseholdDevice{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, household_id, device_token, device_name, COALESCE(pair_code, ''), pair_code_expires_at, paired_at, last_seen_at, created_at
		 FROM household_devices WHERE pair_code = ?`, code).
		Scan(&d.ID, &d.HouseholdID, &d.DeviceToken, &d.DeviceName, &d.PairCode, &d.PairCodeExpiresAt, &d.PairedAt, &d.LastSeenAt, &d.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrHouseholdDeviceNotFound
	}
	if d.PairCodeExpiresAt != nil && d.PairCodeExpiresAt.Before(time.Now().UTC()) {
		return nil, ErrPairCodeExpired
	}
	return d, err
}

func (s *Store) ClaimHouseholdDevice(ctx context.Context, code string, householdID int64, newDeviceToken string, deviceName string) (*model.HouseholdDevice, error) {
	d, err := s.GetHouseholdDeviceByPairCode(ctx, code)
	if err != nil {
		return nil, err
	}

	if deviceName == "" {
		deviceName = d.DeviceName
	}
	now := time.Now().UTC()
	_, err = s.db.ExecContext(ctx,
		`UPDATE household_devices
		 SET household_id = ?, device_token = ?, device_name = ?, paired_at = ?, last_seen_at = ?
		 WHERE id = ?`,
		householdID, newDeviceToken, deviceName, now, now, d.ID)
	if err != nil {
		return nil, err
	}
	d.HouseholdID = householdID
	d.DeviceToken = newDeviceToken
	d.DeviceName = deviceName
	d.PairedAt = &now
	d.LastSeenAt = &now
	return d, nil
}

func (s *Store) TouchHouseholdDevice(ctx context.Context, token string) error {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx,
		`UPDATE household_devices SET last_seen_at = ? WHERE device_token = ?`, now, token)
	return err
}

func (s *Store) ListHouseholdDevices(ctx context.Context, householdID int64) ([]model.HouseholdDevice, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, household_id, device_token, device_name, paired_at, last_seen_at, created_at
		 FROM household_devices WHERE household_id = ? AND paired_at IS NOT NULL
		 ORDER BY id DESC`, householdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.HouseholdDevice
	for rows.Next() {
		var d model.HouseholdDevice
		if err := rows.Scan(&d.ID, &d.HouseholdID, &d.DeviceToken, &d.DeviceName, &d.PairedAt, &d.LastSeenAt, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) DeleteHouseholdDevice(ctx context.Context, id int64, householdID int64) error {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM household_devices WHERE id = ? AND household_id = ?`, id, householdID)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrHouseholdDeviceNotFound
	}
	return nil
}

// --- Scoped Household Settings ---

func (s *Store) GetHouseholdSetting(ctx context.Context, householdID int64, key string) (string, error) {
	var val string
	err := s.db.QueryRowContext(ctx,
		`SELECT value FROM household_settings WHERE household_id = ? AND key = ?`, householdID, key).
		Scan(&val)
	if err == sql.ErrNoRows {
		// Fallback to legacy app_settings if not customized for household yet
		errLegacy := s.db.QueryRowContext(ctx, `SELECT value FROM app_settings WHERE key = ?`, key).Scan(&val)
		if errLegacy != nil {
			return "", err
		}
		return val, nil
	}
	return val, err
}

func (s *Store) SetHouseholdSetting(ctx context.Context, householdID int64, key, value string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO household_settings (household_id, key, value) VALUES (?, ?, ?)
		 ON CONFLICT(household_id, key) DO UPDATE SET value = ?`,
		householdID, key, value, value)
	return err
}

func (s *Store) ListHouseholdSettings(ctx context.Context, householdID int64) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT key, value FROM household_settings WHERE household_id = ?`, householdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	settings := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		settings[k] = v
	}
	return settings, rows.Err()
}

// ListUsersByHousehold returns all users belonging to a specific household.
func (s *Store) ListUsersByHousehold(ctx context.Context, householdID int64) ([]model.User, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, household_id, account_id, name, avatar_url, role, age, theme, line_color, color, paused, pin_hash, pin_length, session_version, created_at
		 FROM users WHERE household_id = ? ORDER BY name`, householdID)
	if err != nil {
		return nil, err
	}
	var users []model.User
	for rows.Next() {
		var u model.User
		var paused int
		if err := rows.Scan(&u.ID, &u.HouseholdID, &u.AccountID, &u.Name, &u.AvatarURL, &u.Role, &u.Age, &u.Theme, &u.LineColor, &u.Color, &paused, &u.PinHash, &u.PinLength, &u.SessionVersion, &u.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		u.Paused = paused == 1
		u.HasPin = u.PinHash != ""
		users = append(users, u)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	providers, err := s.listIdentityProviders(ctx)
	if err != nil {
		return nil, err
	}
	for i := range users {
		users[i].AuthProviders = nonNilStrings(providers[users[i].ID])
	}
	return users, nil
}

