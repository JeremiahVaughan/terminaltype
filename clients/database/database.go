package database

import (
    "fmt"
    "os"
	"database/sql"

    "github.com/JeremiahVaughan/terminaltype/config"
	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
)

type Client struct {
    Conn *sql.DB
    migrationDir string
}

func New(config config.Database) (*Client, error) {
    var err error
    _, err = os.Stat(config.DataDirectory)
    if os.IsNotExist(err) {
        err = os.MkdirAll(config.DataDirectory, 0700)
        if err != nil {
            return nil, fmt.Errorf("error, when creating database data directory. Error: %v", err)
        }
    }
    c := Client{
        migrationDir: config.MigrationDirectory,
    }
    dbFile := fmt.Sprintf("%s/data", config.DataDirectory)
    c.Conn, err = sql.Open("sqlite3", dbFile)
    if err != nil {
        return nil, fmt.Errorf("error, when entablishing database connection. Error: %v", err)
    }
    err = c.migrate()
    if err != nil {
        return nil, fmt.Errorf("error, when migrating database files. Error: %v", err)
    }
    return &c, nil
}
