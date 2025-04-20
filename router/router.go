package router

import (
    "net/http"
    "fmt"
    "context"

    "github.com/JeremiahVaughan/terminaltype/controllers"
    "github.com/JeremiahVaughan/terminaltype/config"
    "github.com/JeremiahVaughan/terminaltype/ui_util"

    "github.com/nats-io/nats.go"
)

type Router struct {
    mux *http.ServeMux
}

type Topic struct {
    subject string
    handler func(context.Context)
}

type Subscription struct {
    subject string
    handler nats.MsgHandler
    sub *nats.Subscription
}

func New(
    controllers *controllers.Controllers,
    config config.Config,
) *Router {
    mux := http.NewServeMux()
    mux.HandleFunc("/", controllers.DashBoard.Handle)
    if config.LocalMode {
        mux.HandleFunc("/hotreload", controllers.TemplateLoader.HandleHotReload)
    }
    mux.HandleFunc("/health", controllers.Health.Check)
    ui_util.InitStaticFiles(mux, config.UiPath + "/static")
    return &Router{ 
        mux: mux,
    }
}

func (r *Router) Run(ctx context.Context, port int) error {
    url := fmt.Sprintf(":%d", port)
    err := http.ListenAndServe(url, r.mux)
    if err != nil {
        return fmt.Errorf("error, when serving http. Error: %v", err)
    }
    return nil
}

