package agent

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kevin07696/payment-service/internal/adapters/database"
	adapterports "github.com/kevin07696/payment-service/internal/adapters/ports"
	"github.com/kevin07696/payment-service/internal/db/sqlc"
	"github.com/kevin07696/payment-service/internal/domain"
	"github.com/kevin07696/payment-service/internal/services/ports"
	"go.uber.org/zap"
)

// agentService implements the AgentService port
type agentService struct {
	db            *database.PostgreSQLAdapter
	secretManager adapterports.SecretManagerAdapter
	logger        *zap.Logger
}

// NewAgentService creates a new agent service
func NewAgentService(
	db *database.PostgreSQLAdapter,
	secretManager adapterports.SecretManagerAdapter,
	logger *zap.Logger,
) ports.AgentService {
	return &agentService{
		db:            db,
		secretManager: secretManager,
		logger:        logger,
	}
}

// RegisterAgent adds a new agent/merchant to the system
func (s *agentService) RegisterAgent(ctx context.Context, req *ports.RegisterAgentRequest) (*domain.Agent, error) {
	s.logger.Info("Registering new agent",
		zap.String("agent_id", req.AgentID),
		zap.String("environment", string(req.Environment)),
	)

	// Check idempotency
	if req.IdempotencyKey != nil {
		existing, err := s.getAgentByIdempotencyKey(ctx, *req.IdempotencyKey)
		if err == nil {
			s.logger.Info("Idempotent request, returning existing agent",
				zap.String("agent_id", existing.AgentID),
			)
			return existing, nil
		}
	}

	// Validate agent_id is unique
	exists, err := s.db.Queries().AgentExists(ctx, req.AgentID)
	if err != nil {
		return nil, fmt.Errorf("failed to check agent existence: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("agent_id already exists")
	}

	// Validate EPX credentials are provided
	if req.CustNbr == "" || req.MerchNbr == "" || req.DBAnbr == "" || req.TerminalNbr == "" {
		return nil, fmt.Errorf("all EPX credentials (cust_nbr, merch_nbr, dba_nbr, terminal_nbr) are required")
	}

	// Validate MAC secret is provided
	if req.MACSecret == "" {
		return nil, fmt.Errorf("mac_secret is required")
	}

	// Generate MAC secret path
	macSecretPath := fmt.Sprintf("payment-service/agents/%s/mac", req.AgentID)

	var agent *domain.Agent
	err = s.db.WithTx(ctx, func(q *sqlc.Queries) error {
		// Store MAC secret in secret manager
		_, err := s.secretManager.PutSecret(ctx, macSecretPath, req.MACSecret, nil)
		if err != nil {
			return fmt.Errorf("failed to store MAC secret: %w", err)
		}

		// Create agent in database
		params := sqlc.CreateAgentParams{
			ID:            uuid.New(),
			AgentID:       req.AgentID,
			CustNbr:       req.CustNbr,
			MerchNbr:      req.MerchNbr,
			DbaNbr:        req.DBAnbr,
			TerminalNbr:   req.TerminalNbr,
			MacSecretPath: macSecretPath,
			Environment:   string(req.Environment),
			IsActive:      pgtype.Bool{Bool: true, Valid: true},
			AgentName:     req.AgentName,
		}

		dbAgent, err := q.CreateAgent(ctx, params)
		if err != nil {
			return fmt.Errorf("failed to create agent: %w", err)
		}

		agent = sqlcAgentToDomain(&dbAgent)
		return nil
	})

	if err != nil {
		return nil, err
	}

	s.logger.Info("Agent registered successfully",
		zap.String("agent_id", agent.AgentID),
		zap.String("environment", string(agent.Environment)),
	)

	return agent, nil
}

// GetAgent retrieves agent credentials (internal use only)
func (s *agentService) GetAgent(ctx context.Context, agentID string) (*domain.Agent, error) {
	dbAgent, err := s.db.Queries().GetAgentByAgentID(ctx, agentID)
	if err != nil {
		s.logger.Debug("Agent not found",
			zap.String("agent_id", agentID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("agent not found: %w", err)
	}

	return sqlcAgentToDomain(&dbAgent), nil
}

// ListAgents lists all registered agents
func (s *agentService) ListAgents(ctx context.Context, environment *domain.Environment, isActive *bool, limit, offset int) ([]*domain.Agent, int, error) {
	var envStr pgtype.Text
	if environment != nil {
		envStr = pgtype.Text{String: string(*environment), Valid: true}
	}

	var activeFlag pgtype.Bool
	if isActive != nil {
		activeFlag = pgtype.Bool{Bool: *isActive, Valid: true}
	}

	params := sqlc.ListAgentsParams{
		Environment: envStr,
		IsActive:    activeFlag,
		LimitVal:    int32(limit),
		OffsetVal:   int32(offset),
	}

	dbAgents, err := s.db.Queries().ListAgents(ctx, params)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list agents: %w", err)
	}

	countParams := sqlc.CountAgentsParams{
		Environment: envStr,
		IsActive:    activeFlag,
	}

	count, err := s.db.Queries().CountAgents(ctx, countParams)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count agents: %w", err)
	}

	agents := make([]*domain.Agent, len(dbAgents))
	for i, dbAgent := range dbAgents {
		agents[i] = sqlcAgentToDomain(&dbAgent)
	}

	return agents, int(count), nil
}

// UpdateAgent updates agent credentials
func (s *agentService) UpdateAgent(ctx context.Context, req *ports.UpdateAgentRequest) (*domain.Agent, error) {
	s.logger.Info("Updating agent",
		zap.String("agent_id", req.AgentID),
	)

	// Check idempotency
	if req.IdempotencyKey != nil {
		existing, err := s.getAgentByIdempotencyKey(ctx, *req.IdempotencyKey)
		if err == nil {
			return existing, nil
		}
	}

	// Get existing agent
	existing, err := s.db.Queries().GetAgentByAgentID(ctx, req.AgentID)
	if err != nil {
		return nil, fmt.Errorf("agent not found: %w", err)
	}

	var agent *domain.Agent
	err = s.db.WithTx(ctx, func(q *sqlc.Queries) error {
		// If MAC secret is being rotated, update it in secret manager
		if req.MACSecret != nil {
			_, err := s.secretManager.PutSecret(ctx, existing.MacSecretPath, *req.MACSecret, nil)
			if err != nil {
				return fmt.Errorf("failed to update MAC secret: %w", err)
			}
		}

		// Build update params with defaults from existing
		params := sqlc.UpdateAgentParams{
			AgentID:     req.AgentID,
			CustNbr:     valueOrDefault(req.CustNbr, existing.CustNbr),
			MerchNbr:    valueOrDefault(req.MerchNbr, existing.MerchNbr),
			DbaNbr:      valueOrDefault(req.DBAnbr, existing.DbaNbr),
			TerminalNbr: valueOrDefault(req.TerminalNbr, existing.TerminalNbr),
			Environment: valueOrEnvironment(req.Environment, existing.Environment),
			AgentName:   valueOrDefault(req.AgentName, existing.AgentName),
		}

		dbAgent, err := q.UpdateAgent(ctx, params)
		if err != nil {
			return fmt.Errorf("failed to update agent: %w", err)
		}

		agent = sqlcAgentToDomain(&dbAgent)
		return nil
	})

	if err != nil {
		return nil, err
	}

	s.logger.Info("Agent updated successfully",
		zap.String("agent_id", agent.AgentID),
	)

	return agent, nil
}

// DeactivateAgent deactivates an agent
func (s *agentService) DeactivateAgent(ctx context.Context, agentID, reason string) error {
	s.logger.Info("Deactivating agent",
		zap.String("agent_id", agentID),
		zap.String("reason", reason),
	)

	// Verify agent exists
	_, err := s.db.Queries().GetAgentByAgentID(ctx, agentID)
	if err != nil {
		return fmt.Errorf("agent not found: %w", err)
	}

	err = s.db.Queries().DeactivateAgent(ctx, agentID)
	if err != nil {
		return fmt.Errorf("failed to deactivate agent: %w", err)
	}

	s.logger.Info("Agent deactivated successfully",
		zap.String("agent_id", agentID),
	)

	return nil
}

// RotateMAC rotates MAC secret in secret manager
func (s *agentService) RotateMAC(ctx context.Context, req *ports.RotateMACRequest) error {
	s.logger.Info("Rotating MAC secret",
		zap.String("agent_id", req.AgentID),
	)

	// Get agent to retrieve MAC secret path
	agent, err := s.db.Queries().GetAgentByAgentID(ctx, req.AgentID)
	if err != nil {
		return fmt.Errorf("agent not found: %w", err)
	}

	if !agent.IsActive.Valid || !agent.IsActive.Bool {
		return fmt.Errorf("cannot rotate MAC for inactive agent")
	}

	// Update MAC secret in secret manager
	_, err = s.secretManager.PutSecret(ctx, agent.MacSecretPath, req.NewMACSecret, nil)
	if err != nil {
		return fmt.Errorf("failed to rotate MAC secret: %w", err)
	}

	s.logger.Info("MAC secret rotated successfully",
		zap.String("agent_id", req.AgentID),
	)

	return nil
}

// getAgentByIdempotencyKey retrieves an agent by idempotency key
func (s *agentService) getAgentByIdempotencyKey(ctx context.Context, key string) (*domain.Agent, error) {
	// Note: This would require adding idempotency_key to agents table
	// For now, returning not found error
	return nil, fmt.Errorf("agent not found")
}

// Helper functions

func sqlcAgentToDomain(dbAgent *sqlc.AgentCredential) *domain.Agent {
	return &domain.Agent{
		ID:            dbAgent.ID.String(),
		AgentID:       dbAgent.AgentID,
		CustNbr:       dbAgent.CustNbr,
		MerchNbr:      dbAgent.MerchNbr,
		DBAnbr:        dbAgent.DbaNbr,
		TerminalNbr:   dbAgent.TerminalNbr,
		MACSecretPath: dbAgent.MacSecretPath,
		Environment:   domain.Environment(dbAgent.Environment),
		IsActive:      dbAgent.IsActive.Bool,
		CreatedAt:     dbAgent.CreatedAt,
		UpdatedAt:     dbAgent.UpdatedAt,
	}
}

func valueOrDefault(value *string, defaultValue string) string {
	if value != nil {
		return *value
	}
	return defaultValue
}

func valueOrEnvironment(value *domain.Environment, defaultValue string) string {
	if value != nil {
		return string(*value)
	}
	return defaultValue
}
