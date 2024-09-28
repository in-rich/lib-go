package monitor

import (
	"context"
	"fmt"
	"github.com/getsentry/sentry-go"
	sentrygin "github.com/getsentry/sentry-go/gin"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"strings"
	"time"
)

type gcpLogger struct {
	logger    zerolog.Logger
	projectID string
}

func (l *gcpLogger) Fatal(err error, msg string) {
	l.logger.Fatal().Err(err).Msg(msg)
}

func (l *gcpLogger) Error(err error, msg string) {
	l.logger.Error().Err(err).Msg(msg)
}

func (l *gcpLogger) Warn(msg string) {
	l.logger.Warn().Msg(msg)
}

func (l *gcpLogger) Info(msg string) {
	l.logger.Info().Msg(msg)
}

func (l *gcpLogger) Write(p []byte) (n int, err error) {
	l.logger.Info().Msg(string(p))
	return len(p), nil
}

func newGCPLogger(logger zerolog.Logger, projectID string) *gcpLogger {
	return &gcpLogger{
		logger:    logger,
		projectID: projectID,
	}
}

func NewGCPLogger(logger zerolog.Logger, projectID string) Logger {
	return newGCPLogger(logger, projectID)
}

type gcpGinLogger struct {
	gcpLogger
}

func (l *gcpGinLogger) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		end := time.Now()

		logLevel := zerolog.TraceLevel
		severity := "INFO" // For GCP.

		if c.Writer.Status() > 499 {
			logLevel = zerolog.ErrorLevel
			severity = "ERROR"
		} else if c.Writer.Status() > 399 || len(c.Errors) > 0 {
			logLevel = zerolog.WarnLevel
			severity = "WARNING"
		}

		parsedQuery := zerolog.Dict()
		for k, v := range c.Request.URL.Query() {
			parsedQuery.Strs(k, v)
		}

		// Allow logs to be grouped in log explorer.
		// https://cloud.google.com/run/docs/logging#run_manual_logging-go
		var trace string
		if l.projectID != "" {
			traceHeader := c.GetHeader("X-Cloud-Trace-Context")
			traceParts := strings.Split(traceHeader, "/")
			if len(traceParts) > 0 && len(traceParts[0]) > 0 {
				trace = fmt.Sprintf("projects/%s/traces/%s", l.projectID, traceParts[0])
			}
		}

		ll := l.logger.WithLevel(logLevel).
			Dict(
				"httpRequest", zerolog.Dict().
					Str("requestMethod", c.Request.Method).
					Str("requestUrl", c.FullPath()).
					Int("status", c.Writer.Status()).
					Str("userAgent", c.Request.UserAgent()).
					Str("remoteIp", c.ClientIP()).
					Str("protocol", c.Request.Proto).
					Str("latency", end.Sub(start).String()),
			).
			Time("start", start).
			Str("ip", c.ClientIP()).
			Str("contentType", c.ContentType()).
			Strs("errors", c.Errors.Errors()).
			Dict("query", parsedQuery).
			Str("severity", severity)

		if len(trace) > 0 {
			ll = ll.Str("logging.googleapis.com/trace", trace)
		}

		ll.Msg(c.Request.URL.String())

		hub := sentrygin.GetHubFromContext(c)
		if hub != nil {
			hub.Scope().SetRequest(c.Request)
			for _, err := range c.Errors {
				hub.CaptureException(err)
			}
		}
	}
}

func NewGCPGinLogger(logger zerolog.Logger, projectID string) GinLogger {
	return &gcpGinLogger{
		gcpLogger: *newGCPLogger(logger, projectID),
	}
}

type gcpGRPCLogger struct {
	gcpLogger
}

func (l *gcpGRPCLogger) Report(ctx context.Context, service string, err error) {
	logLevel := zerolog.TraceLevel
	severity := "INFO" // For GCP.
	code := codes.OK

	if err != nil {
		logLevel = zerolog.ErrorLevel
		severity = "ERROR"
		code = status.Code(err)
	}

	ll := l.logger.WithLevel(logLevel).
		Dict(
			"grpcRequest", zerolog.Dict().
				Str("service", service).
				Uint32("code", uint32(code)),
		).
		Err(err).
		Str("severity", severity)

	ll.Msg(fmt.Sprintf("GRPC %s [status %s]", service, code))

	hub := sentry.GetHubFromContext(ctx)
	if hub != nil && err != nil {
		hub.Scope()
		hub.CaptureException(err)
	}
}

func NewGCPGRPCLogger(logger zerolog.Logger, projectID string) GRPCLogger {
	return &gcpGRPCLogger{
		gcpLogger: *newGCPLogger(logger, projectID),
	}
}
