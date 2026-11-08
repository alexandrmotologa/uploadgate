package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/alexandrmotologa/uploadgate/internal/config"
	"github.com/alexandrmotologa/uploadgate/internal/db"
)

// Dispatcher manages reliable outbound webhook deliveries with HMAC signatures and retries.
type Dispatcher struct {
	repo       *db.Repository
	cfg        *config.WebhooksConfig
	client     *http.Client
	queue      chan *deliveryTask
	wg         sync.WaitGroup
	ctx        context.Context
	cancelFunc context.CancelFunc
}

type deliveryTask struct {
	UploadID string
	Event    string
	Payload  map[string]interface{}
}

// NewDispatcher creates and starts the webhook worker pool.
func NewDispatcher(repo *db.Repository, cfg *config.WebhooksConfig) *Dispatcher {
	ctx, cancel := context.WithCancel(context.Background())
	d := &Dispatcher{
		repo: repo,
		cfg:  cfg,
		client: &http.Client{
			Timeout: time.Duration(cfg.TimeoutSeconds) * time.Second,
		},
		queue:      make(chan *deliveryTask, 1000),
		ctx:        ctx,
		cancelFunc: cancel,
	}

	// Start 2 background dispatch workers
	for i := 0; i < 2; i++ {
		d.wg.Add(1)
		go d.worker()
	}

	return d
}

// Stop shuts down the webhook dispatcher gracefully.
func (d *Dispatcher) Stop() {
	d.cancelFunc()
	close(d.queue)
	d.wg.Wait()
}

// Dispatch queues an event for outbound transmission.
func (d *Dispatcher) Dispatch(uploadID, event string, payload map[string]interface{}) {
	if d.cfg.Endpoint == "" {
		return // No webhook endpoint configured
	}

	task := &deliveryTask{
		UploadID: uploadID,
		Event:    event,
		Payload:  payload,
	}

	select {
	case d.queue <- task:
	default:
		log.Printf("[Webhook] Queue full, dropping event %s for upload %s", event, uploadID)
	}
}

func (d *Dispatcher) worker() {
	defer d.wg.Done()

	for {
		select {
		case <-d.ctx.Done():
			return
		case task, ok := <-d.queue:
			if !ok {
				return
			}
			d.sendWithRetry(task)
		}
	}
}

func (d *Dispatcher) sendWithRetry(task *deliveryTask) {
	payloadBytes, err := json.Marshal(task.Payload)
	if err != nil {
		log.Printf("[Webhook] Failed to serialize payload: %v", err)
		return
	}

	maxRetries := d.cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 1
	}

	var lastStatusCode int
	var lastRespBody string
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		statusCode, respBody, err := d.sendRequest(task.Event, payloadBytes)
		lastStatusCode = statusCode
		lastRespBody = respBody
		lastErr = err

		if err == nil && statusCode >= 200 && statusCode < 300 {
			// Successful delivery
			_ = d.repo.RecordWebhookDelivery(context.Background(), &db.WebhookDelivery{
				UploadID:     task.UploadID,
				Event:        task.Event,
				Endpoint:     d.cfg.Endpoint,
				Payload:      string(payloadBytes),
				StatusCode:   statusCode,
				Attempt:      attempt,
				ResponseBody: respBody,
				DeliveredAt:  time.Now().UTC(),
			})
			return
		}

		if attempt < maxRetries {
			// Exponential backoff: 200ms * 2^(attempt-1)
			backoff := time.Duration(200*(1<<(attempt-1))) * time.Millisecond
			select {
			case <-d.ctx.Done():
				return
			case <-time.After(backoff):
			}
		}
	}

	// Record failed delivery after max retries
	errMsg := ""
	if lastErr != nil {
		errMsg = lastErr.Error()
	}
	_ = d.repo.RecordWebhookDelivery(context.Background(), &db.WebhookDelivery{
		UploadID:     task.UploadID,
		Event:        task.Event,
		Endpoint:     d.cfg.Endpoint,
		Payload:      string(payloadBytes),
		StatusCode:   lastStatusCode,
		Attempt:      maxRetries,
		ResponseBody: lastRespBody,
		ErrorMessage: errMsg,
		DeliveredAt:  time.Now().UTC(),
	})
}

func (d *Dispatcher) sendRequest(event string, payloadBytes []byte) (int, string, error) {
	req, err := http.NewRequestWithContext(d.ctx, "POST", d.cfg.Endpoint, bytes.NewReader(payloadBytes))
	if err != nil {
		return 0, "", err
	}

	timestamp := time.Now().Unix()
	signature := GenerateSignature(d.cfg.Secret, timestamp, payloadBytes)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "UploadGate-Daemon/1.0")
	req.Header.Set("X-UploadGate-Event", event)
	req.Header.Set("X-UploadGate-Signature", fmt.Sprintf("t=%d,v1=%s", timestamp, signature))

	resp, err := d.client.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return resp.StatusCode, string(bodyBytes), nil
}

// GenerateSignature computes the HMAC-SHA256 signature for a webhook payload.
func GenerateSignature(secret string, timestamp int64, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	signedData := fmt.Sprintf("%d.%s", timestamp, string(payload))
	mac.Write([]byte(signedData))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifySignature validates an incoming webhook signature.
func VerifySignature(secret string, headerValue string, payload []byte) bool {
	parts := strings.Split(headerValue, ",")
	var tStr, sigStr string
	for _, p := range parts {
		kv := strings.SplitN(strings.TrimSpace(p), "=", 2)
		if len(kv) == 2 {
			if kv[0] == "t" {
				tStr = kv[1]
			} else if kv[0] == "v1" {
				sigStr = kv[1]
			}
		}
	}

	if tStr == "" || sigStr == "" {
		return false
	}

	var timestamp int64
	_, err := fmt.Sscanf(tStr, "%d", &timestamp)
	if err != nil {
		return false
	}

	expected := GenerateSignature(secret, timestamp, payload)
	return hmac.Equal([]byte(expected), []byte(sigStr))
}
