package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path"
	"strings"
	"sync"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/require"
)

const testTelegramToken = "test-token"

type telegramTestRequest struct {
	method string
	form   url.Values
	files  map[string][]byte
}

type telegramTestResponse struct {
	body []byte
	wait <-chan struct{}
}

type telegramTestServer struct {
	t *testing.T

	server *httptest.Server

	mu        sync.Mutex
	responses map[string][]telegramTestResponse
	requests  chan telegramTestRequest
	pending   []telegramTestRequest

	pollRelease     chan struct{}
	releasePollOnce sync.Once
}

func newTelegramTestServer(t *testing.T) *telegramTestServer {
	t.Helper()

	s := &telegramTestServer{
		t:           t,
		responses:   make(map[string][]telegramTestResponse),
		requests:    make(chan telegramTestRequest, 128),
		pollRelease: make(chan struct{}),
	}
	s.server = httptest.NewServer(http.HandlerFunc(s.handle))
	t.Cleanup(func() {
		s.releasePoll()
		s.server.Close()
	})
	return s
}

func (s *telegramTestServer) client(t *testing.T) *Client {
	t.Helper()

	bot, err := tgbotapi.NewBotAPIWithAPIEndpoint(
		testTelegramToken,
		s.server.URL+"/bot%s/%s",
	)
	require.NoError(t, err)
	s.request(t, "getMe")

	return &Client{bot: bot}
}

type telegramTestListener struct {
	cancel context.CancelFunc
	done   <-chan struct{}
	out    <-chan Update
	server *telegramTestServer

	once sync.Once
	err  error
}

func (s *telegramTestServer) listen(t *testing.T, client *Client) *telegramTestListener {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	out := make(chan Update)
	done := make(chan struct{})
	listener := &telegramTestListener{
		cancel: cancel,
		done:   done,
		out:    out,
		server: s,
	}
	poller := NewPoller(client)
	go func() {
		listener.err = poller.Poll(ctx, out)
		close(done)
	}()
	t.Cleanup(func() {
		if err := listener.stop(t); !errors.Is(err, context.Canceled) {
			t.Errorf("client listener returned %v", err)
		}
	})
	return listener
}

func (l *telegramTestListener) stop(t *testing.T) error {
	t.Helper()

	l.once.Do(func() {
		l.cancel()
		l.server.releasePoll()
		select {
		case <-l.done:
		case <-time.After(time.Second):
			t.Error("timed out waiting for client listener")
		}
	})
	return l.err
}

func (s *telegramTestServer) respond(method string, result any) {
	s.enqueue(method, telegramOK(result))
}

func (s *telegramTestServer) fail(method, description string) {
	s.enqueue(method, telegramFailure(description))
}

func (s *telegramTestServer) updates(updates ...tgbotapi.Update) {
	s.respond("getUpdates", updates)
}

func (s *telegramTestServer) releasePoll() {
	s.releasePollOnce.Do(func() { close(s.pollRelease) })
}

func (s *telegramTestServer) enqueue(method string, response telegramTestResponse) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.responses[method] = append(s.responses[method], response)
}

func (s *telegramTestServer) handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method must be POST", http.StatusMethodNotAllowed)
		return
	}

	request := telegramTestRequest{method: path.Base(r.URL.Path)}
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			s.t.Errorf("parsing multipart Telegram request: %v", err)
			http.Error(w, "invalid multipart form", http.StatusBadRequest)
			return
		}
		request.form = cloneValues(r.MultipartForm.Value)
		request.files = s.readMultipartFiles(r.MultipartForm.File)
	} else {
		if err := r.ParseForm(); err != nil {
			s.t.Errorf("parsing Telegram request: %v", err)
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		request.form = cloneValues(r.PostForm)
	}

	select {
	case s.requests <- request:
	default:
		s.t.Errorf("Telegram test request buffer is full")
	}

	response := s.nextResponse(request.method)
	if response.wait != nil {
		select {
		case <-response.wait:
		case <-r.Context().Done():
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(response.body)
}

// readMultipartFiles reads each uploaded file's content into memory.
func (s *telegramTestServer) readMultipartFiles(fileHeaders map[string][]*multipart.FileHeader) map[string][]byte {
	files := make(map[string][]byte, len(fileHeaders))
	for field, headers := range fileHeaders {
		if len(headers) == 0 {
			continue
		}
		f, err := headers[0].Open()
		if err != nil {
			s.t.Errorf("opening uploaded file %q: %v", field, err)
			continue
		}
		data, err := io.ReadAll(f)
		_ = f.Close()
		if err != nil {
			s.t.Errorf("reading uploaded file %q: %v", field, err)
			continue
		}
		files[field] = data
	}
	return files
}

func (s *telegramTestServer) nextResponse(method string) telegramTestResponse {
	s.mu.Lock()
	defer s.mu.Unlock()

	if queued := s.responses[method]; len(queued) > 0 {
		response := queued[0]
		s.responses[method] = queued[1:]
		return response
	}

	switch method {
	case "getMe":
		return telegramOK(map[string]any{
			"id":         1,
			"is_bot":     true,
			"first_name": "Test",
			"username":   "test_bot",
		})
	case "sendMessage", "editMessageText":
		return telegramOK(map[string]any{"message_id": 1})
	case "deleteMessage", "answerCallbackQuery", "deleteWebhook", "setWebhook":
		return telegramOK(true)
	case "getUpdates":
		// A completed poll immediately starts another request. Holding this one
		// open prevents an empty-response busy loop until test cleanup cancels it.
		return telegramTestResponse{
			body: telegramOK([]tgbotapi.Update{}).body,
			wait: s.pollRelease,
		}
	default:
		s.t.Errorf("unexpected Telegram API method %q", method)
		return telegramFailure("unexpected method")
	}
}

func (s *telegramTestServer) request(t *testing.T, method string) telegramTestRequest {
	t.Helper()

	for {
		for i, request := range s.pending {
			if request.method != method {
				continue
			}
			s.pending = append(s.pending[:i], s.pending[i+1:]...)
			return request
		}

		select {
		case request := <-s.requests:
			if request.method == method {
				return request
			}
			s.pending = append(s.pending, request)
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for Telegram API method %q", method)
			return telegramTestRequest{}
		}
	}
}

func telegramOK(result any) telegramTestResponse {
	return telegramTestResponse{body: marshalTelegramResponse(map[string]any{
		"ok":     true,
		"result": result,
	})}
}

func telegramFailure(description string) telegramTestResponse {
	return telegramTestResponse{body: marshalTelegramResponse(map[string]any{
		"ok":          false,
		"error_code":  400,
		"description": description,
	})}
}

func marshalTelegramResponse(response any) []byte {
	body, err := json.Marshal(response)
	if err != nil {
		panic(err)
	}
	return body
}

func cloneValues(values url.Values) url.Values {
	clone := make(url.Values, len(values))
	for key, values := range values {
		clone[key] = append([]string(nil), values...)
	}
	return clone
}
