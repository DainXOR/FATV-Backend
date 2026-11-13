package migrations

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const formsSectionsV1Source = "v1: backup legacy forms; move questions_info to sections.1; default positions_are_global=false; unset questions_info"

func migrateFormsSectionsV1(ctx context.Context, db *mongo.Database) error {
	forms := db.Collection("forms")
	backup := db.Collection("schema_backup_forms_v1")

	// Preserve complete original BSON before changing any source row.
	cursor, err := forms.Find(ctx, bson.D{{Key: "questions_info", Value: bson.D{{Key: "$exists", Value: true}}}}, options.Find().SetBatchSize(100))
	if err != nil {
		return fmt.Errorf("find legacy forms for backup: %w", err)
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var original bson.Raw
		if err := cursor.Decode(&original); err != nil {
			return fmt.Errorf("decode legacy form for backup: %w", err)
		}
		var identity struct {
			ID bson.ObjectID `bson:"_id"`
		}
		if err := bson.Unmarshal(original, &identity); err != nil {
			return fmt.Errorf("read legacy form identity: %w", err)
		}
		document := bson.D{
			{Key: "_id", Value: identity.ID},
			{Key: "source_collection", Value: "forms"},
			{Key: "migration_version", Value: int32(1)},
			{Key: "backed_up_at", Value: time.Now().UTC()},
			{Key: "original", Value: original},
		}
		_, err = backup.InsertOne(ctx, document)
		if mongo.IsDuplicateKeyError(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("backup legacy form %s: %w", identity.ID.Hex(), err)
		}
	}
	if err := cursor.Err(); err != nil {
		return fmt.Errorf("iterate legacy forms for backup: %w", err)
	}

	// Legacy questions_info did not retain section membership. The agreed
	// compatibility rule assigns those questions to section 1 (Académico).
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$set", Value: bson.D{
			{Key: "sections", Value: bson.D{{Key: "1", Value: "$questions_info"}}},
			{Key: "positions_are_global", Value: bson.D{{Key: "$ifNull", Value: bson.A{"$positions_are_global", false}}}},
		}}},
		bson.D{{Key: "$unset", Value: "questions_info"}},
	}
	if _, err := forms.UpdateMany(ctx, bson.D{{Key: "questions_info", Value: bson.D{{Key: "$exists", Value: true}}}}, pipeline); err != nil {
		return fmt.Errorf("migrate legacy form sections: %w", err)
	}
	if _, err := forms.UpdateMany(ctx,
		bson.D{{Key: "positions_are_global", Value: bson.D{{Key: "$exists", Value: false}}}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "positions_are_global", Value: false}}}},
	); err != nil {
		return fmt.Errorf("set default form position mode: %w", err)
	}
	return nil
}

const formsSchemaV2Source = "v2: apply moderate error jsonSchema validation to forms.sections, sections_info and positions_are_global"

func applyFormsSchemaV2(ctx context.Context, db *mongo.Database) error {
	validator := bson.D{{Key: "$jsonSchema", Value: bson.D{
		{Key: "bsonType", Value: "object"},
		{Key: "properties", Value: bson.D{
			{Key: "sections", Value: bson.D{{Key: "bsonType", Value: "object"}}},
			{Key: "sections_info", Value: bson.D{{Key: "bsonType", Value: "object"}}},
			{Key: "positions_are_global", Value: bson.D{{Key: "bsonType", Value: "bool"}}},
		}},
	}}}
	names, err := db.ListCollectionNames(ctx, bson.D{{Key: "name", Value: "forms"}})
	if err != nil {
		return fmt.Errorf("list forms collection: %w", err)
	}
	if len(names) == 0 {
		err := db.CreateCollection(ctx, "forms", options.CreateCollection().
			SetValidator(validator).SetValidationLevel("moderate").SetValidationAction("error"))
		if err != nil {
			return fmt.Errorf("create forms collection with validator: %w", err)
		}
		return nil
	}
	command := bson.D{
		{Key: "collMod", Value: "forms"},
		{Key: "validator", Value: validator},
		{Key: "validationLevel", Value: "moderate"},
		{Key: "validationAction", Value: "error"},
	}
	if err := db.RunCommand(ctx, command).Err(); err != nil {
		return fmt.Errorf("apply forms JSON schema validator: %w", err)
	}
	return nil
}

func registered() []Migration {
	return []Migration{
		{Version: 1, Name: "normalize_form_sections", Checksum: checksum(formsSectionsV1Source), Apply: migrateFormsSectionsV1},
		{Version: 2, Name: "validate_form_section_schema", Checksum: checksum(formsSchemaV2Source), Apply: applyFormsSchemaV2},
		{Version: 3, Name: "restore_legacy_questions_to_academic_section", Checksum: checksum(formsSectionsV3Source), Apply: restoreLegacyFormsToAcademicSection},
		{Version: 4, Name: "normalize_all_form_sections_and_metadata", Checksum: checksum(formsSectionsV4Source), Apply: normalizeAllFormSections},
		{Version: 5, Name: "normalize_legacy_form_question_parents", Checksum: checksum(formsSectionsV5Source), Apply: normalizeLegacyQuestionParents},
		{Version: 6, Name: "repair_legacy_session_dates", Checksum: checksum(sessionsV6Source), Apply: repairLegacySessionDates},
		{Version: 7, Name: "restore_default_legacy_section_weights", Checksum: checksum(formsSectionsV7Source), Apply: restoreDefaultLegacySectionWeights},
	}
}
