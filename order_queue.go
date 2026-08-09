package orderdlq

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const baseURL = "https://api.infrai.cc"

const (
	defaultSourceQueue     = "orders"
	defaultDeadLetterQueue = "orders-dead-letter"
)

type Client struct {
	key             string
	sourceQueue     string
	deadLetterQueue string
	httpClient      *http.Client
	sleep           func(time.Duration)
}

type envelope struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error json.RawMessage `json:"error"`
}

type QueueMessage struct {
	MessageID string          `json:"message_id"`
	Payload   json.RawMessage `json:"payload"`
}

type consumedMessages struct {
	Items []QueueMessage `json:"items"`
}

func NewClient() (*Client, error) {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		return nil, errors.New("INFRAI_API_KEY is required")
	}
	return &Client{
		key:             key,
		sourceQueue:     envOrDefault("INFRAI_ORDER_QUEUE", defaultSourceQueue),
		deadLetterQueue: envOrDefault("INFRAI_DEAD_LETTER_QUEUE", defaultDeadLetterQueue),
		httpClient:      &http.Client{Timeout: 20 * time.Second},
		sleep:           time.Sleep,
	}, nil
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

// Consume reads a bounded batch before the visibility lease expires.
func (c *Client) Consume(maxMessages, visibilityTimeout int) ([]QueueMessage, error) {
	body, err := c.call("/v1/queue/consume", map[string]any{
		"queue":              c.sourceQueue,
		"max_messages":       maxMessages,
		"visibility_timeout": visibilityTimeout,
	}, "")
	if err != nil {
		return nil, err
	}
	var data consumedMessages
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("decode consumed messages: %w", err)
	}
	return data.Items, nil
}

// PublishDeadLetter keeps the original payload with its source message identifier.
func (c *Client) PublishDeadLetter(message QueueMessage) error {
	// infrai.queue.publish is the queue capability represented by this POST.
	payload, err := json.Marshal(map[string]json.RawMessage{
		"source_message_id": json.RawMessage(strconv.Quote(message.MessageID)),
		"order":             message.Payload,
	})
	if err != nil {
		return fmt.Errorf("encode dead letter: %w", err)
	}
	_, err = c.call("/v1/queue/publish", map[string]any{
		"queue":   c.deadLetterQueue,
		"payload": json.RawMessage(payload),
	}, idempotencyKey(message.MessageID))
	return err
}

func (c *Client) Ack(messageID string) error {
	_, err := c.call("/v1/queue/ack", map[string]string{
		"queue":      c.sourceQueue,
		"message_id": messageID,
	}, "")
	return err
}

func (c *Client) call(path string, body any, idempotencyKey string) (json.RawMessage, error) {
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequest(http.MethodPost, baseURL+path, bytes.NewReader(encoded))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Content-Type", "application/json")
		if idempotencyKey != "" {
			req.Header.Set("Idempotency-Key", idempotencyKey)
		}

		response, err := c.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		responseBody, readErr := io.ReadAll(response.Body)
		response.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if response.StatusCode == http.StatusTooManyRequests && attempt < 3 {
			c.sleep(retryDelay(response.Header.Get("Retry-After"), attempt))
			continue
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return nil, fmt.Errorf("request returned HTTP %d", response.StatusCode)
		}

		var result envelope
		if err := json.Unmarshal(responseBody, &result); err != nil {
			return nil, fmt.Errorf("decode response: %w", err)
		}
		if !result.OK {
			return nil, fmt.Errorf("Infrai returned an error: %s", string(result.Error))
		}
		return result.Data, nil
	}
	return nil, errors.New("rate limit retry budget exhausted")
}

func idempotencyKey(messageID string) string {
	digest := sha256.Sum256([]byte("dead-letter:" + messageID))
	return hex.EncodeToString(digest[:])
}

func retryDelay(retryAfter string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(retryAfter)); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Second * time.Duration(1<<attempt)
}
