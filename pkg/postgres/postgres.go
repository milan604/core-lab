package postgres

import (
	"database/sql"
	"log"
	"net/url"
	"strings"

	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Config struct {
	Host     string
	Port     string
	Name     string
	Username string
	Password string
	SSLMode  string
}

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

// maskDSN redacts password from a postgres DSN when logging.
func maskDSN(dsn string) string {
	if dsn == "" {
		return dsn
	}
	if u, err := url.Parse(dsn); err == nil {
		if u.User != nil {
			user := u.User.Username()
			if _, has := u.User.Password(); has {
				u.User = url.UserPassword(user, "********")
			} else {
				u.User = url.User(user)
			}
		}
		return u.String()
	}
	// Fallback naive masking: redact between first ':' after scheme and '@'
	// Example: postgres://user:<pwd>@host => postgres://user:********@host
	i := strings.Index(dsn, "://")
	if i == -1 {
		return dsn
	}
	rest := dsn[i+3:]
	at := strings.Index(rest, "@")
	if at == -1 {
		return dsn
	}
	cred := rest[:at]
	colon := strings.Index(cred, ":")
	if colon == -1 {
		return dsn
	}
	maskedCred := cred[:colon+1] + "********"
	return dsn[:i+3] + maskedCred + rest[at:]
}

func logConnection(cfg Config, dsn string) {
	log.Printf("\n==============================")
	log.Printf("🚀 Postgres Connected Successfully!")
	log.Printf("Host: %s | Port: %s | DB: %s | User: %s", cfg.Host, cfg.Port, cfg.Name, cfg.Username)
	log.Printf("DSN: %s", maskDSN(dsn))
	log.Printf("==============================\n")
	log.Printf("[Postgres] Connection established. Ready for queries! 🚀")
}
