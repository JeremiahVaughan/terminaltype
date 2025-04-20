package clients

import (
    "fmt"

    "github.com/JeremiahVaughan/terminaltype/clients/database"
    "github.com/JeremiahVaughan/terminaltype/clients/healthy"
    "github.com/JeremiahVaughan/terminaltype/config"
)

type Clients struct {
    Database *database.Client
    Healthy *healthy.Client
}

func New(config config.Config, serviceName string, healthyRefresh func(status healthy.HealthStatus) error) (*Clients, error) {
    db, err := database.New(config.Database)
    if err != nil {
        return nil, fmt.Errorf("error, when creating database client. Error: %v", err)
    }
    healthy, err := healthy.New(config.Nats, serviceName, healthyRefresh)
    if err != nil {
        return nil, fmt.Errorf("error, when creating new healthy client. Error: %v", err)
    }
    return &Clients{
        Database: db,
        Healthy: healthy,
    }, nil
}
