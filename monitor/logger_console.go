package monitor

import (
	"context"
	"fmt"
	"github.com/fatih/color"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"log"
	"strings"
	"time"
)

type consoleLogger struct{}

func (l *consoleLogger) Fatal(err error, msg string) {
	colorizer := color.New(color.FgMagenta).SprintFunc()

	if msg == "" {
		log.Fatal(colorizer(err.Error()))
	} else {
		log.Fatal(colorizer(fmt.Sprintf("%s: %s\n", msg, err.Error())))
	}
}

func (l *consoleLogger) Error(err error, msg string) {
	colorizer := color.New(color.FgRed).SprintFunc()

	if msg == "" {
		log.Printf(colorizer("%s\n", err.Error()))
	} else {
		log.Printf(colorizer("%s: %s\n", msg, err.Error()))
	}
}

func (l *consoleLogger) Warn(msg string) {
	colorizer := color.New(color.FgYellow).SprintFunc()
	log.Println(colorizer(msg))
}

func (l *consoleLogger) Info(msg string) {
	log.Println(msg)
}

func (l *consoleLogger) Write(p []byte) (n int, err error) {
	log.Println(string(p))
	return len(p), nil
}

func newConsoleLogger() *consoleLogger {
	return &consoleLogger{}
}

func NewConsoleLogger() Logger {
	return newConsoleLogger()
}

type consoleGinLogger struct {
	consoleLogger
}

func (l *consoleGinLogger) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		end := time.Now()

		colorizer := color.New(color.FgBlue).SprintFunc()
		prefix := "✓"
		if c.Writer.Status() > 499 {
			colorizer = color.New(color.FgRed).SprintFunc()
			prefix = "✗"
		} else if c.Writer.Status() > 399 || len(c.Errors) > 0 {
			colorizer = color.New(color.FgYellow).SprintFunc()
			prefix = "⟁"
		}

		message := strings.Join([]string{
			"-",
			colorizer(color.New(color.Bold).Sprintf("%s %v", prefix, c.Writer.Status())),
			colorizer(fmt.Sprintf("[%s %s]", c.Request.Method, c.FullPath())),
			color.New(color.Faint).Sprint(fmt.Sprintf("(processed in %s)", end.Sub(start))),
		}, " ")

		log.Println(message)
		for _, err := range c.Errors {
			l.Error(err, "")
		}
	}
}

func NewConsoleGinLogger() GinLogger {
	return &consoleGinLogger{
		consoleLogger: *newConsoleLogger(),
	}
}

type consoleGRPCLogger struct {
	consoleLogger
}

func (l *consoleGRPCLogger) Report(_ context.Context, service string, err error) {
	colorizer := color.New(color.FgBlue).SprintFunc()
	prefix := "✓"
	code := codes.OK

	if err != nil {
		code = status.Code(err)

		if code == codes.Unknown {
			colorizer = color.New(color.FgRed).SprintFunc()
			prefix = "✗"
		} else if code == codes.Unavailable {
			colorizer = color.New(color.FgYellow).SprintFunc()
			prefix = "⟁"
		}
	}

	message := strings.Join([]string{
		"-",
		colorizer(color.New(color.Bold).Sprintf("%s %s", prefix, code)),
		colorizer(fmt.Sprintf("[%s]", service)),
	}, " ")

	log.Println(message)

	if err != nil {
		l.Error(err, "")
	}
}

func NewConsoleGRPCLogger() GRPCLogger {
	return &consoleGRPCLogger{
		consoleLogger: *newConsoleLogger(),
	}
}
