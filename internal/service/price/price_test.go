package price

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- mock PriceRepo ---

type mockPriceRepo struct {
	prices map[string]*model.ModelPrice
}

func newMockPriceRepo() *mockPriceRepo {
	return &mockPriceRepo{prices: map[string]*model.ModelPrice{}}
}

func (m *mockPriceRepo) Create(_ context.Context, price *model.ModelPrice) (*model.ModelPrice, error) {
	price.ID = len(m.prices) + 1
	m.prices[price.ModelName] = price
	return price, nil
}

func (m *mockPriceRepo) GetLatestByModelName(_ context.Context, modelName string) (*model.ModelPrice, error) {
	p, ok := m.prices[modelName]
	if !ok {
		return nil, nil
	}
	return p, nil
}

func (m *mockPriceRepo) HasAnyRecords(_ context.Context) (bool, error) {
	return len(m.prices) > 0, nil
}

// --- Syncer tests ---

func TestSyncer_SyncFromLiteLLM(t *testing.T) {
	// Build a fake LiteLLM price table.
	priceTable := map[string]model.PriceData{
		"sample_spec": {Mode: strPtr("chat")}, // should be skipped
		"claude-opus-4-20250514": {
			InputCostPerToken:  f64Ptr(15e-6),
			OutputCostPerToken: f64Ptr(75e-6),
			LitellmProvider:    strPtr("anthropic"),
			Mode:               strPtr("chat"),
			MaxInputTokens:     intPtr(200000),
			MaxOutputTokens:    intPtr(32000),
		},
		"gpt-4o": {
			InputCostPerToken:  f64Ptr(2.5e-6),
			OutputCostPerToken: f64Ptr(10e-6),
			LitellmProvider:    strPtr("openai"),
			Mode:               strPtr("chat"),
		},
	}
	tableJSON, err := json.Marshal(priceTable)
	require.NoError(t, err)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(tableJSON)
	}))
	defer ts.Close()

	repo := newMockPriceRepo()
	syncer := NewSyncer(repo, ts.Client()).WithSourceURL(ts.URL)

	count, err := syncer.SyncFromLiteLLM(context.Background())
	require.NoError(t, err)

	// sample_spec is skipped, so we expect 2.
	assert.Equal(t, 2, count)
	assert.Len(t, repo.prices, 2)
	assert.Contains(t, repo.prices, "claude-opus-4-20250514")
	assert.Contains(t, repo.prices, "gpt-4o")
	assert.NotContains(t, repo.prices, "sample_spec")

	// Verify the actual price data was preserved.
	opus := repo.prices["claude-opus-4-20250514"]
	assert.Equal(t, "litellm", opus.Source)
	assert.InDelta(t, 15e-6, *opus.PriceData.InputCostPerToken, 1e-12)
}

func TestSyncer_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	repo := newMockPriceRepo()
	syncer := NewSyncer(repo, ts.Client()).WithSourceURL(ts.URL)

	_, err := syncer.SyncFromLiteLLM(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected status 500")
}

func TestSyncer_InvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not json"))
	}))
	defer ts.Close()

	repo := newMockPriceRepo()
	syncer := NewSyncer(repo, ts.Client()).WithSourceURL(ts.URL)

	_, err := syncer.SyncFromLiteLLM(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decode JSON")
}

func TestSyncer_EmptyTable(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	defer ts.Close()

	repo := newMockPriceRepo()
	syncer := NewSyncer(repo, ts.Client()).WithSourceURL(ts.URL)

	count, err := syncer.SyncFromLiteLLM(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

// --- Seed tests ---

func TestSeedDefaultPrices(t *testing.T) {
	repo := newMockPriceRepo()

	err := SeedDefaultPrices(context.Background(), repo)
	require.NoError(t, err)
	assert.Greater(t, len(repo.prices), 0, "should have seeded some prices")

	// Verify known models are present.
	assert.Contains(t, repo.prices, "claude-opus-4-20250514")
	assert.Contains(t, repo.prices, "gpt-4o")

	opus := repo.prices["claude-opus-4-20250514"]
	assert.Equal(t, "seed", opus.Source)
	assert.InDelta(t, 15e-6, *opus.PriceData.InputCostPerToken, 1e-12)
	assert.InDelta(t, 75e-6, *opus.PriceData.OutputCostPerToken, 1e-12)
}

func TestSeedDefaultPrices_IdempotentWhenRecordsExist(t *testing.T) {
	repo := newMockPriceRepo()

	// Pre-populate a record.
	_, _ = repo.Create(context.Background(), &model.ModelPrice{
		ModelName: "existing-model",
		Source:    "manual",
	})

	initialCount := len(repo.prices)

	// Seed should be a no-op.
	err := SeedDefaultPrices(context.Background(), repo)
	require.NoError(t, err)
	assert.Equal(t, initialCount, len(repo.prices), "no new records should be inserted")
}

func TestDefaultSeeds_HaveValidPriceData(t *testing.T) {
	seeds := defaultSeeds()
	assert.Greater(t, len(seeds), 0)

	for _, s := range seeds {
		assert.NotEmpty(t, s.ModelName, "model name must not be empty")
		assert.Equal(t, "seed", s.Source)
		assert.NotNil(t, s.PriceData.InputCostPerToken, "model %s: input cost required", s.ModelName)
		assert.NotNil(t, s.PriceData.OutputCostPerToken, "model %s: output cost required", s.ModelName)
		assert.NotNil(t, s.PriceData.Mode, "model %s: mode required", s.ModelName)
	}
}
