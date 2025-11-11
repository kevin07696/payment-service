package chargeback

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/kevin07696/payment-service/internal/db/sqlc"
	chargebackv1 "github.com/kevin07696/payment-service/proto/chargeback/v1"
	"go.uber.org/zap"
)

// QueryExecutor defines the interface for executing database queries
type QueryExecutor interface {
	GetChargebackByID(ctx context.Context, id uuid.UUID) (sqlc.Chargeback, error)
	ListChargebacks(ctx context.Context, params sqlc.ListChargebacksParams) ([]sqlc.Chargeback, error)
	CountChargebacks(ctx context.Context, params sqlc.CountChargebacksParams) (int64, error)
}

// Handler implements the gRPC ChargebackServiceServer
type Handler struct {
	chargebackv1.UnimplementedChargebackServiceServer
	queries QueryExecutor
	logger  *zap.Logger
}

// NewHandlerWithQueries creates a new chargeback handler with a custom query executor
func NewHandlerWithQueries(queries QueryExecutor, logger *zap.Logger) *Handler {
	return &Handler{
		queries: queries,
		logger:  logger,
	}
}

// DatabaseAdapter wraps a database adapter to extract queries
type DatabaseAdapter interface {
	Queries() sqlc.Querier
}

// NewHandler creates a new chargeback handler from a database adapter
func NewHandler(db DatabaseAdapter, logger *zap.Logger) *Handler {
	return &Handler{
		queries: db.Queries(),
		logger:  logger,
	}
}

// GetChargeback retrieves a specific chargeback by ID
func (h *Handler) GetChargeback(ctx context.Context, req *chargebackv1.GetChargebackRequest) (*chargebackv1.Chargeback, error) {
	h.logger.Info("GetChargeback request received",
		zap.String("chargeback_id", req.ChargebackId),
		zap.String("agent_id", req.AgentId),
	)

	// Validate inputs
	if req.ChargebackId == "" {
		return nil, status.Error(codes.InvalidArgument, "chargeback_id is required")
	}
	if req.AgentId == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
	}

	// Parse UUID
	chargebackID, err := uuid.Parse(req.ChargebackId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid chargeback_id format")
	}

	// Query database
	chargeback, err := h.queries.GetChargebackByID(ctx, chargebackID)
	if err != nil {
		h.logger.Error("Failed to get chargeback",
			zap.String("chargeback_id", req.ChargebackId),
			zap.Error(err),
		)
		return nil, status.Error(codes.NotFound, "chargeback not found")
	}

	// Verify agent authorization
	if chargeback.AgentID != req.AgentId {
		h.logger.Warn("Unauthorized chargeback access attempt",
			zap.String("chargeback_id", req.ChargebackId),
			zap.String("requested_agent", req.AgentId),
			zap.String("actual_agent", chargeback.AgentID),
		)
		return nil, status.Error(codes.PermissionDenied, "not authorized to access this chargeback")
	}

	// Convert to proto
	return convertChargebackToProto(&chargeback), nil
}

// ListChargebacks retrieves chargebacks with flexible filters
func (h *Handler) ListChargebacks(ctx context.Context, req *chargebackv1.ListChargebacksRequest) (*chargebackv1.ListChargebacksResponse, error) {
	h.logger.Info("ListChargebacks request received",
		zap.String("agent_id", req.AgentId),
	)

	// Validate required fields
	if req.AgentId == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
	}

	// Set defaults
	if req.Limit == 0 {
		req.Limit = 100
	}
	if req.Limit > 1000 {
		req.Limit = 1000 // Cap at 1000
	}

	// Build query params
	params := sqlc.ListChargebacksParams{
		AgentID:   pgtype.Text{String: req.AgentId, Valid: true},
		LimitVal:  req.Limit,
		OffsetVal: req.Offset,
	}

	// Optional filters
	if req.CustomerId != nil && *req.CustomerId != "" {
		params.CustomerID = pgtype.Text{String: *req.CustomerId, Valid: true}
	} else {
		params.CustomerID = pgtype.Text{Valid: false}
	}

	if req.GroupId != nil && *req.GroupId != "" {
		groupID, err := uuid.Parse(*req.GroupId)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid group_id format")
		}
		params.GroupID = pgtype.UUID{Bytes: groupID, Valid: true}
	} else {
		params.GroupID = pgtype.UUID{Valid: false}
	}

	if req.Status != nil && *req.Status != chargebackv1.ChargebackStatus_CHARGEBACK_STATUS_UNSPECIFIED {
		params.Status = pgtype.Text{String: mapProtoStatusToDomain(*req.Status), Valid: true}
	} else {
		params.Status = pgtype.Text{Valid: false}
	}

	// Date filters
	if req.DisputeDateFrom != nil {
		fromDate := req.DisputeDateFrom.AsTime()
		params.DisputeDateFrom = pgtype.Date{Time: fromDate, Valid: true}
	} else {
		params.DisputeDateFrom = pgtype.Date{Valid: false}
	}

	if req.DisputeDateTo != nil {
		toDate := req.DisputeDateTo.AsTime()
		params.DisputeDateTo = pgtype.Date{Time: toDate, Valid: true}
	} else {
		params.DisputeDateTo = pgtype.Date{Valid: false}
	}

	// Query database
	chargebacks, err := h.queries.ListChargebacks(ctx, params)
	if err != nil {
		h.logger.Error("Failed to list chargebacks",
			zap.String("agent_id", req.AgentId),
			zap.Error(err),
		)
		return nil, status.Error(codes.Internal, "failed to list chargebacks")
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
		zap.String("agent_id", req.AgentId),
		zap.Int("total_count", int(totalCount)),
		zap.Int("returned_count", len(protoChargebacks)),
	)

	return &chargebackv1.ListChargebacksResponse{
		Chargebacks: protoChargebacks,
		TotalCount:  int32(totalCount),
	}, nil
}

// convertChargebackToProto converts database model to protobuf message
func convertChargebackToProto(cb *sqlc.Chargeback) *chargebackv1.Chargeback {
	proto := &chargebackv1.Chargeback{
		Id:               cb.ID.String(),
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

	if cb.GroupID.Valid {
		groupUUID := uuid.UUID(cb.GroupID.Bytes)
		proto.GroupId = groupUUID.String()
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

// Status mapping functions

// mapDomainStatusToProto maps database status to proto enum
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

// mapProtoStatusToDomain maps proto enum to database status
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
