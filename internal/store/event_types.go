package store

import "slices"

// ValidEventTypes is the canonical set of event types, and must stay in step
// with the events_type_check constraint in the migrations.
//
// Adding a type here without widening that CHECK makes every insert of it fail
// at the database and surface as a 500 — which is exactly how rollback and
// scale events were silently dropped between migrations 015 and 019.
// TestCreateEventAllValidTypes inserts one of each against a migrated database
// so the two cannot drift apart again.
var ValidEventTypes = []string{"deploy", "alert", "note", "teardown", "rollback", "scale"}

// IsValidEventType reports whether t is an event type the schema accepts.
func IsValidEventType(t string) bool {
	return slices.Contains(ValidEventTypes, t)
}
