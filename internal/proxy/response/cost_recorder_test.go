package response

import (
	"context"
	"testing"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/quagmt/udecimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------
// mocks
// ---------------------------------------------------------------

type mockMessageRequestRepo struct {
	mock.Mock
}

func (m *mockMessageRequestRepo) UpdateAfterResponse(ctx context.Context, id int, fields *MessageRequestUpdate) error {
	args := m.Called(ctx, id, fields)
	return args.Error(0)
}

type mockUsageLedgerRepo struct {
	mock.Mock
}

func (m *mockUsageLedgerRepo) Create(ctx context.Context, entry *model.UsageLedger) error {
	args := m.Called(ctx, entry)
	return args.Error(0)
}

type mockRateLimitService struct {
	mock.Mock
}

func (m *mockRateLimitService) RecordUsage(ctx context.Context, userID, keyID, providerID int, costUSD udecimal.Decimal) error {
	args := m.Called(ctx, userID, keyID, providerID, costUSD)
	return args.Error(0)
}

// ---------------------------------------------------------------
// CostRecorder.RecordCost
// ---------------------------------------------------------------

func TestCostRecorder_RecordCost_Success(t *testing.T) {
	mrRepo := new(mockMessageRequestRepo)
	ulRepo := new(mockUsageLedgerRepo)
	rlSvc := new(mockRateLimitService)

	mrRepo.On("UpdateAfterResponse", mock.Anything, 42, mock.AnythingOfType("*response.MessageRequestUpdate")).Return(nil)
	ulRepo.On("Create", mock.Anything, mock.AnythingOfType("*model.UsageLedger")).Return(nil)
	rlSvc.On("RecordUsage", mock.Anything, 1, 10, 5, mock.AnythingOfType("udecimal.Decimal")).Return(nil)

	recorder := NewCostRecorder(mrRepo, ulRepo, rlSvc)

	req := RecordCostRequest{
		MessageRequestID: 42,
		Usage: &Usage{
			InputTokens:  100,
			OutputTokens: 50,
		},
		CostUSD:        udecimal.MustParse("0.005"),
		CostMultiplier: udecimal.MustParse("1.5"),
		ProviderID:     5,
		FinalProviderID: 5,
		Model:          "claude-4-sonnet",
		UserID:         1,
		KeyID:          10,
		KeyName:        "test-key",
		TTFBMs:         250,
		DurationMs:     1500,
		StatusCode:     200,
		IsSuccess:      true,
	}

	err := recorder.RecordCost(context.Background(), req)
	require.NoError(t, err)

	mrRepo.AssertExpectations(t)
	ulRepo.AssertExpectations(t)
	rlSvc.AssertExpectations(t)
}

func TestCostRecorder_RecordCost_NilUsage(t *testing.T) {
	mrRepo := new(mockMessageRequestRepo)
	ulRepo := new(mockUsageLedgerRepo)
	rlSvc := new(mockRateLimitService)

	mrRepo.On("UpdateAfterResponse", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	ulRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
	rlSvc.On("RecordUsage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	recorder := NewCostRecorder(mrRepo, ulRepo, rlSvc)

	req := RecordCostRequest{
		MessageRequestID: 1,
		Usage:            nil,
		CostUSD:          udecimal.Zero,
		UserID:           1,
		KeyID:            1,
		KeyName:          "k",
		StatusCode:       200,
		IsSuccess:        true,
	}

	err := recorder.RecordCost(context.Background(), req)
	require.NoError(t, err)
}

func TestCostRecorder_RecordCost_PartialFailure(t *testing.T) {
	mrRepo := new(mockMessageRequestRepo)
	ulRepo := new(mockUsageLedgerRepo)
	rlSvc := new(mockRateLimitService)

	mrRepo.On("UpdateAfterResponse", mock.Anything, mock.Anything, mock.Anything).Return(assert.AnError)
	ulRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
	rlSvc.On("RecordUsage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	recorder := NewCostRecorder(mrRepo, ulRepo, rlSvc)

	req := RecordCostRequest{
		MessageRequestID: 1,
		Usage:            &Usage{InputTokens: 10},
		CostUSD:          udecimal.MustParse("0.001"),
		UserID:           1,
		KeyID:            1,
		KeyName:          "k",
		StatusCode:       200,
		IsSuccess:        true,
	}

	err := recorder.RecordCost(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update message_request")

	// The other two should still have been called.
	ulRepo.AssertExpectations(t)
	rlSvc.AssertExpectations(t)
}

func TestCostRecorder_RecordCost_AllFieldsPopulated(t *testing.T) {
	mrRepo := new(mockMessageRequestRepo)
	ulRepo := new(mockUsageLedgerRepo)
	rlSvc := new(mockRateLimitService)

	mrRepo.On("UpdateAfterResponse", mock.Anything, 99, mock.MatchedBy(func(u *MessageRequestUpdate) bool {
		return u.StatusCode != nil && *u.StatusCode == 200 &&
			u.TtfbMs != nil && *u.TtfbMs == 150 &&
			u.InputTokens != nil && *u.InputTokens == 500 &&
			u.CacheCreationInputTokens != nil && *u.CacheCreationInputTokens == 30
	})).Return(nil)

	ulRepo.On("Create", mock.Anything, mock.MatchedBy(func(entry *model.UsageLedger) bool {
		return entry.UserID == 2 &&
			entry.ProviderID == 7 &&
			entry.IsSuccess &&
			entry.InputTokens != nil && *entry.InputTokens == 500
	})).Return(nil)

	rlSvc.On("RecordUsage", mock.Anything, 2, 20, 7, mock.Anything).Return(nil)

	recorder := NewCostRecorder(mrRepo, ulRepo, rlSvc)

	req := RecordCostRequest{
		MessageRequestID:    99,
		Usage:               &Usage{InputTokens: 500, OutputTokens: 200, CacheCreationTokens: 30},
		CostUSD:             udecimal.MustParse("0.01"),
		CostMultiplier:      udecimal.MustParse("2.0"),
		GroupCostMultiplier:  udecimal.MustParse("1.5"),
		ProviderID:          7,
		FinalProviderID:     7,
		Model:               "claude-4-opus",
		OriginalModel:       "claude-4-opus-latest",
		UserID:              2,
		KeyID:               20,
		KeyName:             "prod-key",
		SessionID:           "sess-abc",
		Endpoint:            "/v1/messages",
		APIType:             "claude",
		TTFBMs:              150,
		DurationMs:          2000,
		StatusCode:          200,
		CacheTtlApplied:     "ephemeral",
		Context1mApplied:    true,
		SwapCacheTtlApplied: false,
		ClientIP:            "1.2.3.4",
		IsSuccess:           true,
	}

	err := recorder.RecordCost(context.Background(), req)
	require.NoError(t, err)

	mrRepo.AssertExpectations(t)
	ulRepo.AssertExpectations(t)
	rlSvc.AssertExpectations(t)
}

// ---------------------------------------------------------------
// helpers
// ---------------------------------------------------------------

func TestIntPtr(t *testing.T) {
	p := intPtr(42)
	require.NotNil(t, p)
	assert.Equal(t, 42, *p)
}

func TestStrPtr(t *testing.T) {
	p := strPtr("hello")
	require.NotNil(t, p)
	assert.Equal(t, "hello", *p)
}

// ---------------------------------------------------------------
// RecordCostRequest field coverage -- ledger timestamps
// ---------------------------------------------------------------

func TestCostRecorder_LedgerTimestamp(t *testing.T) {
	mrRepo := new(mockMessageRequestRepo)
	ulRepo := new(mockUsageLedgerRepo)
	rlSvc := new(mockRateLimitService)

	mrRepo.On("UpdateAfterResponse", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	rlSvc.On("RecordUsage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	var capturedLedger *model.UsageLedger
	ulRepo.On("Create", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		capturedLedger = args.Get(1).(*model.UsageLedger)
	}).Return(nil)

	recorder := NewCostRecorder(mrRepo, ulRepo, rlSvc)

	before := time.Now()
	_ = recorder.RecordCost(context.Background(), RecordCostRequest{
		MessageRequestID: 1,
		Usage:            &Usage{},
		CostUSD:          udecimal.Zero,
		UserID:           1,
		KeyID:            1,
		KeyName:          "k",
		IsSuccess:        true,
	})
	after := time.Now()

	require.NotNil(t, capturedLedger)
	assert.True(t, !capturedLedger.CreatedAt.Before(before))
	assert.True(t, !capturedLedger.CreatedAt.After(after))
}
