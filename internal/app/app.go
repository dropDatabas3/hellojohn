package app

import (
	"github.com/dropDatabas3/hellojohn/internal/jwt"
	"github.com/dropDatabas3/hellojohn/internal/store/core"
)

type Container struct {
	Store  core.Repository
	Issuer *jwt.Issuer
}
