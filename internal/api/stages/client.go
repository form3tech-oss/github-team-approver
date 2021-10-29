package stages

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/require"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
)

type client struct {
	t *testing.T

	testAddress string
	secretToken []byte
	http        *http.Client
}

func newClient(t *testing.T, address string, token []byte) *client {
	return &client{
		t,
		address,
		token,
		&http.Client{
			Timeout: 5 * time.Second,
		},
	}
}


func (c *client) sendEventWithIncorrectSignature(e interface{}) *http.Response {
	payload, err := json.Marshal(e)
	require.NoError(c.t, err)

	req, err := http.NewRequest(http.MethodPost, c.endpoint(), bytes.NewReader(payload))
	require.NoError(c.t, err)

	fudgedPayload := append(payload, []byte{1}...)

	req.Header.Add("X-Hub-Signature", c.generateSignature(fudgedPayload))

	resp, err := c.http.Do(req)
	require.NoError(c.t, err)

	return resp
}

func (c *client) sendEvent(e interface{}, eventType string) *http.Response {
	payload, err := json.Marshal(e)
	require.NoError(c.t, err)

	req, err := http.NewRequest(http.MethodPost, c.endpoint(), bytes.NewReader(payload))
	require.NoError(c.t, err)

	req.Header.Add("X-Hub-Signature", c.generateSignature(payload))
	req.Header.Add("X-GitHub-Event", eventType)

	u, err := uuid.NewRandom()
	require.NoError(c.t, err, "uuid.NewRandom")
	req.Header.Add("X-GitHub-Delivery", u.String())

	resp, err := c.http.Do(req)
	require.NoError(c.t, err)

	return resp
}

func (c *client) generateSignature(payload []byte) string {
	// TODO once upgraded go-github, we should switch to using only X-Hub-Signature-256
	// #nosec G505 (CWE-327): Blocklisted import crypto/sha1: weak cryptographic primitive (Confidence: HIGH, Severity: MEDIUM)
	h := hmac.New(sha1.New, []byte(c.secretToken))
	_, err := h.Write(payload)
	require.NoError(c.t, err)

	mac := h.Sum(nil)

	return "sha1=" + hex.EncodeToString(mac)
}

func (c *client) endpoint() string {
	return fmt.Sprintf("%s/events", c.testAddress)
}
