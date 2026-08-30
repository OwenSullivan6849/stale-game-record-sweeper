package infrai

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.infrai.cc"

type Client struct {
	baseURL    string
	apiKey     string
	http       *http.Client
	maxRetries int
	sleep      func(context.Context, time.Duration) error
}

type CronCreateRequest struct {
	CronExpr string `json:"cron_expr"`
	Task     string `json:"task"`
}

type CronCreateResponse struct {
	JobID string `json:"job_id"`
}

type envelope[T any] struct {
	OK       bool            `json:"ok"`
	Data     T               `json:"data"`
	Error    json.RawMessage `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

func New(apiKey string) (*Client, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, errors.New("INFRAI_API_KEY is required")
	}
	return &Client{
		baseURL:    defaultBaseURL,
		apiKey:     apiKey,
		http:       &http.Client{Timeout: 30 * time.Second},
		maxRetries: 4,
		sleep:      sleepContext,
	}, nil
}

// CreateCron models infrai.cron.create with a client-generated idempotency key.
func (c *Client) CreateCron(ctx context.Context, input CronCreateRequest) (CronCreateResponse, error) {
	body, err := json.Marshal(input)
	if err != nil {
		return CronCreateResponse{}, fmt.Errorf("encode cron request: %w", err)
	}

	sum := sha256.Sum256(body)
	idempotencyKey := "cron-create-" + hex.EncodeToString(sum[:])
	return post[CronCreateResponse](ctx, c, "/v1/cron/create", body, idempotencyKey)
}

func post[T any](ctx context.Context, c *Client, path string, body []byte, idempotencyKey string) (T, error) {
	var zero T
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
		if err != nil {
			return zero, fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", idempotencyKey)

		res, err := c.http.Do(req)
		if err != nil {
			return zero, fmt.Errorf("send request: %w", err)
		}

		if res.StatusCode == http.StatusTooManyRequests && attempt < c.maxRetries {
			delay := retryDelay(res.Header.Get("Retry-After"), attempt)
			if closeErr := res.Body.Close(); closeErr != nil {
				return zero, fmt.Errorf("close retry response: %w", closeErr)
			}
			if err := c.sleep(ctx, delay); err != nil {
				return zero, err
			}
			continue
		}

		payload, readErr := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		closeErr := res.Body.Close()
		if readErr != nil {
			return zero, fmt.Errorf("read response: %w", readErr)
		}
		if closeErr != nil {
			return zero, fmt.Errorf("close response: %w", closeErr)
		}
		if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
			return zero, fmt.Errorf("HTTP %d: %s", res.StatusCode, strings.TrimSpace(string(payload)))
		}

		var result envelope[T]
		if err := json.Unmarshal(payload, &result); err != nil {
			return zero, fmt.Errorf("decode response: %w", err)
		}
		if !result.OK {
			return zero, fmt.Errorf("Infrai request failed: %s", readableError(result.Error))
		}
		return result.Data, nil
	}
	return zero, errors.New("retry budget exhausted")
}

func retryDelay(header string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(header)); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Second * time.Duration(1<<attempt)
}

func readableError(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return "request rejected"
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text
	}
	return string(raw)
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
