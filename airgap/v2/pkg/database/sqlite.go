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
	"fmt"

	"github.com/go-logr/logr"
	v1 "github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/model/v1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/pkg/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type SqliteDatabase struct {
	DB  *gorm.DB
	Log logr.Logger
}

type SqliteConfig struct {
	Name string
	Log  logr.Logger
}

func (db *SqliteConfig) InitDB() (*SqliteDatabase, error) {
	var database SqliteDatabase
	gormDb, err := gorm.Open(sqlite.Open(db.Name), &gorm.Config{})
	if err != nil {
		db.Log.Error(err, "Error during creation of Database")
		return nil, err
	}
	database.DB = gormDb
	database.Log = db.Log
	return &database, nil
}

func (db *SqliteDatabase) Close() {
	sqliteDB, err := db.DB.DB()
	if err != nil {
		db.Log.Error(err, "Error: Failed at close Sqlite database")
	}
	sqliteDB.Close()
}

func (db *SqliteDatabase) CreateModels() error {
	err := db.DB.AutoMigrate(&models.FileMetadata{}, &models.File{}, &models.Metadata{})
	if err != nil {
		db.Log.Error(err, "Error during creation of Models: %v")
		return err
	}
	return nil
}

func (db *SqliteDatabase) SaveFile(finfo *v1.FileInfo, bs []byte) error {
	// Create a slice of file metadata models
	var fms []models.FileMetadata
	m := finfo.GetMetadata()
	for k, v := range m {
		fm := models.FileMetadata{
			Key:   k,
			Value: v,
		}
		fms = append(fms, fm)
	}

	// Create metadata along with associations
	metadata := models.Metadata{
		ProvidedId:      finfo.GetFileId().GetId(),
		ProvidedName:    finfo.GetFileId().GetName(),
		Size:            finfo.GetSize(),
		Compression:     finfo.GetCompression(),
		CompressionType: finfo.GetCompressionType(),
		File: models.File{
			Content: bs,
		},
		FileMetadata: fms,
	}
	err := db.DB.Create(&metadata).Error
	if err != nil {
		db.Log.Error(err, "Failed to save model")
		return err
	}

	db.Log.Info(fmt.Sprintf("File of size: %v saved with id: %v", metadata.Size, metadata.FileID))
	return nil
}
