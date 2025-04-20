package views

import (
    "net/http"
    "fmt"

    "github.com/JeremiahVaughan/terminaltype/ui_util"
)

type DashBoardView struct {
    tl *ui_util.TemplateLoader
    localMode bool
}

func NewDashBoardView(
    tl *ui_util.TemplateLoader,
    localMode bool,
) *DashBoardView {
    return &DashBoardView{
        tl: tl,
        localMode: localMode,
    }
}

type DashBoard struct {
    LocalMode bool
}

func (i *DashBoardView) Render(w http.ResponseWriter) error {
    d := DashBoard{
        LocalMode: i.localMode,
    }
    err := i.tl.GetTemplateGroup("dash-board").ExecuteTemplate(w, "base", d)
    if err != nil {
        return fmt.Errorf("error, when rendering template for DashBoardView.Render(). Error: %v", err)
    }
    return nil
}
