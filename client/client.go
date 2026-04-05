package client

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/textproto"
	"strings"

	"github.com/matstech/aegis-go/internal/protocol"
)

// Config configures Aegis request signing.
type Config struct {
	Kid           string
	Secret        string
	SignedHeaders []string
}

// ValidationError reports invalid signer configuration fields.
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("client: invalid %s: %s", e.Field, e.Message)
}

// RequestSigner signs HTTP requests using the Aegis protocol.
type RequestSigner struct {
	kid           string
	secret        string
	signedHeaders []string
}

// NewSigner validates cfg and returns a signer ready to mutate outbound requests.
func NewSigner(cfg Config) (*RequestSigner, error) {
	normalized, err := normalizeConfig(cfg)
	if err != nil {
		return nil, err
	}

	return &RequestSigner{
		kid:           normalized.Kid,
		secret:        normalized.Secret,
		signedHeaders: normalized.SignedHeaders,
	}, nil
}

// Sign populates the Aegis authentication headers on req.
func (s *RequestSigner) Sign(req *http.Request) error {
	if req == nil {
		return fmt.Errorf("client: request is nil")
	}

	if req.Header == nil {
		req.Header = make(http.Header)
	}

	body, err := snapshotBody(req)
	if err != nil {
		return fmt.Errorf("client: read request body: %w", err)
	}

	correlationID := req.Header.Get(protocol.HeaderAuthCorrelationID)
	if correlationID == "" {
		correlationID, err = generateCorrelationID()
		if err != nil {
			return fmt.Errorf("client: generate correlation id: %w", err)
		}
		req.Header.Set(protocol.HeaderAuthCorrelationID, correlationID)
	}

	if len(s.signedHeaders) > 0 {
		req.Header.Set(protocol.HeaderAuthHeaders, protocol.HeaderList(s.signedHeaders))
	} else {
		req.Header.Del(protocol.HeaderAuthHeaders)
	}
	req.Header.Set(protocol.HeaderAuthKid, s.kid)

	canonical, err := protocol.CanonicalString(correlationID, protocol.HeaderValues(req.Header, s.signedHeaders), body)
	if err != nil {
		return fmt.Errorf("client: build canonical string: %w", err)
	}

	req.Header.Set(protocol.HeaderSignature, protocol.Sign(s.secret, canonical))
	return nil
}

// NewTransport wraps base and signs requests before they are sent.
//
// Config validation happens when the transport is created. If cfg is invalid,
// the returned RoundTripper will fail requests with that validation error.
func NewTransport(base http.RoundTripper, cfg Config) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}

	signer, err := NewSigner(cfg)
	return &signingTransport{
		base:      base,
		signer:    signer,
		signerErr: err,
	}
}

type signingTransport struct {
	base      http.RoundTripper
	signer    *RequestSigner
	signerErr error
}

func (t *signingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req == nil {
		return nil, fmt.Errorf("client: request is nil")
	}
	if t.signerErr != nil {
		return nil, t.signerErr
	}

	clone, err := cloneRequest(req)
	if err != nil {
		return nil, err
	}
	if err := t.signer.Sign(clone); err != nil {
		return nil, err
	}

	return t.base.RoundTrip(clone)
}

func normalizeConfig(cfg Config) (Config, error) {
	kid := strings.TrimSpace(cfg.Kid)
	if kid == "" {
		return Config{}, ValidationError{Field: "Kid", Message: "must not be empty"}
	}

	secret := strings.TrimSpace(cfg.Secret)
	if secret == "" {
		return Config{}, ValidationError{Field: "Secret", Message: "must not be empty"}
	}

	headers := make([]string, 0, len(cfg.SignedHeaders))
	seen := make(map[string]struct{}, len(cfg.SignedHeaders))

	for index, rawHeader := range cfg.SignedHeaders {
		headerName := textproto.CanonicalMIMEHeaderKey(strings.TrimSpace(rawHeader))
		if headerName == "" {
			return Config{}, ValidationError{
				Field:   fmt.Sprintf("SignedHeaders[%d]", index),
				Message: "must not be empty",
			}
		}

		key := strings.ToLower(headerName)
		if _, found := seen[key]; found {
			return Config{}, ValidationError{
				Field:   fmt.Sprintf("SignedHeaders[%d]", index),
				Message: fmt.Sprintf("duplicate header %q", headerName),
			}
		}

		headers = append(headers, headerName)
		seen[key] = struct{}{}
	}

	return Config{
		Kid:           kid,
		Secret:        secret,
		SignedHeaders: headers,
	}, nil
}

func snapshotBody(req *http.Request) ([]byte, error) {
	if req.Body == nil || req.Body == http.NoBody {
		return nil, nil
	}

	if req.GetBody != nil {
		reader, err := req.GetBody()
		if err != nil {
			return nil, err
		}
		defer reader.Close()

		return io.ReadAll(reader)
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	_ = req.Body.Close()
	setReplayableBody(req, body)
	return body, nil
}

func cloneRequest(req *http.Request) (*http.Request, error) {
	clone := req.Clone(req.Context())
	clone.Header = req.Header.Clone()

	body, err := snapshotBody(req)
	if err != nil {
		return nil, fmt.Errorf("client: read request body: %w", err)
	}

	if req.Body == nil || req.Body == http.NoBody {
		clone.Body = req.Body
		clone.GetBody = req.GetBody
		return clone, nil
	}

	setReplayableBody(clone, body)
	return clone, nil
}

func setReplayableBody(req *http.Request, body []byte) {
	reader := func() io.ReadCloser {
		return io.NopCloser(bytes.NewReader(body))
	}

	req.Body = reader()
	req.GetBody = func() (io.ReadCloser, error) {
		return reader(), nil
	}
	req.ContentLength = int64(len(body))
}

func generateCorrelationID() (string, error) {
	var raw [16]byte

	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}

	return hex.EncodeToString(raw[:]), nil
}
