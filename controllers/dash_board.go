package controllers

import (
    "net/http"
    "fmt"

    "github.com/JeremiahVaughan/terminaltype/views"
    "github.com/JeremiahVaughan/terminaltype/models"
)

type DashBoardController struct {
    view *views.DashBoardView
    healthy *models.HealthyModel
}

func NewDashBoardController(views *views.Views, models *models.Models) *DashBoardController {
    return &DashBoardController{
        view: views.DashBoard,
        healthy: models.Healthy,
    }
}

func (c *DashBoardController) Handle(w http.ResponseWriter, r *http.Request) {
    err := c.view.Render(w) 
    if err != nil {
        err = fmt.Errorf("error, when handling dashboard request. Error: %v", err)
        c.healthy.ReportUnexpectedError(w, err)
        return
    }
}
