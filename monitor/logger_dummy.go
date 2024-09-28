package monitor

import (
	"context"
	"github.com/gin-gonic/gin"
)

type dummyLogger struct{}

func (d *dummyLogger) Fatal(_ error, _ string) {

}

func (d *dummyLogger) Error(_ error, _ string) {

}

func (d *dummyLogger) Warn(_ string) {

}

func (d *dummyLogger) Info(_ string) {

}

func (d *dummyLogger) Write(_ []byte) (int, error) {
	return 0, nil
}

func (d *dummyLogger) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}

func (d *dummyLogger) Report(_ context.Context, _ string, _ error) {

}

func NewDummyLogger() Logger {
	return &dummyLogger{}
}

func NewDummyGinLogger() GinLogger {
	return &dummyLogger{}
}

func NewDummyGRPCLogger() GRPCLogger {
	return &dummyLogger{}
}
