package main

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"time"
)

// runHealthcheck hace una petición GET a url y devuelve el código de salida
// que espera Docker: 0 = healthy, 1 = unhealthy.
// insecure permite certificados autofirmados (solo para el proxy).
func runHealthcheck(url string, insecure bool) int {
	client := &http.Client{
		Timeout: 3 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: insecure},
		},
	}
	res, err := client.Get(url)
	if err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck:", err)
		return 1
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "healthcheck: estado", res.Status)
		return 1
	}
	return 0
}
