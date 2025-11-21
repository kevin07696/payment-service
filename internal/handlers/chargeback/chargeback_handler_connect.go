package chargeback

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/kevin07696/payment-service/internal/db/sqlc"
	chargebackv1 "github.com/kevin07696/payment-service/proto/chargeback/v1"
)

// QueryExecutor defines the interface for executing database queries
type QueryExecutor interface {
	GetChargebackByID(ctx context.Context, id uuid.UUID) (sqlc.Chargeback, error)
	ListChargebacks(ctx context.Context, params sqlc.ListChargebacksParams) ([]sqlc.Chargeback, error)
	CountChargebacks(ctx context.Context, params sqlc.CountChargebacksParams) (int64, error)
}

// DatabaseAdapter wraps database operations
type DatabaseAdapter interface {
	Queries() sqlc.Querier
}

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

	if msg.TransactionId != nil && *msg.TransactionId != "" {
		transactionID, err := uuid.Parse(*msg.TransactionId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid transaction_id format"))
		}
		params.TransactionID = pgtype.UUID{Bytes: transactionID, Valid: true}
	} else {
		params.TransactionID = pgtype.UUID{Valid: false}
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
		TransactionID:   params.TransactionID,
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

// convertChargebackToProto converts database model to protobuf message
func convertChargebackToProto(cb *sqlc.Chargeback) *chargebackv1.Chargeback {
	proto := &chargebackv1.Chargeback{
		Id:               cb.ID.String(),
		TransactionId:    cb.TransactionID.String(), // Transaction ID is always set (NOT NULL)
		AgentId:          cb.AgentID,
		CaseNumber:       cb.CaseNumber,
		DisputeDate:      timestamppb.New(cb.DisputeDate),
		ChargebackDate:   timestamppb.New(cb.ChargebackDate),
		ChargebackAmount: cb.ChargebackAmount,
		Currency:         cb.Currency,
		ReasonCode:       cb.ReasonCode,
		Status:           mapDomainStatusToProto(cb.Status),
		EvidenceFileUrls: cb.EvidenceFiles,
		CreatedAt:        timestamppb.New(cb.CreatedAt),
		UpdatedAt:        timestamppb.New(cb.UpdatedAt),
	}

	if cb.CustomerID.Valid {
		proto.CustomerId = cb.CustomerID.String
	}

	if cb.ReasonDescription.Valid {
		proto.ReasonDescription = cb.ReasonDescription.String
	}

	if cb.RespondByDate.Valid {
		proto.RespondByDate = timestamppb.New(cb.RespondByDate.Time)
	}

	if cb.ResponseSubmittedAt.Valid {
		proto.ResponseSubmittedAt = timestamppb.New(cb.ResponseSubmittedAt.Time)
	}

	if cb.ResolvedAt.Valid {
		proto.ResolvedAt = timestamppb.New(cb.ResolvedAt.Time)
	}

	if cb.ResponseNotes.Valid {
		proto.ResponseText = &cb.ResponseNotes.String
	}

	if cb.InternalNotes.Valid {
		proto.InternalNotes = &cb.InternalNotes.String
	}

	return proto
}

// mapDomainStatusToProto converts database status to protobuf enum
func mapDomainStatusToProto(domainStatus string) chargebackv1.ChargebackStatus {
	switch domainStatus {
	case "new":
		return chargebackv1.ChargebackStatus_CHARGEBACK_STATUS_NEW
	case "pending":
		return chargebackv1.ChargebackStatus_CHARGEBACK_STATUS_PENDING
	case "responded":
		return chargebackv1.ChargebackStatus_CHARGEBACK_STATUS_RESPONDED
	case "won":
		return chargebackv1.ChargebackStatus_CHARGEBACK_STATUS_WON
	case "lost":
		return chargebackv1.ChargebackStatus_CHARGEBACK_STATUS_LOST
	case "accepted":
		return chargebackv1.ChargebackStatus_CHARGEBACK_STATUS_ACCEPTED
	default:
		return chargebackv1.ChargebackStatus_CHARGEBACK_STATUS_UNSPECIFIED
	}
}

// mapProtoStatusToDomain converts protobuf enum to database status
func mapProtoStatusToDomain(protoStatus chargebackv1.ChargebackStatus) string {
	switch protoStatus {
	case chargebackv1.ChargebackStatus_CHARGEBACK_STATUS_NEW:
		return "new"
	case chargebackv1.ChargebackStatus_CHARGEBACK_STATUS_PENDING:
		return "pending"
	case chargebackv1.ChargebackStatus_CHARGEBACK_STATUS_RESPONDED:
		return "responded"
	case chargebackv1.ChargebackStatus_CHARGEBACK_STATUS_WON:
		return "won"
	case chargebackv1.ChargebackStatus_CHARGEBACK_STATUS_LOST:
		return "lost"
	case chargebackv1.ChargebackStatus_CHARGEBACK_STATUS_ACCEPTED:
		return "accepted"
	default:
		return "new"
	}
}
