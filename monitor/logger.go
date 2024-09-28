package monitor

import (
	"context"
	"github.com/gin-gonic/gin"
	"io"
)

type Logger interface {
	Fatal(err error, msg string)
	Error(err error, msg string)
	Warn(msg string)
	Info(msg string)

	io.Writer
}

type GinLogger interface {
	Logger
	Middleware() gin.HandlerFunc
}

type GRPCLogger interface {
	Report(ctx context.Context, service string, err error)
}
