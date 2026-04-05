package client

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestNewSignerValidation(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		cfg  Config
	}{
		{
			name: "missing kid",
			cfg: Config{
				Secret: "secret",
			},
		},
		{
			name: "missing secret",
			cfg: Config{
				Kid: "kid",
			},
		},
		{
			name: "duplicate signed headers",
			cfg: Config{
				Kid:           "kid",
				Secret:        "secret",
				SignedHeaders: []string{"Content-Type", "content-type"},
			},
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if _, err := NewSigner(testCase.cfg); err == nil {
				t.Fatal("NewSigner() error = nil, want non-nil")
			}
		})
	}
}

func TestRequestSignerSignMatchesServerVector(t *testing.T) {
	t.Parallel()

	signer, err := NewSigner(Config{
		Kid:           "c0y44e8LL4",
		Secret:        "QTEiL2Jy92",
		SignedHeaders: []string{"header1", "header2"},
	})
	if err != nil {
		t.Fatalf("NewSigner() error = %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, "http://example.test", strings.NewReader("DuqjbeoyE9LIo77MaATfF0zl3hu2BZ31"))
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v", err)
	}

	req.Header.Set("Header1", "header1")
	req.Header.Set("Header2", "header2")
	req.Header.Set("Auth-CorrelationId", "1fkEphx2qq")

	if err := signer.Sign(req); err != nil {
		t.Fatalf("Sign() error = %v", err)
	}

	if got, want := req.Header.Get("Auth-Kid"), "c0y44e8LL4"; got != want {
		t.Fatalf("Auth-Kid = %q, want %q", got, want)
	}
	if got, want := req.Header.Get("Auth-Headers"), "Header1;Header2"; got != want {
		t.Fatalf("Auth-Headers = %q, want %q", got, want)
	}
	if got, want := req.Header.Get("Signature"), "XciMlTpNQSefPAjCbHzHU6fF3YorGGOMyP8qMuYKCOc3Z1MD5iSb9dgUyvg6arCRd/Bz4/EfJRO00HXLZLX1Dw=="; got != want {
		t.Fatalf("Signature = %q, want %q", got, want)
	}
}

func TestRequestSignerSignGeneratesCorrelationID(t *testing.T) {
	t.Parallel()

	signer, err := NewSigner(Config{
		Kid:    "kid",
		Secret: "secret",
	})
	if err != nil {
		t.Fatalf("NewSigner() error = %v", err)
	}

	req, err := http.NewRequest(http.MethodGet, "http://example.test", nil)
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v", err)
	}

	if err := signer.Sign(req); err != nil {
		t.Fatalf("Sign() error = %v", err)
	}

	if req.Header.Get("Auth-CorrelationId") == "" {
		t.Fatal("Auth-CorrelationId = empty, want generated value")
	}
}

func TestRequestSignerSignRestoresBodyWhenGetBodyIsMissing(t *testing.T) {
	t.Parallel()

	signer, err := NewSigner(Config{
		Kid:    "kid",
		Secret: "secret",
	})
	if err != nil {
		t.Fatalf("NewSigner() error = %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, "http://example.test", io.NopCloser(strings.NewReader("payload")))
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v", err)
	}
	req.GetBody = nil

	if err := signer.Sign(req); err != nil {
		t.Fatalf("Sign() error = %v", err)
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("io.ReadAll() error = %v", err)
	}

	if got, want := string(body), "payload"; got != want {
		t.Fatalf("request body after Sign() = %q, want %q", got, want)
	}
	if req.GetBody == nil {
		t.Fatal("GetBody = nil, want replayable body")
	}
}

func TestTransportSignsClonedRequest(t *testing.T) {
	t.Parallel()

	base := &captureTransport{}
	transport := NewTransport(base, Config{
		Kid:           "kid",
		Secret:        "secret",
		SignedHeaders: []string{"Content-Type"},
	})

	req, err := http.NewRequest(http.MethodPost, "http://example.test", io.NopCloser(bytes.NewBufferString(`{"message":"hello"}`)))
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v", err)
	}
	req.GetBody = nil
	req.Header.Set("Content-Type", "application/json")

	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip() error = %v", err)
	}
	defer resp.Body.Close()

	if got := req.Header.Get("Auth-Kid"); got != "" {
		t.Fatalf("original request mutated with Auth-Kid = %q", got)
	}
	if got, want := base.req.Header.Get("Auth-Kid"), "kid"; got != want {
		t.Fatalf("captured Auth-Kid = %q, want %q", got, want)
	}
	if got, want := string(base.body), `{"message":"hello"}`; got != want {
		t.Fatalf("captured body = %q, want %q", got, want)
	}

	originalBody, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("io.ReadAll(original body) error = %v", err)
	}
	if got, want := string(originalBody), `{"message":"hello"}`; got != want {
		t.Fatalf("original body after RoundTrip() = %q, want %q", got, want)
	}
}

type captureTransport struct {
	req  *http.Request
	body []byte
}

func (c *captureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	c.req = req
	if req.Body != nil {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		c.body = body
	}

	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader("ok")),
		Request:    req,
	}, nil
}
