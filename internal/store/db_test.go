package store_test

import (
	"testing"

	"github.com/liftedkilt/openchore/internal/store"
)

func TestRebindPostgres(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "single placeholder",
			input:    "SELECT * FROM users WHERE id = ?",
			expected: "SELECT * FROM users WHERE id = $1",
		},
		{
			name:     "multiple placeholders",
			input:    "INSERT INTO users (name, role, age) VALUES (?, ?, ?)",
			expected: "INSERT INTO users (name, role, age) VALUES ($1, $2, $3)",
		},
		{
			name:     "placeholder in single quotes ignored",
			input:    "SELECT * FROM items WHERE note = 'Is this ok?' AND id = ?",
			expected: "SELECT * FROM items WHERE note = 'Is this ok?' AND id = $1",
		},
		{
			name:     "escaped single quote in string literal",
			input:    "SELECT * FROM items WHERE note = 'Can''t find ?' AND id = ?",
			expected: "SELECT * FROM items WHERE note = 'Can''t find ?' AND id = $1",
		},
		{
			name:     "placeholder in double quotes ignored",
			input:    `SELECT "col?1", "col?2" FROM items WHERE id = ?`,
			expected: `SELECT "col?1", "col?2" FROM items WHERE id = $1`,
		},
		{
			name:     "single line comment with question mark",
			input:    "SELECT * FROM items -- is this active?\nWHERE id = ?",
			expected: "SELECT * FROM items -- is this active?\nWHERE id = $1",
		},
		{
			name:     "block comment with question mark",
			input:    "SELECT /* is this a test? */ * FROM items WHERE id = ?",
			expected: "SELECT /* is this a test? */ * FROM items WHERE id = $1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := store.RebindPostgres(tc.input)
			if got != tc.expected {
				t.Fatalf("RebindPostgres mismatch:\n  got:  %q\n  want: %q", got, tc.expected)
			}
		})
	}
}

func TestInsertReturningID(t *testing.T) {
	s := setupStore(t)

	// Direct test through public store methods (which internally use InsertReturningID)
	u := createTestUser(t, s, "TestUserReturningID", "child")
	if u.ID <= 0 {
		t.Fatalf("expected positive ID, got %d", u.ID)
	}

	chore := createTestChore(t, s, "TestChoreReturningID", 10, u.ID)
	if chore.ID <= 0 {
		t.Fatalf("expected positive chore ID, got %d", chore.ID)
	}
}
