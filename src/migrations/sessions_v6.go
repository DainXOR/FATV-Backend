package migrations

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const sessionsV6Source = "v6: back up sessions and repair placeholder date fields from parseable depr_date values without deleting legacy data"

type legacySessionDate struct {
	ID       bson.ObjectID `bson:"_id"`
	Date     time.Time     `bson:"date"`
	DeprDate string        `bson:"depr_date"`
}

func parseLegacySessionDate(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"02-01-2006T15:04",
		"02/01/2006 15:04:05",
		"2/01/2006 15:04:05",
	}
	for _, layout := range layouts {
		var parsed time.Time
		var err error
		if strings.Contains(layout, "Z07:00") {
			parsed, err = time.Parse(layout, raw)
		} else {
			parsed, err = time.ParseInLocation(layout, raw, time.Local)
		}
		if err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported legacy session date %q", raw)
}

func isSessionDatePlaceholder(value time.Time) bool {
	return value.IsZero() || (value.Year() == 2000 && value.Month() == time.January && value.Day() == 1)
}

func isLegacyDateSentinel(value time.Time) bool {
	return value.IsZero() || (value.Year() == 2000 && value.Month() == time.January && value.Day() == 1)
}

func repairLegacySessionDates(ctx context.Context, db *mongo.Database) error {
	sessions := db.Collection("sessions")
	backups := db.Collection("schema_backup_sessions_v6")
	cursor, err := sessions.Find(ctx, bson.D{}, options.Find().SetBatchSize(100))
	if err != nil {
		return fmt.Errorf("find sessions for date audit: %w", err)
	}
	defer cursor.Close(ctx)

	updates := make(map[bson.ObjectID]time.Time)
	for cursor.Next(ctx) {
		var original bson.Raw
		if err := cursor.Decode(&original); err != nil {
			return fmt.Errorf("decode original session for backup: %w", err)
		}
		var session legacySessionDate
		if err := bson.Unmarshal(original, &session); err != nil {
			return fmt.Errorf("decode session date fields: %w", err)
		}
		backup := bson.D{
			{Key: "_id", Value: session.ID},
			{Key: "source_collection", Value: "sessions"},
			{Key: "migration_version", Value: int32(6)},
			{Key: "backed_up_at", Value: time.Now().UTC()},
			{Key: "original", Value: original},
		}
		if _, err := backups.InsertOne(ctx, backup); err != nil && !mongo.IsDuplicateKeyError(err) {
			return fmt.Errorf("backup session %s: %w", session.ID.Hex(), err)
		}

		if !isSessionDatePlaceholder(session.Date) || session.DeprDate == "" {
			continue
		}
		parsed, err := parseLegacySessionDate(session.DeprDate)
		if err != nil {
			return fmt.Errorf("session %s has an unparseable depr_date: %w", session.ID.Hex(), err)
		}
		if isLegacyDateSentinel(parsed) {
			continue
		}
		updates[session.ID] = parsed
	}
	if err := cursor.Err(); err != nil {
		return fmt.Errorf("iterate sessions for date audit: %w", err)
	}

	for id, expected := range updates {
		if _, err := sessions.UpdateOne(ctx, bson.D{{Key: "_id", Value: id}},
			bson.D{{Key: "$set", Value: bson.D{
				{Key: "date", Value: expected},
				{Key: "updated_at", Value: time.Now().UTC()},
			}}}); err != nil {
			return fmt.Errorf("repair session date %s: %w", id.Hex(), err)
		}
	}

	for id, expected := range updates {
		var actual struct {
			Date time.Time `bson:"date"`
		}
		if err := sessions.FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&actual); err != nil {
			return fmt.Errorf("verify repaired session %s: %w", id.Hex(), err)
		}
		if !actual.Date.Equal(expected) {
			return fmt.Errorf("session %s date verification failed: expected %s, got %s", id.Hex(), expected, actual.Date)
		}
	}
	return nil
}
