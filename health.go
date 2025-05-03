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
        Healthy: true,
        UnhealthyDelayInSeconds: 20,
        Message: "its healthy",
    }
    theClients.Healthy.PublishHealthStatus(newStatus) 
}
