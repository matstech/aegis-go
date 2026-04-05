package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/matstech/aegis-go/client"
)

func main() {
	signer, err := client.NewSigner(client.Config{
		Kid:           "test",
		Secret:        "integration-secret",
		SignedHeaders: []string{"Content-Type"},
	})
	if err != nil {
		log.Fatal(err)
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

	if err := signer.Sign(req); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Auth-CorrelationId:", req.Header.Get("Auth-CorrelationId"))
	fmt.Println("Auth-Kid:", req.Header.Get("Auth-Kid"))
	fmt.Println("Auth-Headers:", req.Header.Get("Auth-Headers"))
	fmt.Println("Signature:", req.Header.Get("Signature"))
}
