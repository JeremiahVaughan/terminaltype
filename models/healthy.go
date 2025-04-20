package models

import (
    "net/http"

    "github.com/JeremiahVaughan/terminaltype/clients/healthy"
    "github.com/JeremiahVaughan/terminaltype/clients"
)

type HealthyModel struct {
    healthy *healthy.Client
}

func NewHealthyModel(clients *clients.Clients) *HealthyModel {
    return &HealthyModel{
        healthy: clients.Healthy,
    }
}

func (m *HealthyModel) ReportUnexpectedError(w http.ResponseWriter, err error) {
    m.healthy.ReportUnexpectedError(w, err)
}

