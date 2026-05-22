package price

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/ding113/claude-code-hub/internal/pkg/logger"
	"github.com/ding113/claude-code-hub/internal/repository"
)

const (
	// LiteLLMPriceURL is the canonical source for model pricing data.
	LiteLLMPriceURL = "https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json"

	// defaultSyncTimeout prevents the sync from hanging indefinitely.
	defaultSyncTimeout = 60 * time.Second
)

// PriceRepo is the subset of repository.ModelPriceRepository used by the syncer.
type PriceRepo interface {
	Create(ctx context.Context, price *model.ModelPrice) (*model.ModelPrice, error)
	GetLatestByModelName(ctx context.Context, modelName string) (*model.ModelPrice, error)
	HasAnyRecords(ctx context.Context) (bool, error)
}

// Syncer fetches model prices from external sources and upserts them locally.
type Syncer struct {
	priceRepo  PriceRepo
	httpClient *http.Client
	sourceURL  string
}

// NewSyncer creates a Syncer with the given repository.
// An optional custom HTTP client can be provided for testing; pass nil for the default.
func NewSyncer(repo PriceRepo, client *http.Client) *Syncer {
	if client == nil {
		client = &http.Client{Timeout: defaultSyncTimeout}
	}
	return &Syncer{
		priceRepo:  repo,
		httpClient: client,
		sourceURL:  LiteLLMPriceURL,
	}
}

// WithSourceURL overrides the default LiteLLM URL (useful in tests).
func (s *Syncer) WithSourceURL(url string) *Syncer {
	s.sourceURL = url
	return s
}

// SyncFromLiteLLM fetches the LiteLLM price table, parses it, and upserts
// each model into the model_prices table with source="litellm".
// It returns the number of models successfully synced.
func (s *Syncer) SyncFromLiteLLM(ctx context.Context) (int, error) {
	priceMap, err := s.fetchPriceTable(ctx)
	if err != nil {
		return 0, fmt.Errorf("fetch price table: %w", err)
	}

	synced := 0
	for modelName, priceData := range priceMap {
		// Skip the special "sample_spec" key that LiteLLM uses for documentation.
		if modelName == "sample_spec" {
			continue
		}

		mp := &model.ModelPrice{
			ModelName: modelName,
			PriceData: priceData,
			Source:    "litellm",
		}

		if _, err := s.priceRepo.Create(ctx, mp); err != nil {
			logger.Warn().Err(err).Str("model", modelName).Msg("price sync: failed to upsert model")
			continue
		}
		synced++
	}

	logger.Info().Int("synced", synced).Int("total", len(priceMap)).Msg("price sync: LiteLLM sync complete")
	return synced, nil
}

// fetchPriceTable downloads and decodes the JSON price table.
func (s *Syncer) fetchPriceTable(ctx context.Context) (map[string]model.PriceData, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.sourceURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "cch-price-sync/1.0")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d from %s", resp.StatusCode, s.sourceURL)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	var priceMap map[string]model.PriceData
	if err := json.Unmarshal(body, &priceMap); err != nil {
		return nil, fmt.Errorf("decode JSON: %w", err)
	}

	return priceMap, nil
}

// EnsurePriceData is a convenience helper that seeds default prices when the
// table is empty and then performs a full LiteLLM sync.
func EnsurePriceData(ctx context.Context, repo repository.ModelPriceRepository) error {
	has, err := repo.HasAnyRecords(ctx)
	if err != nil {
		return fmt.Errorf("check existing prices: %w", err)
	}

	if !has {
		if err := SeedDefaultPrices(ctx, repo); err != nil {
			logger.Warn().Err(err).Msg("price sync: seed default prices failed (non-fatal)")
		}
	}

	syncer := NewSyncer(repo, nil)
	count, err := syncer.SyncFromLiteLLM(ctx)
	if err != nil {
		return fmt.Errorf("litellm sync: %w", err)
	}
	logger.Info().Int("count", count).Msg("price sync: initial sync done")
	return nil
}
