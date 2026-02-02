package auth

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	authv3 "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
)

// RelayClient interface for dependency injection
type RelayClient interface {
	SendRequest(data interface{}) error
}

type Service struct {
	authv3.UnimplementedAuthorizationServer
	relayClient  RelayClient
	pendingReqs  map[string]*PendingRequest
	mu           sync.RWMutex
	requestQueue chan *PendingRequest
}

type PendingRequest struct {
	ID        string
	Method    string
	Path      string
	Headers   map[string]string
	SourceIP  string
	Timestamp time.Time
	Response  chan bool
	ctx       context.Context
}

func NewService(relayClient RelayClient) *Service {
	s := &Service{
		relayClient:  relayClient,
		pendingReqs:  make(map[string]*PendingRequest),
		requestQueue: make(chan *PendingRequest, 100),
	}
	go s.processQueue()
	return s
}

func (s *Service) Check(ctx context.Context, req *authv3.CheckRequest) (*authv3.CheckResponse, error) {
	// Extract request attributes
	attrs := req.GetAttributes()
	if attrs == nil {
		return s.denyResponse("No attributes"), nil
	}

	httpReq := attrs.GetRequest().GetHttp()
	if httpReq == nil {
		return s.denyResponse("No HTTP request"), nil
	}

	// Create pending request
	reqID := fmt.Sprintf("%d", time.Now().UnixNano())
	pendingReq := &PendingRequest{
		ID:        reqID,
		Method:    httpReq.GetMethod(),
		Path:      httpReq.GetPath(),
		Headers:   httpReq.GetHeaders(),
		SourceIP:  attrs.GetSource().GetAddress().GetSocketAddress().GetAddress(),
		Timestamp: time.Now(),
		Response:  make(chan bool, 1),
		ctx:       ctx,
	}

	// Store pending request
	s.mu.Lock()
	s.pendingReqs[reqID] = pendingReq
	s.mu.Unlock()

	// Add to queue for processing
	s.requestQueue <- pendingReq

	// Wait for response with 30s timeout
	timeout := time.NewTimer(30 * time.Second)
	defer timeout.Stop()

	select {
	case approved := <-pendingReq.Response:
		s.cleanup(reqID)
		if approved {
			slog.Info("Request approved", "requestID", reqID, "method", pendingReq.Method, "path", pendingReq.Path)
			return s.okResponse(), nil
		}
		slog.Info("Request denied", "requestID", reqID, "method", pendingReq.Method, "path", pendingReq.Path)
		return s.denyResponse("Access denied by user"), nil
	case <-timeout.C:
		s.cleanup(reqID)
		slog.Info("Request timed out", "requestID", reqID, "method", pendingReq.Method, "path", pendingReq.Path)
		return s.denyResponse("Authorization timeout"), nil
	case <-ctx.Done():
		s.cleanup(reqID)
		slog.Info("Request cancelled", "requestID", reqID, "method", pendingReq.Method, "path", pendingReq.Path)
		return s.denyResponse("Request cancelled"), nil
	}
}

func (s *Service) processQueue() {
	for pendingReq := range s.requestQueue {
		// Send encrypted request via relay
		msg := map[string]interface{}{
			"id":        pendingReq.ID,
			"method":    pendingReq.Method,
			"path":      pendingReq.Path,
			"headers":   pendingReq.Headers,
			"sourceIP":  pendingReq.SourceIP,
			"timestamp": pendingReq.Timestamp.Format(time.RFC3339),
		}

		if err := s.relayClient.SendRequest(msg); err != nil {
			slog.Error("Failed to send request to relay", "error", err)
			pendingReq.Response <- false
			continue
		}

		slog.Info("Sent encrypted request to relay", "requestID", pendingReq.ID)
	}
}

func (s *Service) HandleDecision(reqID string, approved bool) {
	s.mu.RLock()
	pendingReq, exists := s.pendingReqs[reqID]
	s.mu.RUnlock()

	if !exists {
		slog.Info("Request not found or already processed", "requestID", reqID)
		return
	}

	select {
	case pendingReq.Response <- approved:
		slog.Info("Decision sent for request", "requestID", reqID, "approved", approved)
	default:
		slog.Error("Failed to send decision for request (channel full or closed)", "requestID", reqID)
	}
}

func (s *Service) cleanup(reqID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.pendingReqs, reqID)
}

func (s *Service) okResponse() *authv3.CheckResponse {
	return &authv3.CheckResponse{
		Status: &status.Status{Code: int32(codes.OK)},
		HttpResponse: &authv3.CheckResponse_OkResponse{
			OkResponse: &authv3.OkHttpResponse{
				Headers: []*corev3.HeaderValueOption{
					{
						Header: &corev3.HeaderValue{
							Key:   "x-authz-result",
							Value: "approved",
						},
					},
				},
			},
		},
	}
}

func (s *Service) denyResponse(reason string) *authv3.CheckResponse {
	return &authv3.CheckResponse{
		Status: &status.Status{Code: int32(codes.PermissionDenied)},
		HttpResponse: &authv3.CheckResponse_DeniedResponse{
			DeniedResponse: &authv3.DeniedHttpResponse{
				Status: &typev3.HttpStatus{Code: typev3.StatusCode_Forbidden},
				Headers: []*corev3.HeaderValueOption{
					{
						Header: &corev3.HeaderValue{
							Key:   "content-type",
							Value: "application/json",
						},
					},
				},
				Body: fmt.Sprintf(`{"error":"%s"}`, reason),
			},
		},
	}
}
