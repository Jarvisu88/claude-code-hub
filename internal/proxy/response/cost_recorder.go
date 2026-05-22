package response

import (
	"context"
	"fmt"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/quagmt/udecimal"
)

// MessageRequestRepo persists message request records.
type MessageRequestRepo interface {
	UpdateAfterResponse(ctx context.Context, id int, fields *MessageRequestUpdate) error
}

// MessageRequestUpdate carries the fields that need updating after a
// response has been received and costs have been calculated.
type MessageRequestUpdate struct {
	StatusCode               *int
	DurationMs               *int
	TtfbMs                   *int
	InputTokens              *int
	OutputTokens             *int
	CacheCreationInputTokens *int
	CacheReadInputTokens     *int
	CostUSD                  udecimal.Decimal
	CostMultiplier           *udecimal.Decimal
	GroupCostMultiplier      *udecimal.Decimal
	CacheTtlApplied          *string
	SpecialSettings          []model.SpecialSetting
	ErrorMessage             *string
}

// UsageLedgerRepo creates immutable usage ledger entries.
type UsageLedgerRepo interface {
	Create(ctx context.Context, entry *model.UsageLedger) error
}

// SessionManager manages session-to-provider bindings.
type SessionManager interface {
	BindProviderToSession(ctx context.Context, sessionID string, providerID int) error
}

// RateLimitService records usage for downstream rate limiting.
type RateLimitService interface {
	RecordUsage(ctx context.Context, userID int, keyID int, providerID int, costUSD udecimal.Decimal) error
}

// CostCalculator computes cost from usage data.
type CostCalculator interface {
	CalculateCost(ctx context.Context, model string, usage *Usage, costMultiplier udecimal.Decimal) (udecimal.Decimal, error)
}

// -------------------------------------------------------------------
// CostRecorder
// -------------------------------------------------------------------

// RecordCostRequest bundles everything needed to persist cost data.
type RecordCostRequest struct {
	MessageRequestID    int
	Usage               *Usage
	CostUSD             udecimal.Decimal
	CostMultiplier      udecimal.Decimal
	GroupCostMultiplier  udecimal.Decimal
	ProviderID          int
	FinalProviderID     int
	Model               string
	OriginalModel       string
	UserID              int
	KeyID               int
	KeyName             string
	SessionID           string
	Endpoint            string
	APIType             string
	TTFBMs              int
	DurationMs          int
	StatusCode          int
	CacheTtlApplied     string
	Context1mApplied    bool
	SwapCacheTtlApplied bool
	SpecialSettings     []model.SpecialSetting
	ClientIP            string
	IsSuccess           bool
}

// CostRecorder records costs to the database and updates rate limit counters.
type CostRecorder struct {
	messageRepo     MessageRequestRepo
	usageLedgerRepo UsageLedgerRepo
	rateLimitSvc    RateLimitService
}

// NewCostRecorder creates a CostRecorder.
func NewCostRecorder(
	messageRepo MessageRequestRepo,
	usageLedgerRepo UsageLedgerRepo,
	rateLimitSvc RateLimitService,
) *CostRecorder {
	return &CostRecorder{
		messageRepo:     messageRepo,
		usageLedgerRepo: usageLedgerRepo,
		rateLimitSvc:    rateLimitSvc,
	}
}

