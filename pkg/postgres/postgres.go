package postgres

import (
	"database/sql"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Config holds the connection parameters for Postgres
type Config struct {
	Host     string
	Port     string
	Name     string
	Username string
	Password string
	SSLMode  string
}

// DB wraps the actual connection clients
type DB struct {
	Client *gorm.DB
	SQL    *sql.DB
	DSN    string
}

// New creates a new DB connection from user-supplied config
func New(cfg Config) (*DB, error) {
	if cfg.SSLMode == "" {
		cfg.SSLMode = "disable"
	}
	dbConnectionStr := "host=" + cfg.Host + " port=" + cfg.Port + " user=" + cfg.Username + " dbname=" + cfg.Name + " password=" + cfg.Password + " sslmode=" + cfg.SSLMode
	dsn := "postgres://" + cfg.Username + ":" + cfg.Password + "@" + cfg.Host + ":" + cfg.Port + "/" + cfg.Name + "?sslmode=" + cfg.SSLMode
	client, err := gorm.Open(postgres.Open(dbConnectionStr), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	sqlDB, err := client.DB()
	if err != nil {
		return nil, err
	}
	if err := sqlDB.Ping(); err != nil {
		return nil, err
	}
	logConnection(cfg, dsn)
	return &DB{Client: client, SQL: sqlDB, DSN: dsn}, nil
}

func logConnection(cfg Config, dsn string) {
	log.Printf("\n==============================")
	log.Printf("🚀 Postgres Connected Successfully!")
	log.Printf("Host: %s | Port: %s | DB: %s | User: %s", cfg.Host, cfg.Port, cfg.Name, cfg.Username)
	log.Printf("DSN: %s", dsn)
	log.Printf("==============================\n")
	log.Printf("[Postgres] Connection established. Ready for queries! 🚀")
}
