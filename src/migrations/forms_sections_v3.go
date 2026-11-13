package migrations

import (
	"context"
	"fmt"
	"reflect"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const formsSectionsV3Source = "v3: restore legacy questions from immutable migration backup into sections key 1 only and verify"

type legacyFormBackup struct {
	ID       bson.ObjectID `bson:"_id"`
	Original bson.M        `bson:"original"`
}

func restoreLegacyFormsToAcademicSection(ctx context.Context, db *mongo.Database) error {
	backups := db.Collection("schema_backup_forms_v1")
	forms := db.Collection("forms")

	cursor, err := backups.Find(ctx,
		bson.D{{Key: "source_collection", Value: "forms"}, {Key: "migration_version", Value: int32(1)}},
		options.Find().SetBatchSize(100),
	)
	if err != nil {
		return fmt.Errorf("find legacy form backups: %w", err)
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var backup legacyFormBackup
		if err := cursor.Decode(&backup); err != nil {
			return fmt.Errorf("decode legacy form backup: %w", err)
		}
		questions, ok := backup.Original["questions_info"]
		if !ok {
			return fmt.Errorf("backup for form %s has no questions_info", backup.ID.Hex())
		}
		if _, ok := questions.(bson.A); !ok {
			return fmt.Errorf("backup for form %s has unexpected questions_info type %T", backup.ID.Hex(), questions)
		}

		update := bson.D{{Key: "$set", Value: bson.D{
			{Key: "sections", Value: bson.D{{Key: "1", Value: questions}}},
		}}}
		result, err := forms.UpdateOne(ctx, bson.D{{Key: "_id", Value: backup.ID}}, update)
		if err != nil {
			return fmt.Errorf("assign legacy questions to Académico for form %s: %w", backup.ID.Hex(), err)
		}
		if result.MatchedCount != 1 {
			return fmt.Errorf("legacy form %s was not found during section assignment", backup.ID.Hex())
		}
	}

	if err := cursor.Err(); err != nil {
		return fmt.Errorf("iterate legacy form backups: %w", err)
	}

	// Verify the migrated source documents against the preserved backup.
	verifyCursor, err := backups.Find(ctx,
		bson.D{{Key: "source_collection", Value: "forms"}, {Key: "migration_version", Value: int32(1)}},
		options.Find().SetBatchSize(100),
	)
	if err != nil {
		return fmt.Errorf("find legacy form backups for verification: %w", err)
	}
	defer verifyCursor.Close(ctx)

	for verifyCursor.Next(ctx) {
		var backup legacyFormBackup
		if err := verifyCursor.Decode(&backup); err != nil {
			return fmt.Errorf("decode legacy form backup during verification: %w", err)
		}
		var current struct {
			Sections bson.D `bson:"sections"`
		}
		if err := forms.FindOne(ctx, bson.D{{Key: "_id", Value: backup.ID}}).Decode(&current); err != nil {
			return fmt.Errorf("read migrated form %s: %w", backup.ID.Hex(), err)
		}
		if len(current.Sections) != 1 || current.Sections[0].Key != "1" {
			return fmt.Errorf("form %s does not contain only Académico after migration", backup.ID.Hex())
		}
		expected := backup.Original["questions_info"].(bson.A)
		actual, ok := current.Sections[0].Value.(bson.A)
		if !ok || !reflect.DeepEqual(actual, expected) {
			return fmt.Errorf("form %s question data differs from the preserved legacy list", backup.ID.Hex())
		}
	}
	if err := verifyCursor.Err(); err != nil {
		return fmt.Errorf("verify legacy form section assignments: %w", err)
	}
	return nil
}
