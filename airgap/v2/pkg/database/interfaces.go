package database

import (
	v1 "github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/model/v1"
)

type GenericDatabase interface {
	SaveFile(*v1.FileInfo, []byte) error
	Close()
	CreateModels() error
}

type GenericDatabaseConfig interface {
	InitDB() (GenericDatabase, error)
}

type DatabaseWrapper struct {
	Database       GenericDatabase
	DatabaseConfig GenericDatabaseConfig
}

func (db DatabaseWrapper) InitDB() error {
	//Initialize Database
	genDB, err := db.DatabaseConfig.InitDB()
	db.Database = genDB
	if err != nil {
		return err
	}
	//Create models
	err = db.Database.CreateModels()
	if err != nil {
		return err
	}

	return nil
}

func (db DatabaseWrapper) SaveFile(finfo *v1.FileInfo, bs []byte) error {
	err := db.Database.SaveFile(finfo, bs)
	return err
}

func (db DatabaseWrapper) Close() {
	db.Database.Close()
}
