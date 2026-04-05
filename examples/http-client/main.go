package main

import (
	"log"
	"net/http"
	"strings"

	"github.com/matstech/aegis-go/client"
)

func main() {
	httpClient := &http.Client{
		Transport: client.NewTransport(nil, client.Config{
			Kid:           "test",
			Secret:        "integration-secret",
			SignedHeaders: []string{"Content-Type", "Authorization"},
		}),
	}

	req, err := http.NewRequest(
		http.MethodPost,
		"http://localhost:8080/anything",
		strings.NewReader(`{"message":"hello"}`),
	)
	if err != nil {
		log.Fatal(err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer client-token")

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	log.Printf("response status: %s", resp.Status)
}
