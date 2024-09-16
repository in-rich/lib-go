package deploy

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"log"
	"net"
)

func OpenGRPCConn(host string) *grpc.ClientConn {
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

	conn, err := grpc.NewClient(host, opts...)
	if err != nil {
		log.Fatal(err, "failed to connect to service")
	}

	return conn
}

func CloseGRPCConn(conn *grpc.ClientConn) {
	if err := conn.Close(); err != nil {
		log.Fatal(err, "failed to close connection")
	}
}

func StartGRPCServer(port int) (net.Listener, *grpc.Server) {
	if port == 0 {
		log.Fatal("port is required")
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	return listener, grpc.NewServer()
}

func CloseGRPCServer(listener net.Listener, server *grpc.Server) {
	server.GracefulStop()
	_ = listener.Close()
}
