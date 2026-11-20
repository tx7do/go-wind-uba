package server

import (
	"bytes"
	"context"
	"errors"
	"io"
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	authenticationV1 "go-wind-uba/api/gen/go/authentication/service/v1"
)

type stubAccessTokenChecker struct {
	validToken string
}

func (s *stubAccessTokenChecker) IsValidAccessToken(_ context.Context, accessToken string, _ bool) (bool, *authenticationV1.UserTokenPayload) {
	return accessToken == s.validToken, nil
}

func (s *stubAccessTokenChecker) IsBlockedAccessToken(_ context.Context, _ string) bool {
	return false
}

// countingBody 记录是否被读过；用于断言"拦截发生在读取 body 之前"。
type countingBody struct {
	read bool
}

func (b *countingBody) Read([]byte) (int, error) {
	b.read = true
	return 0, io.EOF
}

func (b *countingBody) Close() error { return nil }

func TestUploadBodyGuardRejectsDeclaredOversizeWithoutReading(t *testing.T) {
	body := &countingBody{}
	nextCalled := false

	h := uploadBodyGuardWith(func(*nethttp.Request) int64 { return 16 })(nethttp.HandlerFunc(func(nethttp.ResponseWriter, *nethttp.Request) {
		nextCalled = true
	}))

	req := httptest.NewRequest(nethttp.MethodPost, "/upload", body)
	req.ContentLength = 17
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != nethttp.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", rec.Code, nethttp.StatusRequestEntityTooLarge)
	}
	if nextCalled {
		t.Fatal("next handler should not be called for oversize body")
	}
	if body.read {
		t.Fatal("body must not be read when Content-Length already exceeds the limit")
	}
}

func TestRequestBodyGuardAllowsSmallUnauthenticatedBody(t *testing.T) {
	nextCalled := false

	h := requestBodyGuard(&stubAccessTokenChecker{validToken: "good"})(nethttp.HandlerFunc(func(nethttp.ResponseWriter, *nethttp.Request) {
		nextCalled = true
	}))

	req := httptest.NewRequest(nethttp.MethodPost, "/login", strings.NewReader(`{"username":"admin"}`))
	req.ContentLength = int64(len(`{"username":"admin"}`))
	h.ServeHTTP(httptest.NewRecorder(), req)

	if !nextCalled {
		t.Fatal("small unauthenticated request should pass through")
	}
}

func TestRequestBodyGuardRejectsLargeUnauthenticatedBodyWithoutReading(t *testing.T) {
	body := &countingBody{}
	nextCalled := false

	h := requestBodyGuard(&stubAccessTokenChecker{validToken: "good"})(nethttp.HandlerFunc(func(nethttp.ResponseWriter, *nethttp.Request) {
		nextCalled = true
	}))

	req := httptest.NewRequest(nethttp.MethodPost, "/login", body)
	req.ContentLength = maxUnauthenticatedBody + 1
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != nethttp.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, nethttp.StatusUnauthorized)
	}
	if nextCalled {
		t.Fatal("next handler should not be called without a valid token")
	}
	if body.read {
		t.Fatal("body must not be read before authentication")
	}
}

func TestRequestBodyGuardRejectsInvalidTokenForLargeBody(t *testing.T) {
	h := requestBodyGuard(&stubAccessTokenChecker{validToken: "good"})(nethttp.HandlerFunc(func(nethttp.ResponseWriter, *nethttp.Request) {
		t.Fatal("next handler should not be called with an invalid token")
	}))

	req := httptest.NewRequest(nethttp.MethodPost, "/profile/avatar", &countingBody{})
	req.ContentLength = maxUnauthenticatedBody + 1
	req.Header.Set("Authorization", "Bearer bad")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != nethttp.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, nethttp.StatusUnauthorized)
	}
}

func TestRequestBodyGuardAllowsLargeAuthenticatedBody(t *testing.T) {
	nextCalled := false

	h := requestBodyGuard(&stubAccessTokenChecker{validToken: "good"})(nethttp.HandlerFunc(func(_ nethttp.ResponseWriter, r *nethttp.Request) {
		nextCalled = true
		if _, err := io.Copy(io.Discard, r.Body); err != nil {
			t.Errorf("read large body: %v", err)
		}
	}))

	payload := bytes.Repeat([]byte("a"), maxUnauthenticatedBody+1024)
	req := httptest.NewRequest(nethttp.MethodPost, "/profile/avatar", bytes.NewReader(payload))
	req.ContentLength = int64(len(payload))
	req.Header.Set("Authorization", "Bearer good")

	h.ServeHTTP(httptest.NewRecorder(), req)

	if !nextCalled {
		t.Fatal("large request with a valid token should pass through")
	}
}

func TestRequestBodyGuardCapsChunkedUnauthenticatedBody(t *testing.T) {
	var readErr error

	h := requestBodyGuard(&stubAccessTokenChecker{validToken: "good"})(nethttp.HandlerFunc(func(_ nethttp.ResponseWriter, r *nethttp.Request) {
		_, readErr = io.ReadAll(r.Body)
	}))

	req := httptest.NewRequest(nethttp.MethodPost, "/login", strings.NewReader(strings.Repeat("a", maxUnauthenticatedBody+1)))
	req.ContentLength = -1

	h.ServeHTTP(httptest.NewRecorder(), req)

	var maxBytesErr *nethttp.MaxBytesError
	if !errors.As(readErr, &maxBytesErr) {
		t.Fatalf("read error = %v, want *http.MaxBytesError", readErr)
	}
}

func TestRequestBodyGuardRejectsBodyBeyondGlobalLimit(t *testing.T) {
	body := &countingBody{}

	h := requestBodyGuard(&stubAccessTokenChecker{validToken: "good"})(nethttp.HandlerFunc(func(nethttp.ResponseWriter, *nethttp.Request) {
		t.Fatal("next handler should not be called for an oversize body")
	}))

	req := httptest.NewRequest(nethttp.MethodPost, "/upload", body)
	req.ContentLength = maxRequestBody + 1
	req.Header.Set("Authorization", "Bearer good")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != nethttp.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", rec.Code, nethttp.StatusRequestEntityTooLarge)
	}
	if body.read {
		t.Fatal("body must not be read when Content-Length already exceeds the global limit")
	}
}
