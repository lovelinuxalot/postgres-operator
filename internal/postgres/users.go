package postgres

import (
	"database/sql"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

// CreateUser creates a new PostgreSQL user and grants access to the specified database
func CreateUser(db *sql.DB, username, password, database, roleType string) error {
	// Create the role with login and the specified password
	createRoleQuery := fmt.Sprintf("CREATE ROLE %s WITH LOGIN PASSWORD '%s'", QuoteIdentifier(username), password)
	_, err := db.Exec(createRoleQuery)
	if err != nil {
		log.Log.Error(err, "Failed to create user")
		return err
	}

	// Grant privileges based on the role type
	switch roleType {
	case "owner":
		err = grantOwnerPrivileges(db, username, database)
	case "read":
		err = grantReadPrivileges(db, username, database)
	case "write":
		err = grantWritePrivileges(db, username, database)
	default:
		return fmt.Errorf("invalid roleType: %s", roleType)
	}

	if err != nil {
		return fmt.Errorf("failed to grant privileges to role: %w", err)
	}

	return nil
}

// DropUser drops an existing PostgreSQL user
func DropUser(db *sql.DB, username string) error {
	// Drop the user if it exists
	dropUserQuery := fmt.Sprintf("DROP USER IF EXISTS %s", QuoteIdentifier(username))
	_, err := db.Exec(dropUserQuery)
	if err != nil {
		return fmt.Errorf("failed to drop user: %w", err)
	}

	return nil
}

// QuoteIdentifier safely quotes a PostgreSQL identifier (e.g., table name, column name, etc.)
func QuoteIdentifier(identifier string) string {
	return fmt.Sprintf(`"%s"`, identifier)
}

// grantOwnerPrivileges grants all privileges to the role
func grantOwnerPrivileges(db *sql.DB, username, database string) error {
	grantPrivilegesQuery := fmt.Sprintf("GRANT ALL PRIVILEGES ON DATABASE %s TO %s", QuoteIdentifier(database), QuoteIdentifier(username))
	_, err := db.Exec(grantPrivilegesQuery)
	return err
}

// grantReadPrivileges grants read-only privileges to the role
func grantReadPrivileges(db *sql.DB, username, database string) error {
	grantSelectQuery := fmt.Sprintf("GRANT CONNECT ON DATABASE %s TO %s; GRANT USAGE ON SCHEMA public TO %s; GRANT SELECT ON ALL TABLES IN SCHEMA public TO %s;", QuoteIdentifier(database), QuoteIdentifier(username), QuoteIdentifier(username), QuoteIdentifier(username))
	_, err := db.Exec(grantSelectQuery)
	if err != nil {
		return err
	}
	// Ensure future tables in this schema get the same privileges
	alterDefaultPrivilegesQuery := fmt.Sprintf("ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO %s;", QuoteIdentifier(username))
	_, err = db.Exec(alterDefaultPrivilegesQuery)
	return err
}

// grantWritePrivileges grants write privileges to the role
func grantWritePrivileges(db *sql.DB, username, database string) error {
	grantWriteQuery := fmt.Sprintf("GRANT CONNECT ON DATABASE %s TO %s; GRANT USAGE ON SCHEMA public TO %s; GRANT INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO %s;", QuoteIdentifier(database), QuoteIdentifier(username), QuoteIdentifier(username), QuoteIdentifier(username))
	_, err := db.Exec(grantWriteQuery)
	if err != nil {
		return err
	}
	// Ensure future tables in this schema get the same privileges
	alterDefaultPrivilegesQuery := fmt.Sprintf("ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT INSERT, UPDATE, DELETE ON TABLES TO %s;", QuoteIdentifier(username))
	_, err = db.Exec(alterDefaultPrivilegesQuery)
	return err
}
