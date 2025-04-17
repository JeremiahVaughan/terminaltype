package clients

import (
    "fmt"

    "github.com/JeremiahVaughan/terminaltype/clients/database"
    "github.com/JeremiahVaughan/terminaltype/config"
)

type Clients struct {
    Database *database.Client
}

func New(config config.Config) (*Clients, error) {
    db, err := database.New(config.Database)
    if err != nil {
        return nil, fmt.Errorf("error, when creating database client. Error: %v", err)
    }
    return &Clients{
        Database: db,
    }, nil
}
