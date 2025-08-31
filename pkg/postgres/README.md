# Postgres Wrapper

This package provides a simple, flexible, and advanced wrapper for connecting to PostgreSQL using GORM in Go projects.

## Features
- Clean API: Just create a `Config` and call `postgres.New(cfg)`
- Returns both GORM and raw SQL clients
- Attractive connection logs
- Idiomatic Go design for testability and modularity

## Usage Example
```go
import "github.com/milan604/core-lab/pkg/postgres"

func main() {
    cfg := postgres.Config{
        Host:     "localhost",
        Port:     "5432",
        Name:     "mydb",
        Username: "user",
        Password: "pass",
        SSLMode:  "disable",
    }
    db, err := postgres.New(cfg)
    if err != nil {
        log.Fatalf("failed to connect: %v", err)
    }
    // Use db.Client for GORM, db.SQL for raw SQL
}
```

## API Reference
- `type Config`: Connection parameters
- `func New(cfg Config) (*DB, error)`: Connect and return DB struct
- `type DB`: Holds `Client` (*gorm.DB), `SQL` (*sql.DB), and `DSN` (string)

## Best Practices
- Pass the `DB` struct to your service/repository layer, not via Gin context
- Use environment variables or config files for credentials
- Handle errors and close connections gracefully

## Advanced
- Customize GORM options as needed
- Add migrations, health checks, or pooling in your own code

---
For issues or questions, open an issue in the repository.
