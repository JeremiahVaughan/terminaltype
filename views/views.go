package views

import (
    "fmt"

    "github.com/JeremiahVaughan/terminaltype/ui_util"
    "github.com/JeremiahVaughan/terminaltype/config"
)

type Views struct {
    TemplateLoader *ui_util.TemplateLoader
    DashBoard *DashBoardView
}

func New(config config.Config) (*Views, error) { 
    templates := []ui_util.HtmlTemplate{
        {
            Name: "dash-board",
            FileOverrides: []string{
                "dash_board.html",
            },
        },
    }
    tl, err := ui_util.NewTemplateLoader(
        config.UiPath + "/templates/base",
        config.UiPath + "/templates/overrides",
        templates,
        config.LocalMode,
    )
    if err != nil {
        return nil, fmt.Errorf("error, when creating template loader. Error: %v", err)
    }
    return &Views{
        DashBoard: NewDashBoardView(tl, config.LocalMode),
        TemplateLoader: tl,
    }, nil
}

