package healthy

import (
    "fmt"
    "log"
    "net/http"
    "encoding/json"

    "github.com/JeremiahVaughan/terminaltype/config"
    "github.com/nats-io/nats.go"
)

// healthy requires nats
type Client struct {
    Conn *nats.Conn
    Sub *nats.Subscription
    serviceName string
    updateRefreshStatus func(status HealthStatus) error
}

// one health status for each of your statuses
type HealthStatus struct {
    // Service name of your service
    Service string `json:"service"`
    // StatusKey name of your status, must be unique within the service
    StatusKey string `json:"statusKey"`
    // Unhealthy report true if an undesirable condition has been met for this status
    Healthy bool `json:"healthy"`
    // UnhealthyDelayInSeconds this many seconds will pass with an unhealthy status of true before status cake is triggered
    UnhealthyDelayInSeconds int64 `json:"unhealthyDelayInSeconds"` 
    // Message the context of what the issue is
    Message string `json:"message"`
}

func New(
    config config.Nats,
    serviceName string,
    updateRefreshStatus func(status HealthStatus) error,
) (*Client, error) {
    url := fmt.Sprintf("%s:%d", config.Host, config.Port)
    var err error
    var result Client
    result.serviceName = serviceName
    result.updateRefreshStatus = updateRefreshStatus
    result.Conn, err = nats.Connect(url)
    if err != nil {
        return nil, fmt.Errorf("error, when connecting to nats service for client init. Error: %v", err)
    }
    key := getRefreshStatusKey(serviceName)
    result.Sub, err = result.Conn.Subscribe(key, result.handle)
    if err != nil {
        return nil, fmt.Errorf("error, when subscribing to refresh-all-health-statuses. Error: %v", err)
    }
    return &result, nil
}

// Close call during program shutdown
func (c *Client) Close() {
    err := c.Sub.Unsubscribe()
    if err != nil {
        err = fmt.Errorf("error, when unsubscribing from healthy nats. Error: %v", err)
        c.ReportUnexpectedError(nil, err)
        return
    }
    c.Conn.Close()
}

func (c *Client) PublishHealthStatus(status HealthStatus) {
    err := c.publishHealthStatus("update-health-status", status)    
    if err != nil {
        err = fmt.Errorf("error, when publishHealthStatus() for PublishHealthStatus(). Error: %v", err)
        c.ReportUnexpectedError(nil, err)
        return
    }
}

func (c *Client) ReportUnexpectedError(w http.ResponseWriter, err error) {
    log.Println(err.Error())
	if w != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
    status := HealthStatus{
        Service: c.serviceName,
        Message: err.Error(),
    }
    err = c.publishHealthStatus("report-unexpected-error", status)    
    if err != nil {
        // logging fatal so ha-proxy will get an unhealthy check
        log.Fatalf("error, unable to report unexpected error. Error: %v", err)
    }
}

func (c *Client) publishHealthStatus(subject string, status HealthStatus) error {
    bytes, err := json.Marshal(status)
    if err != nil {
        return fmt.Errorf("error, when encoding health status for key: %s. Error: %v", status.StatusKey, err)
    }
    err = c.Conn.Publish(subject, bytes)    
    if err != nil {
        return fmt.Errorf("error, when updating health status for key: %s. Error: %v", status.StatusKey, err)
    }
    return nil
}

func getRefreshStatusKey(serviceName string) string {
    return fmt.Sprintf("refresh-all-health-statuses:%s", serviceName) 
}


func (c *Client) handle(msg *nats.Msg) {
    var s HealthStatus
    err := json.Unmarshal(msg.Data, &s)
    if err != nil {
        err = fmt.Errorf("error, when decoding status from healthy. Error: %v", err)
        c.ReportUnexpectedError(nil, err)
        return
    }
    err = c.updateRefreshStatus(s)
    if err != nil {
        err = fmt.Errorf("error, when handling refresh status for status: %s. Error: %v", s.StatusKey, err)
        c.ReportUnexpectedError(nil, err)
        return
    }
}
