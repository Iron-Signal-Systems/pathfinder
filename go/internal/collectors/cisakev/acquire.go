package cisakev

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	CatalogURL = "https://www.cisa.gov/sites/default/files/feeds/known_exploited_vulnerabilities.json"

	DefaultMaxResponseBytes int64 = 16 * 1024 * 1024
	UserAgent                     = "Pathfinder/phase-1.4 (Iron Signal Systems; https://github.com/Iron-Signal-Systems/pathfinder)"
)

var ErrResponseTooLarge = errors.New("CISA KEV response exceeds configured byte limit")

// HTTPResponse describes one received HTTP response without interpreting its
// body as KEV data. Body remains the authoritative response entity stream.
type HTTPResponse struct {
	Body            io.ReadCloser
	ContentEncoding string
	ContentLength   int64
	ContentType     string
	ETag            string
	FinalURL        string
	LastModified    string
	Status          string
	StatusCode      int
}

// FetchCatalog retrieves the canonical CISA KEV JSON feed. A received HTTP
// response is returned even when its status is non-2xx so the caller can
// preserve the returned entity body and then record the acquisition outcome.
func FetchCatalog(
	ctx context.Context,
	client *http.Client,
) (*HTTPResponse, error) {
	return fetchURL(
		ctx,
		client,
		CatalogURL,
		DefaultMaxResponseBytes,
	)
}

// NewHTTPClient returns the Phase 1.4 KEV acquisition client.
//
// Transparent HTTP content decompression is disabled. Pathfinder asks for
// identity encoding so bytes read from Response.Body are the bytes supplied to
// artifact preservation rather than an automatically rewritten representation.
func NewHTTPClient() *http.Client {
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          10,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 20 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DisableCompression:    true,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}

	return &http.Client{
		Transport: transport,
		Timeout:   45 * time.Second,
		CheckRedirect: func(
			request *http.Request,
			via []*http.Request,
		) error {
			if len(via) >= 5 {
				return errors.New("CISA KEV redirect limit exceeded")
			}
			if request.URL.Scheme != "https" {
				return fmt.Errorf(
					"CISA KEV redirect refused non-HTTPS destination %q",
					request.URL.String(),
				)
			}

			originalHost := via[0].URL.Hostname()
			if !strings.EqualFold(request.URL.Hostname(), originalHost) {
				return fmt.Errorf(
					"CISA KEV redirect refused host change %q -> %q",
					originalHost,
					request.URL.Hostname(),
				)
			}

			return nil
		},
	}
}

// PreservationContentEncoding converts an absent Content-Encoding header into
// the explicit identity value required by the SourceArtifact preservation
// contract. Non-empty values are preserved verbatim after surrounding
// whitespace is removed.
func (response *HTTPResponse) PreservationContentEncoding() string {
	value := strings.TrimSpace(response.ContentEncoding)
	if value == "" {
		return "identity"
	}

	return value
}

// UsesIdentityEncoding reports whether the received response body can be passed
// directly to the Phase 1.4 JSON parser after preservation.
func (response *HTTPResponse) UsesIdentityEncoding() bool {
	return strings.EqualFold(
		response.PreservationContentEncoding(),
		"identity",
	)
}

func fetchURL(
	ctx context.Context,
	client *http.Client,
	rawURL string,
	maxResponseBytes int64,
) (*HTTPResponse, error) {
	if client == nil {
		return nil, errors.New("CISA KEV HTTP client must not be nil")
	}
	if maxResponseBytes <= 0 {
		return nil, errors.New("CISA KEV max response bytes must be positive")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse CISA KEV URL: %w", err)
	}
	if parsed.Scheme != "https" {
		return nil, fmt.Errorf(
			"CISA KEV acquisition requires HTTPS, got %q",
			parsed.Scheme,
		)
	}

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		parsed.String(),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("create CISA KEV request: %w", err)
	}

	request.Header.Set("Accept", "application/json")
	request.Header.Set("Accept-Encoding", "identity")
	request.Header.Set("User-Agent", UserAgent)

	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("retrieve CISA KEV catalog: %w", err)
	}

	finalURL := ""
	if response.Request != nil && response.Request.URL != nil {
		finalURL = response.Request.URL.String()
	}

	return &HTTPResponse{
		Body: &boundedReadCloser{
			body:      response.Body,
			remaining: maxResponseBytes,
		},
		ContentEncoding: response.Header.Get("Content-Encoding"),
		ContentLength:   response.ContentLength,
		ContentType:     response.Header.Get("Content-Type"),
		ETag:            response.Header.Get("ETag"),
		FinalURL:        finalURL,
		LastModified:    response.Header.Get("Last-Modified"),
		Status:          response.Status,
		StatusCode:      response.StatusCode,
	}, nil
}

type boundedReadCloser struct {
	body      io.ReadCloser
	remaining int64
}

func (reader *boundedReadCloser) Close() error {
	return reader.body.Close()
}

func (reader *boundedReadCloser) Read(buffer []byte) (int, error) {
	if reader.remaining == 0 {
		var probe [1]byte

		count, err := reader.body.Read(probe[:])
		if count != 0 {
			return 0, ErrResponseTooLarge
		}
		if err != nil {
			return 0, err
		}

		return 0, ErrResponseTooLarge
	}

	if int64(len(buffer)) > reader.remaining {
		buffer = buffer[:reader.remaining]
	}

	count, err := reader.body.Read(buffer)
	reader.remaining -= int64(count)

	return count, err
}
