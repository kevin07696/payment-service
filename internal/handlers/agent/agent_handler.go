package agent

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	agentv1 "github.com/kevin07696/payment-service/proto/agent/v1"
	"github.com/kevin07696/payment-service/internal/domain"
	"github.com/kevin07696/payment-service/internal/services/ports"
	"go.uber.org/zap"
)

// Handler implements the gRPC AgentServiceServer
type Handler struct {
	agentv1.UnimplementedAgentServiceServer
	service ports.AgentService
	logger  *zap.Logger
}

// NewHandler creates a new agent handler
func NewHandler(service ports.AgentService, logger *zap.Logger) *Handler {
	return &Handler{
		service: service,
		logger:  logger,
	}
}

// RegisterAgent adds a new agent/merchant to the system
func (h *Handler) RegisterAgent(ctx context.Context, req *agentv1.RegisterAgentRequest) (*agentv1.AgentResponse, error) {
	h.logger.Info("RegisterAgent request received",
		zap.String("agent_id", req.AgentId),
		zap.String("environment", req.Environment.String()),
	)

	// Validate request
	if err := validateRegisterAgentRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// Convert to service request
	serviceReq := &ports.RegisterAgentRequest{
		AgentID:     req.AgentId,
		MACSecret:   req.MacSecret,
		CustNbr:     req.CustNbr,
		MerchNbr:    req.MerchNbr,
		DBAnbr:      req.DbaNbr,
		TerminalNbr: req.TerminalNbr,
		Environment: environmentFromProto(req.Environment),
		AgentName:   req.AgentId, // Default to agent_id if not provided
	}

	if req.IdempotencyKey != "" {
		serviceReq.IdempotencyKey = &req.IdempotencyKey
	}

	// Call service
	agent, err := h.service.RegisterAgent(ctx, serviceReq)
	if err != nil {
		return nil, handleServiceError(err)
	}

	// Convert to proto response
	return agentToResponse(agent), nil
}

// GetAgent retrieves agent credentials (internal use only)
func (h *Handler) GetAgent(ctx context.Context, req *agentv1.GetAgentRequest) (*agentv1.Agent, error) {
	if req.AgentId == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
	}

	agent, err := h.service.GetAgent(ctx, req.AgentId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, domain.ErrAgentNotFound) {
			return nil, status.Error(codes.NotFound, "agent not found")
		}
		return nil, status.Error(codes.Internal, "failed to get agent")
	}

	return agentToProto(agent), nil
}

// ListAgents lists all registered agents
func (h *Handler) ListAgents(ctx context.Context, req *agentv1.ListAgentsRequest) (*agentv1.ListAgentsResponse, error) {
	// Default pagination
	limit := int(req.Limit)
	if limit <= 0 {
		limit = 100
	}
	offset := int(req.Offset)

	var environment *domain.Environment
	if req.Environment != nil {
		env := environmentFromProto(*req.Environment)
		environment = &env
	}

	var isActive *bool
	if req.IsActive != nil {
		isActive = req.IsActive
	}

	agents, totalCount, err := h.service.ListAgents(ctx, environment, isActive, limit, offset)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to list agents")
	}

	protoAgents := make([]*agentv1.AgentSummary, len(agents))
	for i, agent := range agents {
		protoAgents[i] = agentToSummary(agent)
	}

	return &agentv1.ListAgentsResponse{
		Agents:     protoAgents,
		TotalCount: int32(totalCount),
	}, nil
}

// UpdateAgent updates agent credentials
func (h *Handler) UpdateAgent(ctx context.Context, req *agentv1.UpdateAgentRequest) (*agentv1.AgentResponse, error) {
	h.logger.Info("UpdateAgent request received",
		zap.String("agent_id", req.AgentId),
	)

	if req.AgentId == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
	}

	serviceReq := &ports.UpdateAgentRequest{
		AgentID: req.AgentId,
	}

	if req.MacSecret != nil {
		serviceReq.MACSecret = req.MacSecret
	}
	if req.CustNbr != nil {
		serviceReq.CustNbr = req.CustNbr
	}
	if req.MerchNbr != nil {
		serviceReq.MerchNbr = req.MerchNbr
	}
	if req.DbaNbr != nil {
		serviceReq.DBAnbr = req.DbaNbr
	}
	if req.TerminalNbr != nil {
		serviceReq.TerminalNbr = req.TerminalNbr
	}
	if req.Environment != nil {
		env := environmentFromProto(*req.Environment)
		serviceReq.Environment = &env
	}
	if req.IdempotencyKey != "" {
		serviceReq.IdempotencyKey = &req.IdempotencyKey
	}

	agent, err := h.service.UpdateAgent(ctx, serviceReq)
	if err != nil {
		return nil, handleServiceError(err)
	}

	return agentToResponse(agent), nil
}

// DeactivateAgent deactivates an agent
func (h *Handler) DeactivateAgent(ctx context.Context, req *agentv1.DeactivateAgentRequest) (*agentv1.AgentResponse, error) {
	h.logger.Info("DeactivateAgent request received",
		zap.String("agent_id", req.AgentId),
		zap.String("reason", req.Reason),
	)

	if req.AgentId == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
	}

	// Get agent before deactivation to return in response
	agent, err := h.service.GetAgent(ctx, req.AgentId)
	if err != nil {
		return nil, handleServiceError(err)
	}

	err = h.service.DeactivateAgent(ctx, req.AgentId, req.Reason)
	if err != nil {
		return nil, handleServiceError(err)
	}

	// Mark as inactive in response
	agent.IsActive = false

	return agentToResponse(agent), nil
}

