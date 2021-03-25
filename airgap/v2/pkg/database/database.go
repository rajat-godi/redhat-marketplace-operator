// Copyright 2021 IBM Corp.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/canonical/go-dqlite/app"
	"github.com/canonical/go-dqlite/client"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/pkg/driver/dqlite"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/pkg/models"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type Database struct {
	DB       *gorm.DB
	dqliteDB *sql.DB
	app      *app.App
	Log      logr.Logger
}

// Initialize the GORM connection and return connected struct
func InitDB(name string, dir string, url string, join *[]string, verbose bool) (*Database, error) {
	database, err := initDqlite(name, dir, url, join, verbose)
	if err != nil {
		log.Printf("Error, during initialization of Dqlite Database: %v", err)
		return nil, err
	}

	dqliteDialector := dqlite.Open(database.dqliteDB)
	database.DB, err = gorm.Open(dqliteDialector, &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true, // use singular table name, table for `User` would be `user` with this option enabled
		},
	})
	if err != nil {
		log.Printf("Error during GORM open")
		return nil, err
	}

	return database, err
}

// Initialize the underlying dqlite database and populate a *Database object with the dqlite connection and app
func initDqlite(name string, dir string, url string, join *[]string, verbose bool) (*Database, error) {
	dir = filepath.Join(dir, url)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, errors.Wrapf(err, "can't create %s", dir)
	}
	logFunc := func(l client.LogLevel, format string, a ...interface{}) {
		if !verbose {
			return
		}
		log.Printf(fmt.Sprintf("%s: %s\n", l.String(), format), a...)
	}

	app, err := app.New(dir, app.WithAddress(url), app.WithCluster(*join), app.WithLogFunc(logFunc))
	if err != nil {
		return nil, err
	}

	if err := app.Ready(context.Background()); err != nil {
		return nil, err
	}

	conn, err := app.Open(context.Background(), name)
	if err != nil {
		return nil, err
	}

	return &Database{dqliteDB: conn, app: app}, conn.Ping()
}

// Close connection to the database and perform context handover
func (d *Database) Close() {
	if d != nil {
		d.Log.Info("Attempting graceful shutdown and handover")
		d.dqliteDB.Close()
		d.app.Handover(context.Background())
		d.app.Close()
	}
}

/*
Creates the models defined in pkg/models and returns an error, if it fails.
This function must be called after the struct Database has been fully populated
*/
func (d *Database) CreateModels() error {
	var err error

	//Check if gorm.DB is populated
	if d.DB == nil {
		errors.New("GORM connection has not initialised: Connection of type *grom.DB is nil")
	}

	//Create models
	err = d.DB.AutoMigrate(&models.FileMetadata{})
	if err != nil {
		log.Printf("Error during creation of File Metadata Model: %v", err)
		return err
	}

	err = d.DB.AutoMigrate(&models.File{})
	if err != nil {
		log.Printf("Error during creation of File Model: %v", err)
		return err
	}
	fmt.Println(models.Metadata{})
	err = d.DB.AutoMigrate(&models.Metadata{})
	if err != nil {
		log.Printf("Error during creation of Metadata Model: %v", err)
		return err
	}

	return nil
}

/*
function must accept file information for save. For this, certain things are necessary.
1. Metadata must be in the form of json. So, a json decoder.
2. Provided name
3. Provided id
4. Size
5. Compression
6. CompressionType
7. CleanTombstoneSetAt
	For this, there is a grace period after which the tombstone can be deleted. This must be set at the time of upload.
	Basically, 12 hrs + curren time.
8. Created at (Must be obtained from file itself)
9. Deleted at (Must be entered after the file is deleted. This required a new cli and an executable query)
(must be unique)

Need.
1. Current time in posix form.
2. Posix converter etc.

*/

func (d *Database) CrudTest() error {
	file := models.File{ID: "file1", Content: []byte("Hippity Poppity!")}
	fileMetadata := models.FileMetadata{ID: "file_metadata1", Key: "type", Value: "Magic damage"}
	metadata := models.Metadata{ID: "metadata1", ProvidedId: "123hft", ProvidedName: "dummy", Size: 10, Compression: false, CompressionType: "We don't do compression", CleanTombstoneSetAt: 1000, CreatedAt: 1000, DeletedAt: 0, File: file, FileMetadata: []models.FileMetadata{fileMetadata}}

	d.DB.Create(&metadata)

	var db_metadata models.Metadata

	d.DB.Preload("File").Preload("FileMetadata").First(&db_metadata)
	fmt.Println(db_metadata)

	return nil
}
