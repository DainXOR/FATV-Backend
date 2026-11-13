package migrations

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const formsSectionsV7Source = "v7: set missing legacy section metadata weights to the form creator's 25 percent default while preserving existing metadata and weights"

func restoreDefaultLegacySectionWeights(ctx context.Context, db *mongo.Database) error {
	legacyBackups := db.Collection("schema_backup_forms_v1")
	forms := db.Collection("forms")
	safetyBackups := db.Collection("schema_backup_forms_v7")

	legacyCursor, err := legacyBackups.Find(ctx,
		bson.D{{Key: "source_collection", Value: "forms"}, {Key: "migration_version", Value: int32(1)}},
		options.Find().SetBatchSize(100),
	)
	if err != nil {
		return fmt.Errorf("find legacy form backups for weight normalization: %w", err)
	}
	defer legacyCursor.Close(ctx)

	ids := make([]bson.ObjectID, 0, 100)
	for legacyCursor.Next(ctx) {
		var legacy legacyFormBackup
		if err := legacyCursor.Decode(&legacy); err != nil {
			return fmt.Errorf("decode legacy form backup for weights: %w", err)
		}
		if _, hadMetadata := legacy.Original["sections_info"]; hadMetadata {
			continue
		}
		ids = append(ids, legacy.ID)
	}
	if err := legacyCursor.Err(); err != nil {
		return fmt.Errorf("iterate legacy form backups for weights: %w", err)
	}
	if len(ids) == 0 {
		return nil
	}

	// Back up the current post-migration documents in one batch before changing
	// any weights. Duplicate IDs mean an earlier interrupted attempt already
	// preserved that source document.
	currentCursor, err := forms.Find(ctx, bson.D{{Key: "_id", Value: bson.D{{Key: "$in", Value: ids}}}},
		options.Find().SetBatchSize(100))
	if err != nil {
		return fmt.Errorf("find forms for weight-correction backup: %w", err)
	}
	backupDocuments := make([]any, 0, len(ids))
	for currentCursor.Next(ctx) {
		var original bson.Raw
		if err := currentCursor.Decode(&original); err != nil {
			_ = currentCursor.Close(ctx)
			return fmt.Errorf("decode form for weight-correction backup: %w", err)
		}
		var identity struct {
			ID bson.ObjectID `bson:"_id"`
		}
		if err := bson.Unmarshal(original, &identity); err != nil {
			_ = currentCursor.Close(ctx)
			return fmt.Errorf("decode form identity for weight-correction backup: %w", err)
		}
		backupDocuments = append(backupDocuments, bson.D{
			{Key: "_id", Value: identity.ID},
			{Key: "source_collection", Value: "forms"},
			{Key: "migration_version", Value: int32(7)},
			{Key: "backed_up_at", Value: time.Now().UTC()},
			{Key: "original", Value: original},
		})
	}
	if err := currentCursor.Err(); err != nil {
		_ = currentCursor.Close(ctx)
		return fmt.Errorf("iterate forms for weight-correction backup: %w", err)
	}
	if err := currentCursor.Close(ctx); err != nil {
		return fmt.Errorf("close forms weight-correction backup cursor: %w", err)
	}
	if len(backupDocuments) > 0 {
		_, err := safetyBackups.InsertMany(ctx, backupDocuments, options.InsertMany().SetOrdered(false))
		if err != nil && !mongo.IsDuplicateKeyError(err) {
			return fmt.Errorf("preserve forms before weight correction: %w", err)
		}
	}

	updates := bson.D{}
	for _, sectionKey := range []string{"1", "2", "3", "4"} {
		updates = append(updates, bson.E{Key: "sections_info." + sectionKey + ".weight", Value: 25})
	}
	if _, err := forms.UpdateMany(ctx,
		bson.D{{Key: "_id", Value: bson.D{{Key: "$in", Value: ids}}}},
		bson.D{{Key: "$set", Value: updates}},
	); err != nil {
		return fmt.Errorf("restore default legacy section weights: %w", err)
	}

	verifyCursor, err := forms.Find(ctx, bson.D{{Key: "_id", Value: bson.D{{Key: "$in", Value: ids}}}},
		options.Find().SetBatchSize(100))
	if err != nil {
		return fmt.Errorf("find legacy forms for weight verification: %w", err)
	}
	defer verifyCursor.Close(ctx)
	verified := 0
	for verifyCursor.Next(ctx) {
		var current struct {
			ID           bson.ObjectID `bson:"_id"`
			SectionsInfo bson.D        `bson:"sections_info"`
		}
		if err := verifyCursor.Decode(&current); err != nil {
			return fmt.Errorf("decode legacy form during weight verification: %w", err)
		}
		for _, key := range []string{"1", "2", "3", "4"} {
			var metadata bson.D
			for _, entry := range current.SectionsInfo {
				if entry.Key == key {
					metadata, _ = entry.Value.(bson.D)
					break
				}
			}
			weight := -1
			for _, entry := range metadata {
				if entry.Key == "weight" {
					switch value := entry.Value.(type) {
					case int32:
						weight = int(value)
					case int64:
						weight = int(value)
					case int:
						weight = value
					}
				}
			}
			if weight != 25 {
				return fmt.Errorf("form %s section %s weight verification failed: got %d, want 25", current.ID.Hex(), key, weight)
			}
		}
		verified++
	}
	if err := verifyCursor.Err(); err != nil {
		return fmt.Errorf("iterate forms during weight verification: %w", err)
	}
	if verified != len(ids) {
		return fmt.Errorf("weight verification covered %d of %d legacy forms", verified, len(ids))
	}
	return nil
}
