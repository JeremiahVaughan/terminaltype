package main

import (
    "fmt"

    "github.com/JeremiahVaughan/terminaltype/clients/healthy"
)

const TestKey = "test"

func healthyRefresh(existingStatus healthy.HealthStatus) error {
    switch existingStatus.StatusKey {
    case TestKey:
        testHealthStatus()
    default:
        return fmt.Errorf("error, unknown status key: %s", existingStatus.StatusKey)
    }
    return nil
}


func testHealthStatus() {
    newStatus := healthy.HealthStatus{
        Service: serviceName,
        StatusKey: TestKey,
        Unhealthy: false,
        UnhealthyDelayInSeconds: 0,
        Message: "everythang looks ok",
    }
    theClients.Healthy.PublishHealthStatus(newStatus) 
}
