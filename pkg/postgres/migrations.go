package postgres

import (
	"fmt"
	"log"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

const filePrefix = "file://"
const errCreateMigration = "Failed to create migration instance: %v"

// RunMigrationsUp applies all up migrations
func (db *DB) RunMigrationsUp(migrationsPath string) error {
	fmt.Println("Running up migrations...", maskDSN(db.DSN))
	m, err := migrate.New(
		filePrefix+migrationsPath,
		db.DSN,
	)
	if err != nil {
		log.Printf(errCreateMigration, err)
		return err
	}
	if err := m.Up(); err != nil {
		if err.Error() != "no change" {
			log.Printf("Failed to apply up migrations: %v", err)
			return err
		}
		log.Println("No new up migrations to apply")
	} else {
		log.Println("Up migrations applied successfully")
	}
	return nil
}

// RunMigrationsDown rolls back all migrations
func (db *DB) RunMigrationsDown(migrationsPath string) error {
	fmt.Println("Running down migrations...", maskDSN(db.DSN))
	m, err := migrate.New(
		filePrefix+migrationsPath,
		db.DSN,
	)
	if err != nil {
		log.Printf(errCreateMigration, err)
		return err
	}
	if err := m.Down(); err != nil {
		if err.Error() != "no change" {
			log.Printf("Failed to apply down migrations: %v", err)
			return err
		}
		log.Println("No down migrations to apply")
	} else {
		log.Println("Down migrations applied successfully")
	}
	return nil
}

// RunMigrationsSteps applies N up or down steps
func (db *DB) RunMigrationsSteps(migrationsPath string, steps int) error {
	fmt.Println("Running migration steps...", maskDSN(db.DSN))
	m, err := migrate.New(
		filePrefix+migrationsPath,
		db.DSN,
	)
	if err != nil {
		log.Printf(errCreateMigration, err)
		return err
	}
	if err := m.Steps(steps); err != nil {
		log.Printf("Failed to apply migration steps: %v", err)
		return err
	}
	log.Printf("Migration steps (%d) applied successfully", steps)
	return nil
}

// RunMigrationsForce forces the migration version
func (db *DB) RunMigrationsForce(migrationsPath string, version int) error {
	fmt.Println("Forcing migration version...", maskDSN(db.DSN))
	m, err := migrate.New(
		filePrefix+migrationsPath,
		db.DSN,
	)
	if err != nil {
		log.Printf(errCreateMigration, err)
		return err
	}
	if err := m.Force(version); err != nil {
		log.Printf("Failed to force migration version: %v", err)
		return err
	}
	log.Printf("Migration version forced to %d", version)
	return nil
}

// RunMigrationsVersion returns the current migration version
func (db *DB) RunMigrationsVersion(migrationsPath string) (uint, bool, error) {
	fmt.Println("Getting migration version...", maskDSN(db.DSN))
	m, err := migrate.New(
		filePrefix+migrationsPath,
		db.DSN,
	)
	if err != nil {
		log.Printf(errCreateMigration, err)
		return 0, false, err
	}
	version, dirty, err := m.Version()
	if err != nil {
		log.Printf("Failed to get migration version: %v", err)
		return 0, false, err
	}
	log.Printf("Current migration version: %d (dirty: %v)", version, dirty)
	return version, dirty, nil
}
