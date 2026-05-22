package api

import (
	"encoding/json"

	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/gin-gonic/gin"
)

func bindRawJSONMap(c *gin.Context) (map[string]json.RawMessage, bool) {
	var raw map[string]json.RawMessage
	if err := c.ShouldBindJSON(&raw); err != nil {
		writeAdminError(c, appErrors.NewInvalidRequest("请求体不是合法 JSON"))
		return nil, false
	}
	return raw, true
}

func parseBodyID(c *gin.Context, raw map[string]json.RawMessage) (int, bool) {
	value, ok := raw["id"]
	if !ok {
		writeAdminError(c, appErrors.NewInvalidRequest("id 为必填字段"))
		return 0, false
	}
	var id int
	if err := json.Unmarshal(value, &id); err != nil || id <= 0 {
		writeAdminError(c, appErrors.NewInvalidRequest("id 必须是正整数"))
		return 0, false
	}
	return id, true
}

func decodeOptionalString(raw map[string]json.RawMessage, key string) (string, bool, error) {
	value, ok := raw[key]
	if !ok {
		return "", false, nil
	}
	var result string
	if err := json.Unmarshal(value, &result); err != nil {
		return "", true, err
	}
	return result, true, nil
}

func decodeOptionalNullableString(raw map[string]json.RawMessage, key string) (*string, bool, error) {
	value, ok := raw[key]
	if !ok {
		return nil, false, nil
	}
	if string(value) == "null" {
		return nil, true, nil
	}
	var result string
	if err := json.Unmarshal(value, &result); err != nil {
		return nil, true, err
	}
	return &result, true, nil
}

func decodeOptionalBool(raw map[string]json.RawMessage, key string) (bool, bool, error) {
	value, ok := raw[key]
	if !ok {
		return false, false, nil
	}
	var result bool
	if err := json.Unmarshal(value, &result); err != nil {
		return false, true, err
	}
	return result, true, nil
}

func decodeOptionalInt(raw map[string]json.RawMessage, key string) (int, bool, error) {
	value, ok := raw[key]
	if !ok {
		return 0, false, nil
	}
	if string(value) == "null" {
		return 0, true, nil
	}
	var result int
	if err := json.Unmarshal(value, &result); err != nil {
		return 0, true, err
	}
	return result, true, nil
}

func decodeOptionalIntPointer(raw map[string]json.RawMessage, key string) (*int, bool, error) {
	value, ok := raw[key]
	if !ok {
		return nil, false, nil
	}
	if string(value) == "null" {
		return nil, true, nil
	}
	var result int
	if err := json.Unmarshal(value, &result); err != nil {
		return nil, true, err
	}
	return &result, true, nil
}

func decodeOptionalStringSlice(raw map[string]json.RawMessage, key string) ([]string, bool, error) {
	value, ok := raw[key]
	if !ok {
		return nil, false, nil
	}
	if string(value) == "null" {
		return nil, true, nil
	}
	var result []string
	if err := json.Unmarshal(value, &result); err != nil {
		return nil, true, err
	}
	return result, true, nil
}

func decodeOptionalIntSlice(raw map[string]json.RawMessage, key string) ([]int, bool, error) {
	value, ok := raw[key]
	if !ok {
		return nil, false, nil
	}
	if string(value) == "null" {
		return nil, true, nil
	}
	var result []int
	if err := json.Unmarshal(value, &result); err != nil {
		return nil, true, err
	}
	return result, true, nil
}

func decodeOptionalAny(raw map[string]json.RawMessage, key string) (any, bool, error) {
	value, ok := raw[key]
	if !ok {
		return nil, false, nil
	}
	var result any
	if err := json.Unmarshal(value, &result); err != nil {
		return nil, true, err
	}
	return result, true, nil
}

func decodeOptionalMap(raw map[string]json.RawMessage, key string) (map[string]any, bool, error) {
	value, ok := raw[key]
	if !ok {
		return nil, false, nil
	}
	if string(value) == "null" {
		return nil, true, nil
	}
	var result map[string]any
	if err := json.Unmarshal(value, &result); err != nil {
		return nil, true, err
	}
	return result, true, nil
}

func decodeOptionalOperations(raw map[string]json.RawMessage, key string) ([]map[string]any, bool, error) {
	value, ok := raw[key]
	if !ok {
		return nil, false, nil
	}
	if string(value) == "null" {
		return nil, true, nil
	}
	var result []map[string]any
	if err := json.Unmarshal(value, &result); err != nil {
		return nil, true, err
	}
	return result, true, nil
}
