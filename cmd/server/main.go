package main

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	authv3 "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	"github.com/yuval/extauth-match/internal/auth"
	"github.com/yuval/extauth-match/internal/crypto"
	applog "github.com/yuval/extauth-match/internal/log"
	"github.com/yuval/extauth-match/internal/qrcode"
	"github.com/yuval/extauth-match/internal/relay"
	"google.golang.org/grpc"
)

func main() {
	applog.SetupLogging()
	// Generate encryption key and tenant ID
	encryptionKey, err := crypto.GenerateKey()
	if err != nil {
		slog.Error("Failed to generate encryption key", "error", err)
		panic(err)
	}

	tenantID := crypto.DeriveTenantID(encryptionKey)
	encodedKey := crypto.EncodeKey(encryptionKey)

	// Get relay URL from environment or use default
	relayURL := os.Getenv("RELAY_URL")
	if relayURL == "" {
		relayURL = "ws://localhost:9090"
	}

	// Create relay client
	relayClient, err := relay.NewClient(relayURL, tenantID, encryptionKey)
	if err != nil {
		slog.Error("Failed to create relay client", "error", err)
		os.Exit(1)
	}

	// Connect to relay
	if err := relayClient.Connect(); err != nil {
		slog.Error("Failed to connect to relay", "error", err)
		os.Exit(1)
	}

	// Create auth service with relay client
	authService := auth.NewService(relayClient)

	// Set decision handler
	relayClient.SetDecisionHandler(authService.HandleDecision)

	// Get browser base URL from environment or use default
	browserBaseURL := os.Getenv("BROWSER_BASE_URL")
	if browserBaseURL == "" {
		browserBaseURL = "http://localhost:9090"
	}

	// Generate and display QR code
	browserURL := fmt.Sprintf("%s/s/%s#key=%s", browserBaseURL, tenantID, encodedKey)
	fmt.Println("QR code", "ascii", qrcode.Generate(browserURL))
	slog.Info("Tenant ID", "tenantID", tenantID)
	slog.Info("Browser URL", "url", browserURL)

	// Start gRPC server for ext_authz
	grpcServer := grpc.NewServer()
	authv3.RegisterAuthorizationServer(grpcServer, authService)

	grpcLis, err := net.Listen("tcp", ":9000")
	if err != nil {
		slog.Error("Failed to listen on :9000", "error", err)
		os.Exit(1)
	}

	go func() {
		slog.Info("gRPC ext_authz server listening", "address", ":9000")
		if err := grpcServer.Serve(grpcLis); err != nil {
			slog.Error("Failed to serve gRPC", "error", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	slog.Info("Shutting down...")

	grpcServer.GracefulStop()
	relayClient.Close()
	slog.Info("Shutdown complete")
}

//ss
