package configs

import (
	"dainxor/atv/logger"
	"dainxor/atv/models"
	"dainxor/atv/types"
	"dainxor/atv/utils"
	"errors"

	"os"
)

type dbNS struct {
	dbType           string
	accessor         InterfaceDBAccessor
	name             string
	connectionString string
}

var DB dbNS
var accessors = make(map[string]InterfaceDBAccessor)

func init() {
	DB.DefineAccessor("MONGO", NewMongoAccessor())

	if err := DB.LoadEnv(); err != nil {
		logger.Error("Failed to load DB config from environment:", err)
	}
}

// DefineAccessor registers a new database accessor for a given database type
func (dbNS) DefineAccessor(dbType string, accessor InterfaceDBAccessor) error {
	if dbType == "" {
		logger.Error("Database type is empty")
		return errors.New("Database type is empty")
	}
	if accessor == nil {
		logger.Error("Database accessor is nil")
		return errors.New("Database accessor is nil")
	}

	dbType = utils.ToScreamingSnakeCase(dbType)
	logger.Debug("Defining database accessor for type:", dbType)
	if _, exists := accessors[dbType]; exists {
		logger.Warning("Overwriting existing accessor for type:", dbType)
	}
	accessors[dbType] = accessor

	return nil
}

// LoadDBConfig loads the database configuration from environment variables
// and sets the default values if not found. It also sets the database type.
func (dbNS) LoadEnv() error {
	var present bool
	var err error

	DB.dbType, present = os.LookupEnv("DB_TYPE")
	if !present {
		logger.Error("Database type is not set in environment variables")
		err = errors.New("Database type is not set")
	}
	logger.Debug("Using database type:", DB.dbType)

	DB.connectionString, present = os.LookupEnv(DB.dbType + "_STRING")
	if !present {
		logger.Error("Database connection string is not set in environment variables")
		err = errors.New("Database connection string is not set")
	}
	logger.Debug("Database connection string configured")

	DB.name, present = os.LookupEnv("DB_NAME")
	if !present {
		logger.Error("Database name is not set in environment variables")
		err = errors.New("Database name is not set")

	} else {
		dbEnvName := App.Environment()

		// In case of debug mode, use development database
		if dbEnvName == App.Mode().Debug() {
			dbEnvName = App.Mode().Development()
		}

		DB.name = DB.name + "-" + dbEnvName
		logger.Debug("Using database name:", DB.name)
	}

	return err
}

// ReloadConnection reloads the database connection using the current environment variables
func (dbNS) ReloadConnection() error {
	DB.Close()
	if err := DB.LoadEnv(); err != nil {
		logger.Error("Failed to load DB config from environment:", err)
		return err
	}
	if err := DB.Start(); err != nil {
		logger.Error("Failed to reload DB connection:", err)
		return err
	}
	return nil
}

/*
	Database accessor management
*/

func (dbNS) Use(db InterfaceDBAccessor) *dbNS {
	DB.accessor = db
	return &DB
}

/*
	Database connection management
*/

func (dbNS) Start() error {
	if DB.accessor == nil {
		if len(accessors) == 0 || accessors[DB.dbType] == nil {
			logger.Error("Database accessor is not registered for type", DB.dbType)
			return errors.New("database accessor not registered for type " + DB.dbType)
		}
		DB.accessor = accessors[DB.dbType]
	}

	if DB.connectionString == "" || DB.name == "" {
		logger.Error("Database connection string or name is not set")
		return errors.New("Database connection string or name is not set")
	}

	logger.Debug("Connecting to database:", DB.name)
	logger.Debug("Database connection string configured")
	logger.Debug("Using accessor type:", DB.dbType)
	return DB.accessor.Connect(DB.name, DB.connectionString)
}
func (dbNS) ConnectTo(dbName, connectionString string) error {
	DB.connectionString = connectionString
	DB.name = dbName

	return DB.Start()
}
func (dbNS) ConnectOnce(dbName, connectionString string) error {
	if DB.accessor == nil {
		logger.Error("Database accessor is not set")
		return errors.New("Database accessor is not set")
	}

	return DB.accessor.Connect(dbName, connectionString)
}

/*
	Database operations
*/

func (dbNS) InsertOne(document models.DBModelInterface) types.Result[models.DBID] {
	return DB.accessor.InsertOne(document)
}
func (dbNS) InsertMany(documents ...models.DBModelInterface) types.Result[[]models.DBID] {
	return DB.accessor.InsertMany(documents...)
}

func (dbNS) FindOne(filter any, result models.DBModelInterface) types.Result[models.DBModelInterface] {
	return DB.accessor.FindOne(filter, result)
}
func (dbNS) FindAll(filter any, result models.DBModelInterface) types.Result[[]models.DBModelInterface] {
	return DB.accessor.FindMany(filter, result)
}

func (dbNS) UpdateOne(filter any, update models.DBModelInterface) error {
	return DB.accessor.UpdateOne(filter, update)
}
func (dbNS) UpdateMany(filter any, update models.DBModelInterface) error {
	return DB.accessor.UpdateMany(filter, update)
}

func (dbNS) PatchOne(filter any, update models.DBModelInterface) error {
	return DB.accessor.PatchOne(filter, update)
}
func (dbNS) PatchMany(filter any, update models.DBModelInterface) error {
	return DB.accessor.PatchMany(filter, update)
}

func (dbNS) SoftDeleteOne(filter any, model models.DBModelInterface) error {
	return DB.accessor.SoftDeleteOne(filter, model)
}
func (dbNS) SoftDeleteMany(filter any, model models.DBModelInterface) error {
	return DB.accessor.SoftDeleteMany(filter, model)
}

func (dbNS) PermanentDeleteOne(filter any, model models.DBModelInterface) error {
	return DB.accessor.PermanentDeleteOne(filter, model)
}
func (dbNS) PermanentDeleteMany(filter any, model models.DBModelInterface) error {
	return DB.accessor.PermanentDeleteMany(filter, model)
}

// Migrate performs database migrations for the provided models
// It uses the gorm library to automatically migrate the models to the database
func (dbNS) Migrate(models ...models.DBModelInterface) error {
	logger.Info("Starting migrations")

	if err := DB.accessor.Migrate(models...); err != nil {
		logger.Error("Migration failed:", err)
		return err
	}

	logger.Info("Migrations completed")
	return nil
}

func (dbNS) Close() error {
	if DB.accessor == nil {
		logger.Error("Database accessor is not set")
		return errors.New("Database accessor is not set")
	}

	return DB.accessor.Disconnect()
}

func (dbNS) EnsureAuthIndexes() error {
	indexer, ok := DB.accessor.(interface{ EnsureAuthIndexes() error })
	if !ok {
		return errors.New("database accessor does not support authentication indexes")
	}
	return indexer.EnsureAuthIndexes()
}
