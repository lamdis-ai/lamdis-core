// pkg/logger/logger.go
package logger

import (
	"go.uber.org/zap"
)

type Sugared = *zap.SugaredLogger

func New(env string) Sugared {
	var z *zap.Logger
	if env == "prod" {
		z, _ = zap.NewProduction()
	} else {
		z, _ = zap.NewDevelopment()
	}
	return z.Sugar()
}