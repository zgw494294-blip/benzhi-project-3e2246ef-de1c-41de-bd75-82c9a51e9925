package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type selfcheckClient struct {
	baseURL string
	client  *http.Client
}

type responseEnvelope struct {
	Data  json.RawMessage `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (c selfcheckClient) command(ctx context.Context, caseID string, body any, expectedStatus int, target any) error {
	return c.request(ctx, http.MethodPost, "/api/v1/cases/"+caseID+"/commands", body, expectedStatus, target)
}

func (c selfcheckClient) get(ctx context.Context, path string, expectedStatus int, target any) error {
	return c.request(ctx, http.MethodGet, path, nil, expectedStatus, target)
}

func (c selfcheckClient) request(ctx context.Context, method, path string, body any, expectedStatus int, target any) error {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	request.Header.Set("X-Request-ID", "selfcheck")
	response, err := c.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != expectedStatus {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("%s %s 返回 %d，期望 %d：%s", method, path, response.StatusCode, expectedStatus, message)
	}
	if target == nil {
		return nil
	}
	var envelope responseEnvelope
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		return err
	}
	if envelope.Error != nil {
		return fmt.Errorf("API 错误 %s：%s", envelope.Error.Code, envelope.Error.Message)
	}
	return json.Unmarshal(envelope.Data, target)
}
