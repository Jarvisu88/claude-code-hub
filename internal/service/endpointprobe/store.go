package endpointprobe

import (
	"context"
	"encoding/json"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/ding113/claude-code-hub/internal/database"
)

const defaultTTL = 24 * time.Hour

type Log struct {
	ID           int       `json:"id"`
	EndpointID   int       `json:"endpointId"`
	Source       string    `json:"source"`
	OK           bool      `json:"ok"`
	StatusCode   *int      `json:"statusCode,omitempty"`
	LatencyMs    *int      `json:"latencyMs,omitempty"`
	ErrorType    *string   `json:"errorType,omitempty"`
	ErrorMessage *string   `json:"errorMessage,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
}

type Status struct {
	LastProbedAt          *time.Time `json:"lastProbedAt,omitempty"`
	LastProbeOk           *bool      `json:"lastProbeOk,omitempty"`
	LastProbeStatusCode   *int       `json:"lastProbeStatusCode,omitempty"`
	LastProbeLatencyMs    *int       `json:"lastProbeLatencyMs,omitempty"`
	LastProbeErrorType    *string    `json:"lastProbeErrorType,omitempty"`
	LastProbeErrorMessage *string    `json:"lastProbeErrorMessage,omitempty"`
}

type backend interface {
	record(ctx context.Context, log Log, status Status) error
	getStatus(ctx context.Context, endpointID int) (Status, error)
	listLogs(ctx context.Context, endpointID, limit, offset int) ([]Log, error)
}

type memoryBackend struct {
	mu       sync.Mutex
	nextID   int
	statuses map[int]Status
	logs     map[int][]Log
}

type redisBackend struct {
	client *database.RedisClient
	ttl    time.Duration
}

var current backend = newMemoryBackend()

func Configure(client *database.RedisClient, ttl time.Duration) {
	if client == nil {
		current = newMemoryBackend()
		return
	}
	if ttl <= 0 {
		ttl = defaultTTL
	}
	current = &redisBackend{client: client, ttl: ttl}
}

func ResetForTest() {
	current = newMemoryBackend()
}

func Record(endpointID int, source string, ok bool, statusCode, latencyMs *int, errorType, errorMessage *string, createdAt time.Time) {
	if endpointID <= 0 {
		return
	}
	status := Status{
		LastProbedAt:          &createdAt,
		LastProbeOk:           &ok,
		LastProbeStatusCode:   statusCode,
		LastProbeLatencyMs:    latencyMs,
		LastProbeErrorType:    errorType,
		LastProbeErrorMessage: errorMessage,
	}
	log := Log{
		EndpointID:   endpointID,
		Source:       source,
		OK:           ok,
		StatusCode:   statusCode,
		LatencyMs:    latencyMs,
		ErrorType:    errorType,
		ErrorMessage: errorMessage,
		CreatedAt:    createdAt,
	}
	_ = current.record(context.Background(), log, status)
}

func GetStatus(endpointID int) Status {
	status, err := current.getStatus(context.Background(), endpointID)
	if err != nil {
		return Status{}
	}
	return status
}

func ListLogs(endpointID, limit, offset int) []Log {
	logs, err := current.listLogs(context.Background(), endpointID, limit, offset)
	if err != nil {
		return []Log{}
	}
	return logs
}

func newMemoryBackend() *memoryBackend {
	return &memoryBackend{
		nextID:   1,
		statuses: map[int]Status{},
		logs:     map[int][]Log{},
	}
}

func (s *memoryBackend) record(_ context.Context, log Log, status Status) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	log.ID = s.nextID
	s.nextID++
	s.logs[log.EndpointID] = append([]Log{log}, s.logs[log.EndpointID]...)
	s.statuses[log.EndpointID] = status
	return nil
}

func (s *memoryBackend) getStatus(_ context.Context, endpointID int) (Status, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.statuses[endpointID], nil
}

func (s *memoryBackend) listLogs(_ context.Context, endpointID, limit, offset int) ([]Log, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := append([]Log(nil), s.logs[endpointID]...)
	sort.Slice(items, func(i, j int) bool { return items[i].ID > items[j].ID })
	if offset < 0 {
		offset = 0
	}
	if offset >= len(items) {
		return []Log{}, nil
	}
	items = items[offset:]
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (r *redisBackend) record(ctx context.Context, log Log, status Status) error {
	if r == nil || r.client == nil {
		return nil
	}
	nextID, err := r.client.Incr(ctx, endpointProbeCounterKey(log.EndpointID)).Result()
	if err != nil {
		return err
	}
	log.ID = int(nextID)
	payload, err := json.Marshal(log)
	if err != nil {
		return err
	}
	statusPayload, err := json.Marshal(status)
	if err != nil {
		return err
	}
	pipe := r.client.TxPipeline()
	pipe.Set(ctx, endpointProbeStatusKey(log.EndpointID), statusPayload, r.ttl)
	pipe.LPush(ctx, endpointProbeLogsKey(log.EndpointID), payload)
	pipe.LTrim(ctx, endpointProbeLogsKey(log.EndpointID), 0, 999)
	pipe.Expire(ctx, endpointProbeLogsKey(log.EndpointID), r.ttl)
	pipe.Expire(ctx, endpointProbeCounterKey(log.EndpointID), r.ttl)
	_, err = pipe.Exec(ctx)
	return err
}

func (r *redisBackend) getStatus(ctx context.Context, endpointID int) (Status, error) {
	if r == nil || r.client == nil {
		return Status{}, nil
	}
	raw, err := r.client.Get(ctx, endpointProbeStatusKey(endpointID)).Result()
	if err != nil || raw == "" {
		return Status{}, nil
	}
	var status Status
	if err := json.Unmarshal([]byte(raw), &status); err != nil {
		return Status{}, nil
	}
	return status, nil
}

func (r *redisBackend) listLogs(ctx context.Context, endpointID, limit, offset int) ([]Log, error) {
	if r == nil || r.client == nil {
		return []Log{}, nil
	}
	if offset < 0 {
		offset = 0
	}
	end := int64(-1)
	if limit > 0 {
		end = int64(offset + limit - 1)
	}
	values, err := r.client.LRange(ctx, endpointProbeLogsKey(endpointID), int64(offset), end).Result()
	if err != nil {
		return nil, err
	}
	logs := make([]Log, 0, len(values))
	for _, raw := range values {
		var log Log
		if err := json.Unmarshal([]byte(raw), &log); err == nil {
			logs = append(logs, log)
		}
	}
	return logs, nil
}

func endpointProbeStatusKey(endpointID int) string {
	return "cch:endpoint-probe:status:" + strconv.Itoa(endpointID)
}

func endpointProbeLogsKey(endpointID int) string {
	return "cch:endpoint-probe:logs:" + strconv.Itoa(endpointID)
}

func endpointProbeCounterKey(endpointID int) string {
	return "cch:endpoint-probe:counter:" + strconv.Itoa(endpointID)
}
