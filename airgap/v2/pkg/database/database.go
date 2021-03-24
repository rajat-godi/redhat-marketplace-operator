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
	"github.com/pkg/errors"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/pkg/driver/dqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type Database struct {
	DB       *gorm.DB //GORM connection. Other packages can only access this
	dqliteDB *sql.DB  //The underlying connection to dqlite of type *sql.DB
	app      *app.App //Data used to set up the nodes in dqlite
}

/*
Initialize the GORM onnection and return fully populated Database object
*/
func InitDB(name string, dir string, url string, join []string, verbose bool) (*Database, error) {
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

/*
Initialize the underlying dqlite database and populate a *Database object with the dqlite connection and app.
Which will be used later for closing the connection
*/
func initDqlite(name string, dir string, url string, join []string, verbose bool) (*Database, error) {
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

	app, err := app.New(dir, app.WithAddress(url), app.WithCluster(join), app.WithLogFunc(logFunc))
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

/*
Close connection to the database, also handsover the leadership to another node running dqlite
*/
func (d *Database) Close() {
	if d != nil {
		d.dqliteDB.Close()
		d.app.Handover(context.Background())
		d.app.Close()
	}
}
