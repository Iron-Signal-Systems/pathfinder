package cisakev

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchURLPreservesReceivedEntityBytesAndRequestContract(t *testing.T) {
	t.Parallel()

	body := []byte("{\"catalogVersion\":\"test\",\"vulnerabilities\":[]}\n")

	server := httptest.NewTLSServer(
		http.HandlerFunc(func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			if request.Method != http.MethodGet {
				t.Errorf("method = %q", request.Method)
			}
			if request.Header.Get("Accept") != "application/json" {
				t.Errorf("Accept = %q", request.Header.Get("Accept"))
			}
			if request.Header.Get("Accept-Encoding") != "identity" {
				t.Errorf(
					"Accept-Encoding = %q",
					request.Header.Get("Accept-Encoding"),
				)
			}
			if request.Header.Get("User-Agent") != UserAgent {
				t.Errorf("User-Agent = %q", request.Header.Get("User-Agent"))
			}

			writer.Header().Set("Content-Type", "application/json")
			writer.Header().Set("ETag", `"test-etag"`)
			writer.Header().Set("Last-Modified", "Sun, 13 Sep 2026 10:00:00 GMT")
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write(body)
		}),
	)
	defer server.Close()

	response, err := fetchURL(
		context.Background(),
		server.Client(),
		server.URL,
		int64(len(body)+1),
	)
	if err != nil {
		t.Fatalf("fetchURL() error = %v", err)
	}
	defer response.Body.Close()

	received, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}

	if string(received) != string(body) {
		t.Fatalf("body = %q, want %q", received, body)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d", response.StatusCode)
	}
	if response.ContentType != "application/json" {
		t.Fatalf("ContentType = %q", response.ContentType)
	}
	if response.ETag != `"test-etag"` {
		t.Fatalf("ETag = %q", response.ETag)
	}
	if response.PreservationContentEncoding() != "identity" {
		t.Fatalf(
			"PreservationContentEncoding() = %q",
			response.PreservationContentEncoding(),
		)
	}
	if !response.UsesIdentityEncoding() {
		t.Fatal("UsesIdentityEncoding() = false, want true")
	}
}

func TestFetchURLReturnsNonSuccessHTTPResponseForCallerPreservation(t *testing.T) {
	t.Parallel()

	server := httptest.NewTLSServer(
		http.HandlerFunc(func(
			writer http.ResponseWriter,
			_ *http.Request,
		) {
			writer.WriteHeader(http.StatusForbidden)
			_, _ = writer.Write([]byte("access denied"))
		}),
	)
	defer server.Close()

	response, err := fetchURL(
		context.Background(),
		server.Client(),
		server.URL,
		1024,
	)
	if err != nil {
		t.Fatalf("fetchURL() error = %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("StatusCode = %d", response.StatusCode)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "access denied" {
		t.Fatalf("body = %q", body)
	}
}

func TestFetchURLRejectsResponseOverLimit(t *testing.T) {
	t.Parallel()

	server := httptest.NewTLSServer(
		http.HandlerFunc(func(
			writer http.ResponseWriter,
			_ *http.Request,
		) {
			_, _ = writer.Write([]byte("123456"))
		}),
	)
	defer server.Close()

	response, err := fetchURL(
		context.Background(),
		server.Client(),
		server.URL,
		5,
	)
	if err != nil {
		t.Fatalf("fetchURL() error = %v", err)
	}
	defer response.Body.Close()

	_, err = io.ReadAll(response.Body)
	if !errors.Is(err, ErrResponseTooLarge) {
		t.Fatalf("ReadAll() error = %v, want ErrResponseTooLarge", err)
	}
}

func TestFetchURLRequiresHTTPS(t *testing.T) {
	t.Parallel()

	if _, err := fetchURL(
		context.Background(),
		http.DefaultClient,
		"http://example.invalid/catalog.json",
		1024,
	); err == nil {
		t.Fatal("fetchURL() error = nil, want HTTPS rejection")
	}
}

func TestNewHTTPClientRefusesHTTPSDowngradeRedirect(t *testing.T) {
	t.Parallel()

	httpServer := httptest.NewServer(
		http.HandlerFunc(func(
			writer http.ResponseWriter,
			_ *http.Request,
		) {
			_, _ = writer.Write([]byte("unexpected"))
		}),
	)
	defer httpServer.Close()

	httpsServer := httptest.NewTLSServer(
		http.HandlerFunc(func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			http.Redirect(
				writer,
				request,
				httpServer.URL,
				http.StatusFound,
			)
		}),
	)
	defer httpsServer.Close()

	client := httpsServer.Client()
	client.CheckRedirect = NewHTTPClient().CheckRedirect

	_, err := client.Get(httpsServer.URL)
	if err == nil {
		t.Fatal("redirect error = nil, want HTTPS downgrade rejection")
	}
	if !strings.Contains(err.Error(), "non-HTTPS") {
		t.Fatalf("redirect error = %v", err)
	}
}
