package guard

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/ding113/claude-code-hub/internal/model"
)

// RequestFilterRepo provides access to request filter rules.
type RequestFilterRepo interface {
	GetActive(ctx context.Context) ([]model.RequestFilter, error)
}

// RequestFilterGuard applies request filter rules to modify headers/body.
type RequestFilterGuard struct {
	repo RequestFilterRepo
}

// NewRequestFilterGuard creates a new RequestFilterGuard.
func NewRequestFilterGuard(repo RequestFilterRepo) *RequestFilterGuard {
	return &RequestFilterGuard{repo: repo}
}

// Name returns the guard name.
func (g *RequestFilterGuard) Name() string {
	return "RequestFilterGuard"
}

// Check applies request filter rules. Never returns error (fail-open, mutates in-place).
func (g *RequestFilterGuard) Check(ctx context.Context, req *Request) error {
	filters, err := g.repo.GetActive(ctx)
	if err != nil {
		// Fail-open
		return nil
	}

	for _, filter := range filters {
		if !filter.IsActive() {
			continue
		}
		// Only apply guard-phase filters
		if filter.ExecutionPhase != "guard" {
			continue
		}
		// Only apply global-scope filters at this stage
		if !filter.IsGlobal() {
			continue
		}

		g.applyFilter(req, &filter)
	}

	return nil
}

// applyFilter applies a single filter rule to the request.
func (g *RequestFilterGuard) applyFilter(req *Request, filter *model.RequestFilter) {
	scope := strings.ToLower(filter.Scope)
	action := strings.ToLower(filter.Action)

	switch scope {
	case "header":
		g.applyHeaderFilter(req, action, filter.Target, filter.Replacement)
	case "body":
		g.applyBodyFilter(req, action, filter.Target, filter.Replacement)
	}
}

// applyHeaderFilter applies a header-scoped filter.
func (g *RequestFilterGuard) applyHeaderFilter(req *Request, action, target string, replacement any) {
	if req.Headers == nil {
		req.Headers = make(map[string]string)
	}

	switch action {
	case "remove":
		delete(req.Headers, target)
		// Also delete case-insensitive match
		for k := range req.Headers {
			if strings.EqualFold(k, target) {
				delete(req.Headers, k)
			}
		}
	case "set":
		if val, ok := replacement.(string); ok {
			req.Headers[target] = val
		}
	}
}

// applyBodyFilter applies a body-scoped filter.
func (g *RequestFilterGuard) applyBodyFilter(req *Request, action, target string, replacement any) {
	if len(req.RawBody) == 0 {
		return
	}

	switch action {
	case "json_path":
		g.applyJSONPathFilter(req, target, replacement)
	case "text_replace":
		if replStr, ok := replacement.(string); ok {
			body := string(req.RawBody)
			body = strings.ReplaceAll(body, target, replStr)
			req.RawBody = []byte(body)
		}
	}
}

// applyJSONPathFilter applies a simple JSON path set/remove on the body.
// Supports only top-level keys for safety.
func (g *RequestFilterGuard) applyJSONPathFilter(req *Request, target string, replacement any) {
	var body map[string]interface{}
	if err := json.Unmarshal(req.RawBody, &body); err != nil {
		return
	}

	if replacement == nil {
		// Remove the key
		delete(body, target)
	} else {
		// Set the key
		body[target] = replacement
	}

	newBody, err := json.Marshal(body)
	if err != nil {
		return
	}
	req.RawBody = newBody
}
