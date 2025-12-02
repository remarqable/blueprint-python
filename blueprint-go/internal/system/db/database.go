package db

import (
	"log"
	"sync"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	instance *gorm.DB
	once     sync.Once
)

// Initialize sets up the GORM database connection
func Initialize(databaseURL string, debug bool) error {
	var initErr error

	once.Do(func() {
		logLevel := logger.Silent
		if debug {
			logLevel = logger.Info
		}

		config := &gorm.Config{
			Logger: logger.Default.LogMode(logLevel),
		}

		db, err := gorm.Open(sqlite.Open(databaseURL), config)
		if err != nil {
			initErr = err
			return
		}

		instance = db
		log.Printf("Database connected: %s", databaseURL)
	})

	return initErr
}

// Get returns the GORM database instance
func Get() *gorm.DB {
	if instance == nil {
		log.Fatal("Database not initialized. Call Initialize() first.")
	}
	return instance
}

// AutoMigrate runs auto-migration for the given models
func AutoMigrate(models ...interface{}) error {
	return Get().AutoMigrate(models...)
}

// Close closes the database connection
func Close() error {
	if instance == nil {
		return nil
	}

	sqlDB, err := instance.DB()
	if err != nil {
		return err
	}

	return sqlDB.Close()
}

// Transaction runs a function within a GORM transaction
func Transaction(fn func(tx *gorm.DB) error) error {
	return Get().Transaction(fn)
}
