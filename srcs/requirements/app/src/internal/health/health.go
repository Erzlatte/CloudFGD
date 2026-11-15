package health

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"time"
)

const (
	Healthy   = 0
	Unhealthy = 1
)

func Check(url string, insecure bool) int {
	client := &http.Client{
		Timeout: 3 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: insecure},
		},
	}

	res, err := client.Get(url)
	if err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck:", err)
		return Unhealthy
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "healthcheck: estado", res.Status)
		return Unhealthy
	}
	return Healthy
}