// RecordCost persists cost data to message_request + usage_ledger and
// updates rate limit counters. Errors are collected but the function
// attempts all three writes regardless of individual failures.
func (r *CostRecorder) RecordCost(ctx context.Context, req RecordCostRequest) error {
	var errs []error

	// 1. Update message_request record
	mrUpdate := &MessageRequestUpdate{
		StatusCode:  intPtr(req.StatusCode),
		DurationMs:  intPtr(req.DurationMs),
		CostUSD:     req.CostUSD,
		SpecialSettings: req.SpecialSettings,
	}
	if req.TTFBMs > 0 {
		mrUpdate.TtfbMs = intPtr(req.TTFBMs)
	}
	if req.Usage != nil {
		mrUpdate.InputTokens = intPtr(req.Usage.InputTokens)
		mrUpdate.OutputTokens = intPtr(req.Usage.OutputTokens)
		if req.Usage.CacheCreationTokens > 0 {
			mrUpdate.CacheCreationInputTokens = intPtr(req.Usage.CacheCreationTokens)
		}
		if req.Usage.CacheReadTokens > 0 {
			mrUpdate.CacheReadInputTokens = intPtr(req.Usage.CacheReadTokens)
		}
	}
	if !req.CostMultiplier.IsZero() {
		mrUpdate.CostMultiplier = &req.CostMultiplier
	}
	if !req.GroupCostMultiplier.IsZero() {
		mrUpdate.GroupCostMultiplier = &req.GroupCostMultiplier
	}
	if req.CacheTtlApplied != "" {
		mrUpdate.CacheTtlApplied = strPtr(req.CacheTtlApplied)
	}

	if err := r.messageRepo.UpdateAfterResponse(ctx, req.MessageRequestID, mrUpdate); err != nil {
		errs = append(errs, fmt.Errorf("update message_request: %w", err))
	}

	// 2. Create usage_ledger entry
	ledger := &model.UsageLedger{
		RequestID:       req.MessageRequestID,
		UserID:          req.UserID,
		Key:             req.KeyName,
		ProviderID:      req.ProviderID,
		FinalProviderID: req.FinalProviderID,
		CostUSD:         req.CostUSD,
		IsSuccess:       req.IsSuccess,
		CreatedAt:       time.Now(),
	}
	if req.Model != "" {
		ledger.Model = strPtr(req.Model)
	}
	if req.OriginalModel != "" {
		ledger.OriginalModel = strPtr(req.OriginalModel)
	}
	if req.Endpoint != "" {
		ledger.Endpoint = strPtr(req.Endpoint)
	}
	if req.APIType != "" {
		ledger.APIType = strPtr(req.APIType)
	}
	if req.SessionID != "" {
		ledger.SessionID = strPtr(req.SessionID)
	}
	if req.StatusCode > 0 {
		ledger.StatusCode = intPtr(req.StatusCode)
	}
	if !req.CostMultiplier.IsZero() {
		cm := req.CostMultiplier
		ledger.CostMultiplier = &cm
	}
	if !req.GroupCostMultiplier.IsZero() {
		gcm := req.GroupCostMultiplier
		ledger.GroupCostMultiplier = &gcm
	}
	if req.Usage != nil {
		if req.Usage.InputTokens > 0 {
			v := int64(req.Usage.InputTokens)
			ledger.InputTokens = &v
		}
		if req.Usage.OutputTokens > 0 {
			v := int64(req.Usage.OutputTokens)
			ledger.OutputTokens = &v
		}
		if req.Usage.CacheCreationTokens > 0 {
			v := int64(req.Usage.CacheCreationTokens)
			ledger.CacheCreationInputTokens = &v
		}
		if req.Usage.CacheReadTokens > 0 {
			v := int64(req.Usage.CacheReadTokens)
			ledger.CacheReadInputTokens = &v
		}
	}
	if req.DurationMs > 0 {
		ledger.DurationMs = intPtr(req.DurationMs)
	}
	if req.TTFBMs > 0 {
		ledger.TtfbMs = intPtr(req.TTFBMs)
	}
	if req.CacheTtlApplied != "" {
		ledger.CacheTtlApplied = strPtr(req.CacheTtlApplied)
	}
	ledger.Context1mApplied = req.Context1mApplied
	ledger.SwapCacheTtlApplied = req.SwapCacheTtlApplied
	if req.ClientIP != "" {
		ledger.ClientIP = strPtr(req.ClientIP)
	}

	if err := r.usageLedgerRepo.Create(ctx, ledger); err != nil {
		errs = append(errs, fmt.Errorf("create usage_ledger: %w", err))
	}

	// 3. Update rate limit counters
	if err := r.rateLimitSvc.RecordUsage(ctx, req.UserID, req.KeyID, req.ProviderID, req.CostUSD); err != nil {
		errs = append(errs, fmt.Errorf("record rate limit usage: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("cost recording errors: %v", errs)
	}
	return nil
}

// -------------------------------------------------------------------
// helpers
// -------------------------------------------------------------------

func intPtr(v int) *int       { return &v }
func strPtr(v string) *string { return &v }
