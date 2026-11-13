# MongoDB schema migrations

Migrations are Go functions registered in registered() and run automatically
by the MongoDB accessor during configs.DB.Start(). A failed migration aborts
database startup; the API must not serve traffic against a partially upgraded
schema.

## Adding a migration

1. Add an immutable migration function in a focused *_vN.go file.
2. Give it the next contiguous positive version and a stable name.
3. Add a stable checksum source string describing the migration's exact
   behavior. Update that string whenever the migration implementation changes
   before it has shipped. Never edit a migration after it has been applied;
   add a new version instead.
4. Make the migration idempotent. MongoDB transactions are not assumed because
   standalone deployments do not support multi-document transactions.
5. Back up original documents before destructive transformations, verify
   prerequisites and postconditions, and return errors rather than silently
   skipping failures.
6. Add unit tests for pure migration logic and, where practical, integration
   tests against a disposable MongoDB database.

Applied versions, names, checksums, and timestamps are stored in
atv_schema_migrations. The unique version index prevents duplicate history rows.
Legacy form documents are preserved in schema_backup_forms_v1 by migration 1; migration 4 fills missing section arrays and metadata with the four configured Spanish section labels without overwriting existing values; migration 5 converts legacy flat question-parent fields to the current nested parent structure; migration 6 backs up every session and repairs placeholder date values from parseable legacy date strings, leaving sentinel dates unchanged; migration 7 restores the form creator's 25% section-weight default only where legacy forms had no original section metadata;
do not remove that backup collection as part of routine migrations.

MongoDB $jsonSchema validation is installed by migrations too. It validates
document shape at the database boundary; it complements rather than replaces
the versioned data migrations. Do not use configs.DB.Migrate for automatic
model reflection; it intentionally returns an error so schema changes cannot
silently bypass this migration history.

For local setup-code display, set FATV_DEV_CODE_ENCRYPTION_KEY to a random
secret of at least 32 characters (for example, generate one with
`openssl rand -hex 32`). Keep it only in the local environment file and do
not commit it. If the key changes, issue fresh setup codes.
