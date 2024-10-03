package deploy

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	_ "embed"
	"fmt"
	"github.com/in-rich/lib-go/monitor"
	"github.com/samber/lo"
	"google.golang.org/api/idtoken"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/credentials/oauth"
	"google.golang.org/grpc/health"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"log"
	"net"
	"time"
)

//go:embed grpc-config.json
var grpcConfig string

// GRPCCallback represents the generic signature of exposed RPC services, as generated by the protoc compiler for Go.
type GRPCCallback[In any, Out any] func(ctx context.Context, in *In, opts ...grpc.CallOption) (*Out, error)

// OpenGRPCConn opens a new connection to an existing GRPC service. This method also automatically provides an
// oauth2.TokenSource under release environments, to communicate with the service. The token source is nil under
// local environments.
//
// You must ensure to properly close the connection when you are done, using the CloseGRPCConn method.
//
//	conn, tokenSource := deploy.OpenGRPCConn("localhost:50051")
//	defer deploy.CloseGRPCConn(conn)
//
// This method automatically retrieves credentials under release environments.
func OpenGRPCConn(logger monitor.Logger, host string) *grpc.ClientConn {
	var opts []grpc.DialOption

	if IsReleaseEnv() {
		systemRoots, err := x509.SystemCertPool()
		if err != nil {
			logger.Fatal(err, "failed to load system root CA certificates")
		}

		tokenSource, err := idtoken.NewTokenSource(context.Background(), "https://"+host)
		if err != nil {
			logger.Fatal(err, "failed to create token source")
		}

		cred := credentials.NewTLS(&tls.Config{RootCAs: systemRoots})

		opts = append(
			opts,
			grpc.WithTransportCredentials(cred),
			grpc.WithAuthority(host+":443"),
			grpc.WithPerRPCCredentials(oauth.TokenSource{TokenSource: tokenSource}),
		)
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// TODO: uncomment to enable automatic healthcheck.
	//opts = append(opts, grpc.WithDefaultServiceConfig(grpcConfig))
	conn, err := grpc.NewClient(host, opts...)
	if err != nil {
		logger.Fatal(err, "failed to connect to service")
	}

	return conn
}

// CloseGRPCConn closes an existing connection to a GRPC service.
func CloseGRPCConn(conn *grpc.ClientConn) {
	if err := conn.Close(); err != nil {
		log.Fatal(err, "failed to close connection")
	}
}

type DepCheckCallback func() map[string]error

type DepCheckServices map[string][]string

type DepsCheck struct {
	Dependencies DepCheckCallback
	Services     DepCheckServices
}

// StartGRPCServer starts a new GRPC server on the specified port.
//
// You must ensure to properly close the server when you are done, using the CloseGRPCServer method.
//
//	listener, server, health := deploy.StartGRPCServer(50051)
//	// Graceful shutdown.
//	defer deploy.CloseGRPCServer(listener, server)
//	// Start healthcheck.
//	go health()
func StartGRPCServer(logger monitor.Logger, port int, depsCheck DepsCheck) (net.Listener, *grpc.Server, func()) {
	if port == 0 {
		log.Fatal("port is required")
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		logger.Fatal(err, "failed to listen")
	}

	server := grpc.NewServer()

	// Set healthcheck.
	// https://github.com/grpc/grpc-go/blob/master/examples/features/health/server/main.go
	healthcheck := health.NewServer()
	healthgrpc.RegisterHealthServer(server, healthcheck)

	healthUpdater := func() {
		dependencies := depsCheck.Dependencies()
		global := true

		for dependency, err := range dependencies {
			if err != nil {
				logger.Fatal(err, fmt.Sprintf("dependency check for %s failed", dependency))
				global = false
			}
		}

		for service, serviceDeps := range depsCheck.Services {
			_, hasError := lo.Find(serviceDeps, func(item string) bool {
				return dependencies[item] != nil
			})

			healthcheck.SetServingStatus(
				service,
				lo.Ternary(hasError, healthpb.HealthCheckResponse_NOT_SERVING, healthpb.HealthCheckResponse_SERVING),
			)
		}

		healthcheck.SetServingStatus(
			"",
			lo.Ternary(global, healthpb.HealthCheckResponse_SERVING, healthpb.HealthCheckResponse_NOT_SERVING),
		)

		time.Sleep(5 * time.Second)
	}

	return listener, server, healthUpdater
}

// CloseGRPCServer closes an existing GRPC server.
func CloseGRPCServer(listener net.Listener, server *grpc.Server) {
	server.GracefulStop()
	_ = listener.Close()
}

// CallGRPCEndpoint performs a call to a GRPC endpoint, located in a secure cloud environment.
func CallGRPCEndpoint[In any, Out any](
	ctx context.Context, callback GRPCCallback[In, Out], in *In,
) (*Out, error) {
	// Prevent the call from tasking too long.
	localCTX, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// Call the GRPC endpoint.
	res, err := callback(localCTX, in)
	if err != nil {
		return nil, err
	}

	return res, nil
}
