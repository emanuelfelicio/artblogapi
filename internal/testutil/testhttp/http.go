package testhttp

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/emanuelfelicio/artblogapi/config/response"
)

func DoRequest(t *testing.T, r http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()

	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal request body: %v", err)
		}
		bodyReader = bytes.NewBuffer(b)
	}

	req := httptest.NewRequest(method, path, bodyReader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func DecodeResponse[T any](t *testing.T, w *httptest.ResponseRecorder) response.Response[T] {
	t.Helper()

	var resp response.Response[T]
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v, body: %s", err, w.Body.String())
	}
	return resp
}

func DecodeErrorResponse[T any](t *testing.T, w *httptest.ResponseRecorder) response.ErrorResponse[T] {
	t.Helper()

	var resp response.ErrorResponse[T]
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode error response: %v, body: %s", err, w.Body.String())
	}
	return resp
}
