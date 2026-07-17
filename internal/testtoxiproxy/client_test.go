package testtoxiproxy

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestClientCreateProxy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", request.Method)
		}
		if request.URL.Path != "/proxies" {
			t.Errorf("path = %s, want /proxies", request.URL.Path)
		}
		assertRequestBody(t, request, `{"name":"gotenancy_redis","listen":"0.0.0.0:8668","upstream":"redis:6379","enabled":true}`)
		writer.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	proxy, err := New(server.URL).CreateProxy(context.Background(), "gotenancy_redis", "0.0.0.0:8668", "redis:6379")
	if err != nil {
		t.Fatalf("CreateProxy() error = %v", err)
	}
	if proxy.Name != "gotenancy_redis" {
		t.Fatalf("proxy.Name = %q, want gotenancy_redis", proxy.Name)
	}
}

func TestClientAddTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", request.Method)
		}
		if request.URL.Path != "/proxies/gotenancy_redis/toxics" {
			t.Errorf("path = %s, want /proxies/gotenancy_redis/toxics", request.URL.Path)
		}
		assertRequestBody(t, request, `{"name":"blocked","type":"timeout","stream":"downstream","toxicity":1.0,"attributes":{"timeout":0}}`)
		writer.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	if err := New(server.URL).AddTimeout(context.Background(), "gotenancy_redis", "blocked"); err != nil {
		t.Fatalf("AddTimeout() error = %v", err)
	}
}

func TestClientSetEnabled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", request.Method)
		}
		if request.URL.Path != "/proxies/gotenancy_redis" {
			t.Errorf("path = %s, want /proxies/gotenancy_redis", request.URL.Path)
		}
		assertRequestBody(t, request, `{"enabled":false}`)
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	if err := New(server.URL).SetEnabled(context.Background(), "gotenancy_redis", false); err != nil {
		t.Fatalf("SetEnabled() error = %v", err)
	}
}

func TestClientDeletePaths(t *testing.T) {
	requests := make(chan string, 2)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", request.Method)
		}
		requests <- request.URL.Path
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := New(server.URL)
	if err := client.RemoveToxic(context.Background(), "gotenancy_redis", "blocked"); err != nil {
		t.Fatalf("RemoveToxic() error = %v", err)
	}
	if err := client.DeleteProxy(context.Background(), "gotenancy_redis"); err != nil {
		t.Fatalf("DeleteProxy() error = %v", err)
	}

	if got := <-requests; got != "/proxies/gotenancy_redis/toxics/blocked" {
		t.Fatalf("first DELETE path = %s, want toxic path", got)
	}
	if got := <-requests; got != "/proxies/gotenancy_redis" {
		t.Fatalf("second DELETE path = %s, want proxy path", got)
	}
}

func TestClientWait(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/version" {
			t.Errorf("request = %s %s, want GET /version", request.Method, request.URL.Path)
		}
		if attempts.Add(1) == 1 {
			writer.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := New(server.URL).Wait(ctx); err != nil {
		t.Fatalf("Wait() error = %v", err)
	}
	if attempts.Load() < 2 {
		t.Fatalf("version checks = %d, want at least 2", attempts.Load())
	}
}

func TestClientWaitHonorsDeadline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	err := New(server.URL).Wait(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Wait() error = %v, want context deadline exceeded", err)
	}
}

func TestClientNon2xxErrorIsBounded(t *testing.T) {
	body := strings.Repeat("x", maxErrorBody*2)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusBadGateway)
		_, _ = io.WriteString(writer, body)
	}))
	defer server.Close()

	err := New(server.URL).DeleteProxy(context.Background(), "gotenancy_redis")
	if err == nil {
		t.Fatal("DeleteProxy() error = nil, want non-2xx error")
	}
	if !strings.Contains(err.Error(), "502 Bad Gateway") {
		t.Fatalf("error = %q, want HTTP status", err)
	}
	if len(err.Error()) > len("toxiproxy request failed: 502 Bad Gateway: ")+maxErrorBody {
		t.Fatalf("error length = %d, exceeds bounded status/body message", len(err.Error()))
	}
}

func assertRequestBody(t *testing.T, request *http.Request, want string) {
	t.Helper()
	body, err := io.ReadAll(request.Body)
	if err != nil {
		t.Fatalf("read request body: %v", err)
	}
	if got := string(body); got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}
