package controllers

import (
    "github.com/JeremiahVaughan/terminaltype/views"
    "github.com/JeremiahVaughan/terminaltype/models"
    "github.com/JeremiahVaughan/terminaltype/ui_util"
)

type Controllers struct {
    DashBoard *DashBoardController
    TemplateLoader *ui_util.TemplateLoader
    Health *HealthController
}

func New(views *views.Views, models *models.Models) *Controllers {
    return &Controllers{
        DashBoard: NewDashBoardController(views, models),
        Health: NewHealthController(),
        TemplateLoader: views.TemplateLoader,
    }
}
