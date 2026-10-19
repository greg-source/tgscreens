package telegram

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewWebhook_rejectsInvalidInput(t *testing.T) {
	tests := map[string]string{
		"non-https scheme": "http://example.com:8443/secret-path",
		"empty path":       "https://example.com:8443",
		"root path":        "https://example.com:8443/",
		"disallowed port":  "https://example.com:9000/secret-path",
		"empty URL":        "",
	}

	for name, publicURL := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := NewWebhook(&Client{}, publicURL, "cert.pem", "key.pem")

			require.Error(t, err)
		})
	}
}

func TestNewWebhook_derivesAddrFromURLPort(t *testing.T) {
	w, err := NewWebhook(&Client{}, "https://example.com:8443/secret-path", "cert.pem", "key.pem")

	require.NoError(t, err)
	assert.Equal(t, ":8443", w.addr)
	assert.Equal(t, "/secret-path", w.path)
}

func TestNewWebhook_defaultsToPort443WhenURLOmitsPort(t *testing.T) {
	w, err := NewWebhook(&Client{}, "https://example.com/secret-path", "cert.pem", "key.pem")

	require.NoError(t, err)
	assert.Equal(t, ":443", w.addr)
}

func TestWebhook_handlesPostedUpdate(t *testing.T) {
	server := newTelegramTestServer(t)
	client := server.client(t)
	out := make(chan Update, 1)
	app := newTestWebhookApp(&Webhook{client: client, path: "/secret-path"}, out)

	resp := postWebhookJSON(t, app, tgbotapi.Update{
		Message: &tgbotapi.Message{
			MessageID: 5,
			From:      &tgbotapi.User{UserName: "greg"},
			Chat:      &tgbotapi.Chat{ID: 1},
			Text:      "hi",
		},
	})

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, Update{Username: "greg", ChatID: 1, MessageID: 5, Text: "hi"}, receiveClientUpdate(t, out))
}

func TestWebhook_acknowledgesCallback(t *testing.T) {
	server := newTelegramTestServer(t)
	client := server.client(t)
	out := make(chan Update, 1)
	app := newTestWebhookApp(&Webhook{client: client, path: "/secret-path"}, out)

	resp := postWebhookJSON(t, app, tgbotapi.Update{
		CallbackQuery: &tgbotapi.CallbackQuery{
			ID:   "callback-id",
			From: &tgbotapi.User{UserName: "greg"},
			Message: &tgbotapi.Message{
				MessageID: 42,
				Chat:      &tgbotapi.Chat{ID: 1},
			},
			Data: "menu",
		},
	})

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	request := server.request(t, "answerCallbackQuery")
	assert.Equal(t, "callback-id", request.form.Get("callback_query_id"))
	assert.Equal(t, Update{Username: "greg", ChatID: 1, MessageID: 42, Text: "menu", IsCallback: true}, receiveClientUpdate(t, out))
}

func TestWebhook_skipsUnconvertibleUpdate(t *testing.T) {
	out := make(chan Update, 1)
	app := newTestWebhookApp(&Webhook{path: "/secret-path"}, out)

	resp := postWebhookJSON(t, app, tgbotapi.Update{})

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Empty(t, out)
}

func TestWebhook_wrongPathNotFound(t *testing.T) {
	out := make(chan Update, 1)
	app := newTestWebhookApp(&Webhook{path: "/secret-path"}, out)

	resp, err := app.Test(httptest.NewRequest(http.MethodPost, "/wrong-path", http.NoBody))

	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestWebhook_malformedBodyBadRequest(t *testing.T) {
	out := make(chan Update, 1)
	app := newTestWebhookApp(&Webhook{path: "/secret-path"}, out)

	req := httptest.NewRequest(http.MethodPost, "/secret-path", strings.NewReader("{not json"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestWebhook_registerSendsURLAndCertificate(t *testing.T) {
	certFile, keyFile := writeTestCertificate(t)
	certPEM, err := os.ReadFile(certFile)
	require.NoError(t, err)

	server := newTelegramTestServer(t)
	client := server.client(t)

	const publicURL = "https://example.com:8443/secret-path"
	w, err := NewWebhook(client, publicURL, certFile, keyFile)
	require.NoError(t, err)

	require.NoError(t, w.Register())

	request := server.request(t, "setWebhook")
	assert.Equal(t, publicURL, request.form.Get("url"))
	assert.Equal(t, "1", request.form.Get("max_connections"))
	assert.Equal(t, certPEM, request.files["certificate"])
}

func TestWebhook_registerReturnsTelegramError(t *testing.T) {
	certFile, keyFile := writeTestCertificate(t)
	server := newTelegramTestServer(t)
	client := server.client(t)
	server.fail("setWebhook", "telegram down")

	w, err := NewWebhook(client, "https://example.com:8443/secret-path", certFile, keyFile)
	require.NoError(t, err)

	err = w.Register()

	require.Error(t, err)
	assert.ErrorContains(t, err, "telegram down")
}

func TestWebhook_listenServesAndDeregistersOnCancel(t *testing.T) {
	certFile, keyFile := writeTestCertificate(t)

	server := newTelegramTestServer(t)
	client := server.client(t)

	w, err := NewWebhook(client, "https://example.com:8443/secret-path", certFile, keyFile)
	require.NoError(t, err)
	addr := freeLocalAddr(t) // override the derived :8443 with a free port, collision-safe for tests
	w.addr = addr

	ctx, cancel := context.WithCancel(context.Background())
	out := make(chan Update)
	done := make(chan error, 1)
	go func() { done <- w.Listen(ctx, out) }()

	// confirm the port accepts before cancelling; Shutdown no-ops before Serve starts.
	require.Eventually(t, func() bool {
		conn, dialErr := net.DialTimeout("tcp", addr, 50*time.Millisecond)
		if dialErr != nil {
			return false
		}
		_ = conn.Close()
		return true
	}, 2*time.Second, 10*time.Millisecond, "webhook server did not become ready")
	cancel()

	server.request(t, "deleteWebhook")
	select {
	case err := <-done:
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Listen to return")
	}
	assertClientUpdatesClosed(t, out)
}

// freeLocalAddr reserves a free local TCP port for a server to bind moments later.
func freeLocalAddr(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()
	require.NoError(t, ln.Close())
	return addr
}

// newTestWebhookApp registers w's route on a bare Fiber app, without going
// through Listen — the same setup Listen itself does, minus the network.
func newTestWebhookApp(w *Webhook, out chan<- Update) *fiber.App {
	app := fiber.New()
	app.Post(w.path, w.handle(context.Background(), out))
	return app
}

func postWebhookJSON(t *testing.T, app *fiber.App, update tgbotapi.Update) *http.Response {
	t.Helper()

	body, err := json.Marshal(update)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/secret-path", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	return resp
}

// writeTestCertificate generates a throwaway self-signed cert/key pair so
// app.Listen has something valid to load.
func writeTestCertificate(t *testing.T) (certFile, keyFile string) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	require.NoError(t, err)

	dir := t.TempDir()
	certFile = filepath.Join(dir, "cert.pem")
	keyFile = filepath.Join(dir, "key.pem")

	require.NoError(t, os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600))
	require.NoError(t, os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}), 0o600))

	return certFile, keyFile
}
