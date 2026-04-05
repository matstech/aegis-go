package client_test

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/matstech/aegis-go/client"
)

func ExampleNewSigner() {
	signer, err := client.NewSigner(client.Config{
		Kid:           "test",
		Secret:        "integration-secret",
		SignedHeaders: []string{"Content-Type"},
	})
	if err != nil {
		panic(err)
	}

	req, err := http.NewRequest(http.MethodPost, "http://example.test", strings.NewReader(`{"message":"hello"}`))
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Auth-CorrelationId", "example-correlation-id")

	if err := signer.Sign(req); err != nil {
		panic(err)
	}

	fmt.Println(req.Header.Get("Auth-Kid"))
	fmt.Println(req.Header.Get("Auth-Headers"))
	fmt.Println(req.Header.Get("Auth-CorrelationId"))
	fmt.Println(req.Header.Get("Signature") != "")
	// Output:
	// test
	// Content-Type
	// example-correlation-id
	// true
}

func ExampleNewTransport() {
	httpClient := &http.Client{
		Transport: client.NewTransport(roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			fmt.Println(req.Header.Get("Auth-Kid"))
			fmt.Println(req.Header.Get("Auth-Headers"))

			body, err := io.ReadAll(req.Body)
			if err != nil {
				return nil, err
			}
			fmt.Println(string(body))

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("ok")),
				Request:    req,
			}, nil
		}), client.Config{
			Kid:           "test",
			Secret:        "integration-secret",
			SignedHeaders: []string{"Content-Type"},
		}),
	}

	req, err := http.NewRequest(http.MethodPost, "http://example.test", strings.NewReader(`{"message":"hello"}`))
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Auth-CorrelationId", "example-correlation-id")

	resp, err := httpClient.Do(req)
	if err != nil {
		panic(err)
	}
	resp.Body.Close()

	// Output:
	// test
	// Content-Type
	// {"message":"hello"}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