// RotateMAC rotates MAC secret in secret manager
func (h *Handler) RotateMAC(ctx context.Context, req *agentv1.RotateMACRequest) (*agentv1.RotateMACResponse, error) {
	h.logger.Info("RotateMAC request received",
		zap.String("agent_id", req.AgentId),
	)

	if req.AgentId == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
	}
	if req.NewMacSecret == "" {
		return nil, status.Error(codes.InvalidArgument, "new_mac_secret is required")
	}

	serviceReq := &ports.RotateMACRequest{
		AgentID:      req.AgentId,
		NewMACSecret: req.NewMacSecret,
	}

	err := h.service.RotateMAC(ctx, serviceReq)
	if err != nil {
		return nil, handleServiceError(err)
	}

	// Get updated agent to return mac_secret_path
	agent, err := h.service.GetAgent(ctx, req.AgentId)
	if err != nil {
		return nil, handleServiceError(err)
	}

	return &agentv1.RotateMACResponse{
		AgentId:       agent.AgentID,
		MacSecretPath: agent.MACSecretPath,
		RotatedAt:     timestamppb.New(time.Now()),
	}, nil
}

// Validation helpers

func validateRegisterAgentRequest(req *agentv1.RegisterAgentRequest) error {
	if req.AgentId == "" {
		return fmt.Errorf("agent_id is required")
	}
	if req.MacSecret == "" {
		return fmt.Errorf("mac_secret is required")
	}
	if req.CustNbr == "" {
		return fmt.Errorf("cust_nbr is required")
	}
	if req.MerchNbr == "" {
		return fmt.Errorf("merch_nbr is required")
	}
	if req.DbaNbr == "" {
		return fmt.Errorf("dba_nbr is required")
	}
	if req.TerminalNbr == "" {
		return fmt.Errorf("terminal_nbr is required")
	}
	if req.Environment == agentv1.Environment_ENVIRONMENT_UNSPECIFIED {
		return fmt.Errorf("environment is required")
	}
	return nil
}

// Conversion helpers

func agentToResponse(agent *domain.Agent) *agentv1.AgentResponse {
	return &agentv1.AgentResponse{
		AgentId:       agent.AgentID,
		MacSecretPath: agent.MACSecretPath,
		CustNbr:       agent.CustNbr,
		MerchNbr:      agent.MerchNbr,
		DbaNbr:        agent.DBAnbr,
		TerminalNbr:   agent.TerminalNbr,
		Environment:   environmentToProto(agent.Environment),
		IsActive:      agent.IsActive,
		CreatedAt:     timestamppb.New(agent.CreatedAt),
		UpdatedAt:     timestamppb.New(agent.UpdatedAt),
	}
}

func agentToProto(agent *domain.Agent) *agentv1.Agent {
	return &agentv1.Agent{
		Id:            agent.ID,
		AgentId:       agent.AgentID,
		MacSecretPath: agent.MACSecretPath,
		CustNbr:       agent.CustNbr,
		MerchNbr:      agent.MerchNbr,
		DbaNbr:        agent.DBAnbr,
		TerminalNbr:   agent.TerminalNbr,
		Environment:   environmentToProto(agent.Environment),
		IsActive:      agent.IsActive,
		CreatedAt:     timestamppb.New(agent.CreatedAt),
		UpdatedAt:     timestamppb.New(agent.UpdatedAt),
		Metadata:      nil, // Not storing metadata yet
	}
}

func agentToSummary(agent *domain.Agent) *agentv1.AgentSummary {
	return &agentv1.AgentSummary{
		AgentId:     agent.AgentID,
		MerchNbr:    agent.MerchNbr,
		Environment: environmentToProto(agent.Environment),
		IsActive:    agent.IsActive,
		CreatedAt:   timestamppb.New(agent.CreatedAt),
	}
}

func environmentToProto(env domain.Environment) agentv1.Environment {
	switch env {
	case domain.EnvironmentSandbox:
		return agentv1.Environment_ENVIRONMENT_SANDBOX
	case domain.EnvironmentProduction:
		return agentv1.Environment_ENVIRONMENT_PRODUCTION
	default:
		return agentv1.Environment_ENVIRONMENT_UNSPECIFIED
	}
}

func environmentFromProto(env agentv1.Environment) domain.Environment {
	switch env {
	case agentv1.Environment_ENVIRONMENT_SANDBOX:
		return domain.EnvironmentSandbox
	case agentv1.Environment_ENVIRONMENT_PRODUCTION:
		return domain.EnvironmentProduction
	default:
		return domain.EnvironmentSandbox // Default
	}
}

// Error handling

func handleServiceError(err error) error {
	// Map domain errors to gRPC status codes
	switch {
	case errors.Is(err, domain.ErrAgentNotFound):
		return status.Error(codes.NotFound, "agent not found")
	case errors.Is(err, domain.ErrAgentInactive):
		return status.Error(codes.FailedPrecondition, "agent is inactive")
	case errors.Is(err, domain.ErrAgentAlreadyExists):
		return status.Error(codes.AlreadyExists, "agent already exists")
	case errors.Is(err, domain.ErrInvalidEnvironment):
		return status.Error(codes.InvalidArgument, "invalid environment")
	case errors.Is(err, domain.ErrDuplicateIdempotencyKey):
		return status.Error(codes.AlreadyExists, "duplicate idempotency key")
	case errors.Is(err, sql.ErrNoRows):
		return status.Error(codes.NotFound, "resource not found")
	case err != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)):
		return status.Error(codes.Canceled, "request canceled")
	default:
		// Log internal errors but don't expose details to client
		return status.Error(codes.Internal, "internal server error")
	}
}
