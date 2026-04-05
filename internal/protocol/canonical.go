package protocol

import (
	"errors"
	"net/http"
	"strings"
)

// HeaderValues returns signed header values in the exact order declared by the caller.
func HeaderValues(headers http.Header, signedHeaders []string) []string {
	values := make([]string, 0, len(signedHeaders))

	for _, headerName := range signedHeaders {
		values = append(values, headers.Get(headerName))
	}

	return values
}

// HeaderList joins signed header names using the separator expected by Aegis.
func HeaderList(signedHeaders []string) string {
	return strings.Join(signedHeaders, ";")
}

// CanonicalString builds the string that Aegis verifies server-side.
func CanonicalString(correlationID string, signedHeaderValues []string, payload []byte) (string, error) {
	if strings.TrimSpace(correlationID) == "" {
		return "", errors.New("protocol: correlation id is required")
	}

	var builder strings.Builder
	builder.WriteString(correlationID)

	for _, value := range signedHeaderValues {
		builder.WriteByte(';')
		builder.WriteString(value)
	}

	if len(payload) > 0 {
		builder.WriteByte(':')
		builder.WriteString(XXH64Hex(payload))
	}

	return builder.String(), nil
}
