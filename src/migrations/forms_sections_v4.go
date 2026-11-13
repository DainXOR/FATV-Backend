package migrations

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

const formsSectionsV4Source = "v4: normalize every form to four positional sections and Spanish metadata while preserving existing values"

func normalizeAllFormSections(ctx context.Context, db *mongo.Database) error {
	defaultSections := bson.D{
		{Key: "1", Value: bson.A{}},
		{Key: "2", Value: bson.A{}},
		{Key: "3", Value: bson.A{}},
		{Key: "4", Value: bson.A{}},
	}
	defaultMetadata := bson.D{
		{Key: "1", Value: bson.D{{Key: "name", Value: "Académico"}, {Key: "weight", Value: 0}}},
		{Key: "2", Value: bson.D{{Key: "name", Value: "Socioeconómico"}, {Key: "weight", Value: 0}}},
		{Key: "3", Value: bson.D{{Key: "name", Value: "Relacional"}, {Key: "weight", Value: 0}}},
		{Key: "4", Value: bson.D{{Key: "name", Value: "Socioemocional"}, {Key: "weight", Value: 0}}},
	}
	now := time.Now().UTC()
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$set", Value: bson.D{
			{Key: "sections", Value: bson.D{{Key: "$mergeObjects", Value: bson.A{
				bson.D{{Key: "$literal", Value: defaultSections}},
				bson.D{{Key: "$ifNull", Value: bson.A{"$sections", bson.D{}}}},
			}}}},
			{Key: "sections_info", Value: bson.D{{Key: "$mergeObjects", Value: bson.A{
				bson.D{{Key: "$literal", Value: defaultMetadata}},
				bson.D{{Key: "$ifNull", Value: bson.A{"$sections_info", bson.D{}}}},
			}}}},
			{Key: "positions_are_global", Value: bson.D{{Key: "$ifNull", Value: bson.A{"$positions_are_global", false}}}},
			{Key: "updated_at", Value: now},
		}}},
	}
	if _, err := db.Collection("forms").UpdateMany(ctx, bson.D{}, pipeline); err != nil {
		return fmt.Errorf("normalize form sections and metadata: %w", err)
	}

	cursor, err := db.Collection("forms").Find(ctx, bson.D{})
	if err != nil {
		return fmt.Errorf("verify normalized forms: %w", err)
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var form struct {
			ID                 bson.ObjectID `bson:"_id"`
			Sections           bson.D        `bson:"sections"`
			SectionsInfo       bson.D        `bson:"sections_info"`
			PositionsAreGlobal bool          `bson:"positions_are_global"`
		}
		if err := cursor.Decode(&form); err != nil {
			return fmt.Errorf("decode form during normalization verification: %w", err)
		}
		if len(form.Sections) != 4 || len(form.SectionsInfo) != 4 {
			return fmt.Errorf("form %s does not have four section entries and four metadata entries", form.ID.Hex())
		}
		if form.PositionsAreGlobal && len(form.Sections) == 0 {
			return fmt.Errorf("form %s has an invalid global position configuration", form.ID.Hex())
		}
	}
	if err := cursor.Err(); err != nil {
		return fmt.Errorf("iterate normalized forms: %w", err)
	}
	return nil
}
