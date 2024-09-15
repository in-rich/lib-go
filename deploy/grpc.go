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
	"os"
	"strings"
)

func loadCertificates(prefix string) (tls.Certificate, error) {
	certEnv := fmt.Sprintf("%s_CERT", strings.ToUpper(prefix))
	keyEnv := fmt.Sprintf("%s_KEY", strings.ToUpper(prefix))

	rawCert := os.Getenv(certEnv)
	rawKey := os.Getenv(keyEnv)

	if rawCert == "" {
		return tls.Certificate{}, fmt.Errorf("missing certificate for service: %s\n", prefix)
	}
	if rawKey == "" {
		return tls.Certificate{}, fmt.Errorf("missing key for service: %s\n", prefix)
	}

	cert, err := tls.X509KeyPair([]byte(rawCert), []byte(rawKey))
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to load certificate for service %s: %w", prefix, err)
	}

	return cert, nil
}

func loadCA(prefix string) (*x509.CertPool, error) {
	caEnv := fmt.Sprintf("%s_CA", strings.ToUpper(prefix))

	rawCA := os.Getenv(caEnv)

	if rawCA == "" {
		return nil, fmt.Errorf("missing CA for service: %s\n", prefix)
	}

	ca := x509.NewCertPool()
	if ok := ca.AppendCertsFromPEM([]byte(rawCA)); !ok {
		return nil, fmt.Errorf("failed to load CA for service %s\n", prefix)
	}

	return ca, nil
}

func loadClientCredentials(host, name string) credentials.TransportCredentials {
	if !IsReleaseEnv() {
		return insecure.NewCredentials()
	}

	cert, err := loadCertificates(name)
	if err != nil {
		log.Fatal(err, "failed to load certificate for service "+name)
	}

	ca, err := loadCA(name)
	if err != nil {
		log.Fatal(err, "failed to load CA for service "+name)
	}

	tlsConfig := &tls.Config{
		ServerName:   host,
		Certificates: []tls.Certificate{cert},
		RootCAs:      ca,
	}

	return credentials.NewTLS(tlsConfig)
}

func loadServerCredentials(name string) credentials.TransportCredentials {
	if !IsReleaseEnv() {
		return insecure.NewCredentials()
	}

	cert, err := loadCertificates(name)
	if err != nil {
		log.Fatal(err, "failed to load certificate for service "+name)
	}

	ca, err := loadCA(name)
	if err != nil {
		log.Fatal(err, "failed to load CA for service "+name)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    ca,
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}

	return credentials.NewTLS(tlsConfig)
}

func OpenGRPCConn(host, name string) *grpc.ClientConn {
	conn, err := grpc.NewClient(
		host,
		grpc.WithTransportCredentials(loadClientCredentials(host, name)),
	)
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

func StartGRPCServer(port, name string) (net.Listener, *grpc.Server) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	server := grpc.NewServer(grpc.Creds(loadServerCredentials(name)))

	return listener, server
}

func CloseGRPCServer(listener net.Listener, server *grpc.Server) {
	server.GracefulStop()
	_ = listener.Close()
}
