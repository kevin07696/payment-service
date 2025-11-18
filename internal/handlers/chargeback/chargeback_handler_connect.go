package chargeback

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"

	"github.com/kevin07696/payment-service/internal/db/sqlc"
	chargebackv1 "github.com/kevin07696/payment-service/proto/chargeback/v1"
)

// ConnectHandler implements the Connect RPC ChargebackServiceHandler interface
type ConnectHandler struct {
	queries QueryExecutor
	logger  *zap.Logger
}

// NewConnectHandlerWithQueries creates a new Connect RPC chargeback handler with a custom query executor
func NewConnectHandlerWithQueries(queries QueryExecutor, logger *zap.Logger) *ConnectHandler {
	return &ConnectHandler{
		queries: queries,
		logger:  logger,
	}
}

// NewConnectHandler creates a new Connect RPC chargeback handler from a database adapter
func NewConnectHandler(db DatabaseAdapter, logger *zap.Logger) *ConnectHandler {
	return &ConnectHandler{
		queries: db.Queries(),
		logger:  logger,
	}
}

// GetChargeback retrieves a specific chargeback by ID
func (h *ConnectHandler) GetChargeback(
	ctx context.Context,
	req *connect.Request[chargebackv1.GetChargebackRequest],
) (*connect.Response[chargebackv1.Chargeback], error) {
	msg := req.Msg

	h.logger.Info("GetChargeback request received",
		zap.String("chargeback_id", msg.ChargebackId),
		zap.String("agent_id", msg.AgentId),
	)

	// Validate inputs
	if msg.ChargebackId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("chargeback_id is required"))
	}
	if msg.AgentId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("agent_id is required"))
	}

	// Parse UUID
	chargebackID, err := uuid.Parse(msg.ChargebackId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid chargeback_id format"))
	}

	// Query database
	chargeback, err := h.queries.GetChargebackByID(ctx, chargebackID)
	if err != nil {
		h.logger.Error("Failed to get chargeback",
			zap.String("chargeback_id", msg.ChargebackId),
			zap.Error(err),
		)
		return nil, connect.NewError(connect.CodeNotFound, errors.New("chargeback not found"))
	}

	// Verify agent authorization
	if chargeback.AgentID != msg.AgentId {
		h.logger.Warn("Unauthorized chargeback access attempt",
			zap.String("chargeback_id", msg.ChargebackId),
			zap.String("requested_agent", msg.AgentId),
			zap.String("actual_agent", chargeback.AgentID),
		)
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("not authorized to access this chargeback"))
	}

	// Convert to proto and wrap in Connect response
	return connect.NewResponse(convertChargebackToProto(&chargeback)), nil
}

// ListChargebacks retrieves chargebacks with flexible filters
func (h *ConnectHandler) ListChargebacks(
	ctx context.Context,
	req *connect.Request[chargebackv1.ListChargebacksRequest],
) (*connect.Response[chargebackv1.ListChargebacksResponse], error) {
	msg := req.Msg

	h.logger.Info("ListChargebacks request received",
		zap.String("agent_id", msg.AgentId),
	)

	// Validate required fields
	if msg.AgentId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("agent_id is required"))
	}

	// Set defaults
	if msg.Limit == 0 {
		msg.Limit = 100
	}
	if msg.Limit > 1000 {
		msg.Limit = 1000 // Cap at 1000
	}

	// Build query params
	params := sqlc.ListChargebacksParams{
		AgentID:   pgtype.Text{String: msg.AgentId, Valid: true},
		LimitVal:  msg.Limit,
		OffsetVal: msg.Offset,
	}

	// Optional filters
	if msg.CustomerId != nil && *msg.CustomerId != "" {
		params.CustomerID = pgtype.Text{String: *msg.CustomerId, Valid: true}
	} else {
		params.CustomerID = pgtype.Text{Valid: false}
	}

	if msg.GroupId != nil && *msg.GroupId != "" {
		groupID, err := uuid.Parse(*msg.GroupId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid group_id format"))
		}
		params.GroupID = pgtype.UUID{Bytes: groupID, Valid: true}
	} else {
		params.GroupID = pgtype.UUID{Valid: false}
	}

	if msg.Status != nil && *msg.Status != chargebackv1.ChargebackStatus_CHARGEBACK_STATUS_UNSPECIFIED {
		params.Status = pgtype.Text{String: mapProtoStatusToDomain(*msg.Status), Valid: true}
	} else {
		params.Status = pgtype.Text{Valid: false}
	}

	// Date filters
	if msg.DisputeDateFrom != nil {
		fromDate := msg.DisputeDateFrom.AsTime()
		params.DisputeDateFrom = pgtype.Date{Time: fromDate, Valid: true}
	} else {
		params.DisputeDateFrom = pgtype.Date{Valid: false}
	}

	if msg.DisputeDateTo != nil {
		toDate := msg.DisputeDateTo.AsTime()
		params.DisputeDateTo = pgtype.Date{Time: toDate, Valid: true}
	} else {
		params.DisputeDateTo = pgtype.Date{Valid: false}
	}

	// Query database
	chargebacks, err := h.queries.ListChargebacks(ctx, params)
	if err != nil {
		h.logger.Error("Failed to list chargebacks",
			zap.String("agent_id", msg.AgentId),
			zap.Error(err),
		)
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to list chargebacks"))
	}

	// Get total count
	countParams := sqlc.CountChargebacksParams{
		AgentID:         params.AgentID,
		CustomerID:      params.CustomerID,
		GroupID:         params.GroupID,
		Status:          params.Status,
		DisputeDateFrom: params.DisputeDateFrom,
		DisputeDateTo:   params.DisputeDateTo,
	}

	totalCount, err := h.queries.CountChargebacks(ctx, countParams)
	if err != nil {
		h.logger.Warn("Failed to count chargebacks", zap.Error(err))
		totalCount = int64(len(chargebacks)) // Fallback
	}

	// Convert to proto
	protoChargebacks := make([]*chargebackv1.Chargeback, len(chargebacks))
	for i, cb := range chargebacks {
		protoChargebacks[i] = convertChargebackToProto(&cb)
	}

	h.logger.Info("Chargebacks retrieved",
		zap.String("agent_id", msg.AgentId),
		zap.Int("total_count", int(totalCount)),
		zap.Int("returned_count", len(protoChargebacks)),
	)

	response := &chargebackv1.ListChargebacksResponse{
		Chargebacks: protoChargebacks,
		TotalCount:  int32(totalCount),
	}

	return connect.NewResponse(response), nil
}
