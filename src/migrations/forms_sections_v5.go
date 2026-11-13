package migrations

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

const formsSectionsV5Source = "v5: convert legacy flat id_parent_question and needed_answers into nested parent object in section question records"

func normalizeLegacyQuestionParents(ctx context.Context, db *mongo.Database) error {
	fieldsToRemove := bson.A{"id_parent_question", "needed_answers", "section", "parent"}
	parent := bson.D{
		{Key: "id_question", Value: bson.D{{Key: "$ifNull", Value: bson.A{
			"$$question.parent.id_question", "$$question.id_parent_question", bson.NilObjectID,
		}}}},
		{Key: "needed_answers", Value: bson.D{{Key: "$ifNull", Value: bson.A{
			"$$question.parent.needed_answers", "$$question.needed_answers", bson.A{},
		}}}},
	}

	normalizedQuestion := bson.D{{Key: "$arrayToObject", Value: bson.D{{Key: "$concatArrays", Value: bson.A{
		bson.D{{Key: "$filter", Value: bson.D{
			{Key: "input", Value: bson.D{{Key: "$objectToArray", Value: "$$question"}}},
			{Key: "as", Value: "field"},
			{Key: "cond", Value: bson.D{{Key: "$not", Value: bson.A{
				bson.D{{Key: "$in", Value: bson.A{"$$field.k", fieldsToRemove}}},
			}}}},
		}}},
		bson.A{bson.D{{Key: "k", Value: "parent"}, {Key: "v", Value: parent}}},
	}}}}}
	normalizedSections := bson.D{{Key: "$arrayToObject", Value: bson.D{{Key: "$map", Value: bson.D{
		{Key: "input", Value: bson.D{{Key: "$objectToArray", Value: "$sections"}}},
		{Key: "as", Value: "section"},
		{Key: "in", Value: bson.D{
			{Key: "k", Value: "$$section.k"},
			{Key: "v", Value: bson.D{{Key: "$map", Value: bson.D{
				{Key: "input", Value: "$$section.v"},
				{Key: "as", Value: "question"},
				{Key: "in", Value: normalizedQuestion},
			}}}},
		}},
	}}}}}

	if _, err := db.Collection("forms").UpdateMany(ctx, bson.D{}, mongo.Pipeline{
		bson.D{{Key: "$set", Value: bson.D{{Key: "sections", Value: normalizedSections}}}},
	}); err != nil {
		return fmt.Errorf("normalize legacy form question parent fields: %w", err)
	}

	cursor, err := db.Collection("forms").Find(ctx, bson.D{})
	if err != nil {
		return fmt.Errorf("verify normalized form question parent fields: %w", err)
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var form struct {
			ID       bson.ObjectID `bson:"_id"`
			Sections bson.D        `bson:"sections"`
		}
		if err := cursor.Decode(&form); err != nil {
			return fmt.Errorf("decode form during parent verification: %w", err)
		}
		for _, section := range form.Sections {
			questions, ok := section.Value.(bson.A)
			if !ok {
				return fmt.Errorf("form %s section %s is not an array", form.ID.Hex(), section.Key)
			}
			for index, item := range questions {
				question, ok := item.(bson.D)
				if !ok {
					return fmt.Errorf("form %s section %s question %d is not a document", form.ID.Hex(), section.Key, index)
				}
				hasParent := false
				for _, field := range question {
					if field.Key == "parent" {
						hasParent = true
					}
					if field.Key == "id_parent_question" || field.Key == "needed_answers" || field.Key == "section" {
						return fmt.Errorf("form %s section %s question %d retains a legacy parent field", form.ID.Hex(), section.Key, index)
					}
				}
				if !hasParent {
					return fmt.Errorf("form %s section %s question %d has no normalized parent object", form.ID.Hex(), section.Key, index)
				}
			}
		}
	}
	if err := cursor.Err(); err != nil {
		return fmt.Errorf("iterate forms during parent verification: %w", err)
	}
	return nil
}
