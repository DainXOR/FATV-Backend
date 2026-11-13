package migrations

import (
	"context"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"testing"
)

func TestValidateRegistrySortsVersions(t *testing.T) {
	noop := func(context.Context, *mongo.Database) error { return nil }
	entries := []Migration{{Version: 2, Name: "second", Checksum: "b", Apply: noop}, {Version: 1, Name: "first", Checksum: "a", Apply: noop}}
	if err := validateRegistry(entries); err != nil {
		t.Fatalf("validateRegistry() error = %v", err)
	}
	if entries[0].Version != 1 || entries[1].Version != 2 {
		t.Fatalf("registry not sorted: got %d, %d", entries[0].Version, entries[1].Version)
	}
}

func TestValidateRegistryRejectsInvalidEntries(t *testing.T) {
	noop := func(context.Context, *mongo.Database) error { return nil }
	tests := []struct {
		name    string
		entries []Migration
	}{
		{name: "empty"},
		{name: "zero version", entries: []Migration{{Version: 0, Name: "zero", Checksum: "x", Apply: noop}}},
		{name: "missing name", entries: []Migration{{Version: 1, Checksum: "x", Apply: noop}}},
		{name: "missing checksum", entries: []Migration{{Version: 1, Name: "one", Apply: noop}}},
		{name: "missing apply", entries: []Migration{{Version: 1, Name: "one", Checksum: "x"}}},
		{name: "duplicate version", entries: []Migration{{Version: 1, Name: "one", Checksum: "x", Apply: noop}, {Version: 1, Name: "other", Checksum: "y", Apply: noop}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := validateRegistry(test.entries); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestChecksumIsStableAndContentSensitive(t *testing.T) {
	if checksum("migration") != checksum("migration") {
		t.Fatal("checksum must be stable")
	}
	if checksum("migration") == checksum("different migration") {
		t.Fatal("different sources must have different checksums")
	}
}

func TestMigrationRegistryIsOrderedAndUnique(t *testing.T) {
	entries := registered()
	if err := validateRegistry(entries); err != nil {
		t.Fatal(err)
	}
	for i, entry := range entries {
		if entry.Version != int32(i+1) {
			t.Fatalf("expected contiguous version %d, got %d", i+1, entry.Version)
		}
	}
}

func TestParseLegacySessionDate(t *testing.T) {
	tests := []string{
		"2025-12-05T11:40:00",
		"2025-12-05T11:40",
		"05-12-2025T11:40",
		"23/08/2025 5:00:17",
		"2025-12-05T11:40:00Z",
	}
	for _, value := range tests {
		t.Run(value, func(t *testing.T) {
			if parsed, err := parseLegacySessionDate(value); err != nil || parsed.IsZero() {
				t.Fatalf("parseLegacySessionDate(%q) = %v, %v", value, parsed, err)
			}
		})
	}
}

func TestLegacySessionDateSentinels(t *testing.T) {
	for _, value := range []string{"0001-01-01T00:00:00Z", "2000-01-01T05:00:00Z"} {
		parsed, err := parseLegacySessionDate(value)
		if err != nil {
			t.Fatalf("parseLegacySessionDate(%q): %v", value, err)
		}
		if !isLegacyDateSentinel(parsed) {
			t.Fatalf("expected %q to be recognized as a sentinel", value)
		}
	}
}
