package postgres

import (
	"database/sql"
	"fmt"

	"github.com/lib/pq"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// CreateDatabase creates a new database if it doesn't exist
func CreateDatabase(db *sql.DB, databaseName string) error {

	// Use pq.QuoteLiteral to quote the database name as a literal string
	quotedDBName := pq.QuoteLiteral(databaseName)

	// Use pq.QuoteIdentifier to quote the database name as an SQL identifier
	quotedIdentifier := pq.QuoteIdentifier(databaseName)

	// First, check if the database exists
	var exists bool
	checkQuery := fmt.Sprintf("SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = %s)", quotedDBName)
	err := db.QueryRow(checkQuery).Scan(&exists)
	if err != nil {
		log.Log.Error(err, "Error checking if database exists: ")
	}

	// If the database does not exist, create it
	if !exists {
		createQuery := fmt.Sprintf("CREATE DATABASE %s", quotedIdentifier)
		_, err = db.Exec(createQuery)
		if err != nil {
			log.Log.Error(err, "Error creating database: ")
		}
		log.Log.Info("Database created successfully", "Database: ", databaseName)
	} else {
		log.Log.Info("Database already exists", "Database: ", databaseName)
	}

	return err
}

// DropDatabase drops the database if it exists
func DropDatabase(db *sql.DB, databaseName string) error {
	// Use pq.QuoteIdentifier to quote the database name as an SQL identifier
	quotedIdentifier := pq.QuoteIdentifier(databaseName)
	// Drop the database
	_, err := db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", quotedIdentifier))
	if err != nil {
		log.Log.Error(err, "failed to drop database ")
		return err
	}
	log.Log.Info("Database dropped successfully", "Database: ", databaseName)

	return nil
}
