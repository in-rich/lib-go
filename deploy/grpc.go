package deploy

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	_ "embed"
	"fmt"
	"golang.org/x/oauth2"
	"google.golang.org/api/idtoken"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	grpcMetadata "google.golang.org/grpc/metadata"
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
func OpenGRPCConn(host string) (*grpc.ClientConn, oauth2.TokenSource) {
	var opts []grpc.DialOption

	if IsReleaseEnv() {
		systemRoots, err := x509.SystemCertPool()
		if err != nil {
			log.Fatal(err, "failed to load system root CA certificates")
		}

		cred := credentials.NewTLS(&tls.Config{RootCAs: systemRoots})

		opts = append(opts, grpc.WithAuthority(host), grpc.WithTransportCredentials(cred))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	opts = append(opts, grpc.WithDefaultServiceConfig(grpcConfig))
	conn, err := grpc.NewClient(host, opts...)
	if err != nil {
		log.Fatal(err, "failed to connect to service")
	}

	// There is no authentication required for local environments, so the token source is nil.
	if !IsReleaseEnv() {
		return conn, nil
	}

	tokenSource, err := idtoken.NewTokenSource(context.Background(), host)
	if err != nil {
		log.Fatal(err, "failed to create token source")
	}

	return conn, tokenSource
}

// CloseGRPCConn closes an existing connection to a GRPC service.
func CloseGRPCConn(conn *grpc.ClientConn) {
	if err := conn.Close(); err != nil {
		log.Fatal(err, "failed to close connection")
	}
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
func StartGRPCServer(port int, depsCheck func() map[string]bool) (net.Listener, *grpc.Server, func()) {
	if port == 0 {
		log.Fatal("port is required")
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	server := grpc.NewServer()

	// Set healthcheck.
	// https://github.com/grpc/grpc-go/blob/master/examples/features/health/server/main.go
	healthcheck := health.NewServer()
	healthgrpc.RegisterHealthServer(server, healthcheck)

	healthUpdater := func() {
		status := depsCheck()
		for service, ok := range status {
			if ok {
				healthcheck.SetServingStatus(service, healthpb.HealthCheckResponse_SERVING)
			} else {
				healthcheck.SetServingStatus(service, healthpb.HealthCheckResponse_NOT_SERVING)
			}
		}

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
	ctx context.Context, callback GRPCCallback[In, Out], in *In, tokenSource oauth2.TokenSource,
) (*Out, error) {
	// Prevent the call from tasking too long.
	localCTX, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// tokenSource is not defined under local environments.
	if tokenSource != nil {
		// Retrieve an authentication token for our request.
		token, err := tokenSource.Token()
		if err != nil {
			return nil, err
		}

		// Append credentials to context. Those will be automatically used by the GRPC client.
		localCTX = grpcMetadata.AppendToOutgoingContext(localCTX, "authorization", "Bearer "+token.AccessToken)
	}

	// Call the GRPC endpoint.
	res, err := callback(localCTX, in)
	if err != nil {
		return nil, err
	}

	return res, nil
}
