package models

import (
    "github.com/JeremiahVaughan/terminaltype/clients"
)

type Models struct {
    Healthy *HealthyModel
}

func New(clients *clients.Clients) *Models {
    return &Models{
        Healthy: NewHealthyModel(clients),
    }
}
