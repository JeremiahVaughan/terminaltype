
package controllers

import (
    "net/http"

)

type HealthController struct {
}

func NewHealthController() *HealthController { 
    return &HealthController{}
}

func (i *HealthController) Check(w http.ResponseWriter, r *http.Request) {}
