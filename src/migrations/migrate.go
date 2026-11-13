package migrations

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const collectionName = "atv_schema_migrations"

type Migration struct {
	Version  int32
	Name     string
	Checksum string
	Apply    func(context.Context, *mongo.Database) error
}

type migrationRecord struct {
	Version   int32     `bson:"version"`
	Name      string    `bson:"name"`
	Checksum  string    `bson:"checksum"`
	AppliedAt time.Time `bson:"applied_at"`
}

func checksum(source string) string {
	sum := sha256.Sum256([]byte(source))
	return hex.EncodeToString(sum[:])
}

// Run applies pending migrations in ascending version order. Migration bodies
// must be idempotent because MongoDB deployments may not support transactions.
func Run(ctx context.Context, db *mongo.Database) error {
	entries := registered()
	if err := validateRegistry(entries); err != nil {
		return err
	}

	history := db.Collection(collectionName)
	_, err := history.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "version", Value: 1}},
		Options: options.Index().SetUnique(true).SetName("uniq_schema_migration_version"),
	})
	if err != nil {
		return fmt.Errorf("create migration history index: %w", err)
	}

	for _, entry := range entries {
		var applied migrationRecord
		err := history.FindOne(ctx, bson.D{{Key: "version", Value: entry.Version}}).Decode(&applied)
		if err == nil {
			if applied.Name != entry.Name || applied.Checksum != entry.Checksum {
				return fmt.Errorf("migration %d (%s) checksum/name mismatch; refusing to continue", entry.Version, entry.Name)
			}
			continue
		}
		if !errors.Is(err, mongo.ErrNoDocuments) {
			return fmt.Errorf("read migration %d history: %w", entry.Version, err)
		}

		if err := entry.Apply(ctx, db); err != nil {
			return fmt.Errorf("apply migration %d (%s): %w", entry.Version, entry.Name, err)
		}
		record := migrationRecord{Version: entry.Version, Name: entry.Name, Checksum: entry.Checksum, AppliedAt: time.Now().UTC()}
		if _, err := history.InsertOne(ctx, record); err != nil {
			// Multiple application instances can start together. If another
			// instance recorded the same migration, accept only an exact match.
			var concurrent migrationRecord
			readErr := history.FindOne(ctx, bson.D{{Key: "version", Value: entry.Version}}).Decode(&concurrent)
			if readErr == nil && concurrent.Name == entry.Name && concurrent.Checksum == entry.Checksum {
				continue
			}
			return fmt.Errorf("record migration %d (%s): %w", entry.Version, entry.Name, err)
		}
	}
	return nil
}

func validateRegistry(entries []Migration) error {
	if len(entries) == 0 {
		return errors.New("migration registry is empty")
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Version < entries[j].Version })
	for i, entry := range entries {
		if entry.Version <= 0 || entry.Name == "" || entry.Checksum == "" || entry.Apply == nil {
			return fmt.Errorf("migration at index %d is incomplete", i)
		}
		if i > 0 && entries[i-1].Version == entry.Version {
			return fmt.Errorf("duplicate migration version %d", entry.Version)
		}
	}
	return nil
}
