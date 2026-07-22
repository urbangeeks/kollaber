package store

import (
	"context"
	"testing"
)

func TestIsValidEventType(t *testing.T) {
	for _, typ := range ValidEventTypes {
		if !IsValidEventType(typ) {
			t.Errorf("IsValidEventType(%q) = false, want true", typ)
		}
	}
	for _, typ := range []string{"", "Deploy", "restart", "unknown"} {
		if IsValidEventType(typ) {
			t.Errorf("IsValidEventType(%q) = true, want false", typ)
		}
	}
}

// TestCreateEventAllValidTypes is the regression guard for the class of bug
// that dropped rollback and scale events: a type accepted by the Go validator
// but missing from the events_type_check constraint. Inserting one of every
// ValidEventTypes entry against a migrated database fails loudly the moment
// the two definitions drift.
func TestCreateEventAllValidTypes(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	env := newTestEnv(t, pool, org)
	ctx := context.Background()

	for _, typ := range ValidEventTypes {
		t.Run(typ, func(t *testing.T) {
			got, err := q.CreateEvent(ctx, CreateEventParams{
				Type:          typ,
				Service:       "api",
				EnvironmentID: env,
				Metadata:      []byte(`{}`),
				Status:        "success",
			})
			if err != nil {
				t.Fatalf("CreateEvent(type=%q): %v — is %q missing from the events_type_check constraint?", typ, err, typ)
			}
			if got.Type != typ {
				t.Errorf("stored type = %q, want %q", got.Type, typ)
			}
		})
	}
}

// TestCreateEventRejectsUnknownType pins the other direction: the constraint
// must still reject types the application does not know about.
func TestCreateEventRejectsUnknownType(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	env := newTestEnv(t, pool, org)

	_, err := q.CreateEvent(context.Background(), CreateEventParams{
		Type:          "definitely-not-a-type",
		Service:       "api",
		EnvironmentID: env,
		Metadata:      []byte(`{}`),
		Status:        "success",
	})
	if err == nil {
		t.Fatal("CreateEvent accepted an unknown type; the events_type_check constraint is missing or too permissive")
	}
}
