package controllers

import (
	"github.com/dropDatabas3/hellojohn/internal/http/v2/services"
)

type Controller interface {
}

type NewController func(svc *services.Services) Controller
