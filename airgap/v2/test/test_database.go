package main

import (
	"fmt"
	"log"

	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/pkg/database"
)

type File struct {
	ID      string `gorm:"primaryKey"`
	Content []byte
}
type Metadata struct {
	ID                  string `gorm:"primaryKey"`
	ProvidedId          string
	ProvidedName        string
	Size                uint
	Compression         bool
	CompressionType     string
	CleanTombstoneSetAt int64
	CreatedAt           int64
	DeletedAt           int64
	File                File `gorm:"foreignKey:ID"`
}
type FileMetadata struct {
	ID       string   `gorm:"primaryKey"`
	Metadata Metadata `gorm:"foreignKey:ID"`
	Key      string   `gorm:"index"`
	Value    string
}

func main() {
	dbName := "airgap"
	dir := "db"
	socket := "127.0.0.1:9001" // Unique node address
	cluster := []string{}      // Optional list of existing nodes, when starting a new node

	ormDb, err := database.InitDB(dbName, dir, socket, cluster, true)
	//ormDb.DB.SingularTable(true)

	if err != nil {
		log.Fatalf("Error furing creation of db structrue: %v", err)
	}
	fmt.Printf("ormDb type is %T", ormDb)

	// Migrate the schema
	ormDb.DB.AutoMigrate(&File{})
	ormDb.DB.AutoMigrate(&Metadata{})
	ormDb.DB.AutoMigrate(&FileMetadata{})

	file := File{ID: "file1", Content: []byte("Hippity Poppity!")}
	metadata := Metadata{ID: "metadata1", ProvidedId: "123hft", ProvidedName: "dummy", Size: 10, Compression: false, CompressionType: "We don't do compression", CleanTombstoneSetAt: 1000, CreatedAt: 1000, DeletedAt: 0, File: file}
	fileMetadata := FileMetadata{ID: "file_metadata1", Metadata: metadata, Key: "type", Value: "Magic damage"}
	ormDb.DB.Create(&file)
	//file = File{ID: "file2", Content: []byte("Hippity Poppity-12121!")}
	//ormDb.DB.Create(&file)
	ormDb.DB.Create(&metadata)
	ormDb.DB.Create(&fileMetadata)

	//var db_file File
	var db_metadata Metadata
	//var db_fileMetadata FileMetadata

	//ormDb.DB.First(&db_file, "id = ?", "file1")
	ormDb.DB.Preload("File").First(&db_metadata)
	//ormDb.DB.First(&db_fileMetadata, "id = ?", "file_metadata1")
	//var db_files []File
	//ormDb.DB.Model(db_metadata).Association("File").Find(&db_files)

	//fmt.Println(db_file)
	fmt.Println(db_metadata)
	//fmt.Println(db_files)

	//ormDb.DB.Model(&fileData).Update("FileDeletedAt", 1000)
	//ormDb.DB.Model(&fileStore).Updates(FileStore{Content: []byte{}})

	// Update - update multiple fields
	//ormDb.Model(&product).Updates(Product{Price: 200, Code: "F42"}) // non-zero fields
	//ormDb.Model(&product).Updates(map[string]interface{}{"Price": 200, "Code": "F42"})

	// Delete - delete product
	//ormDb.Delete(&product, 1)
}
