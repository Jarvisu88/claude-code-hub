package endpointprobe

import (
	"sort"
	"sync"
	"time"
)

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

type Store struct {
	mu       sync.Mutex
	nextID   int
	statuses map[int]Status
	logs     map[int][]Log
}

var defaultStore = &Store{
	nextID:   1,
	statuses: map[int]Status{},
	logs:     map[int][]Log{},
}

func ResetForTest() {
	defaultStore = &Store{
		nextID:   1,
		statuses: map[int]Status{},
		logs:     map[int][]Log{},
	}
}

func Record(endpointID int, source string, ok bool, statusCode, latencyMs *int, errorType, errorMessage *string, createdAt time.Time) {
	defaultStore.Record(endpointID, source, ok, statusCode, latencyMs, errorType, errorMessage, createdAt)
}

func GetStatus(endpointID int) Status {
	return defaultStore.GetStatus(endpointID)
}

func ListLogs(endpointID, limit, offset int) []Log {
	return defaultStore.ListLogs(endpointID, limit, offset)
}

func (s *Store) Record(endpointID int, source string, ok bool, statusCode, latencyMs *int, errorType, errorMessage *string, createdAt time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	log := Log{
		ID:           s.nextID,
		EndpointID:   endpointID,
		Source:       source,
		OK:           ok,
		StatusCode:   statusCode,
		LatencyMs:    latencyMs,
		ErrorType:    errorType,
		ErrorMessage: errorMessage,
		CreatedAt:    createdAt,
	}
	s.nextID++
	s.logs[endpointID] = append([]Log{log}, s.logs[endpointID]...)
	s.statuses[endpointID] = Status{
		LastProbedAt:          &createdAt,
		LastProbeOk:           &ok,
		LastProbeStatusCode:   statusCode,
		LastProbeLatencyMs:    latencyMs,
		LastProbeErrorType:    errorType,
		LastProbeErrorMessage: errorMessage,
	}
}

func (s *Store) GetStatus(endpointID int) Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.statuses[endpointID]
}

func (s *Store) ListLogs(endpointID, limit, offset int) []Log {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := append([]Log(nil), s.logs[endpointID]...)
	sort.Slice(items, func(i, j int) bool { return items[i].ID > items[j].ID })
	if offset < 0 {
		offset = 0
	}
	if offset >= len(items) {
		return []Log{}
	}
	items = items[offset:]
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items
}
