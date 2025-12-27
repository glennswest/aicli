package discovery

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/mdns"
)

// OllamaService represents a discovered Ollama instance
type OllamaService struct {
	Name     string
	Host     string
	Port     int
	IP       string
	Endpoint string
	TLS      bool // true if service advertises HTTPS
}

// InsecureSkipVerify controls whether TLS certificate verification is skipped
var InsecureSkipVerify bool

// getHTTPClient returns an HTTP client with appropriate TLS settings
func getHTTPClient() *http.Client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: InsecureSkipVerify,
		},
	}
	return &http.Client{
		Timeout:   3 * time.Second,
		Transport: transport,
	}
}

// CheckLocalOllama checks if Ollama is running on localhost:11434
func CheckLocalOllama() bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://localhost:11434/v1/models")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

// DiscoverOllama searches for Ollama services via mDNS
// Returns a list of discovered services, sorted with HTTPS first
func DiscoverOllama(timeout time.Duration) ([]OllamaService, error) {
	var services []OllamaService

	entriesCh := make(chan *mdns.ServiceEntry, 10)
	done := make(chan struct{})

	go func() {
		for entry := range entriesCh {
			ip := ""
			if entry.AddrV4 != nil {
				ip = entry.AddrV4.String()
			} else if entry.AddrV6 != nil {
				ip = entry.AddrV6.String()
			}

			if ip == "" {
				continue
			}

			// Parse TXT records for proto=http or proto=https
			useTLS := false
			for _, txt := range entry.InfoFields {
				if txt == "proto=https" {
					useTLS = true
					break
				}
			}

			proto := "http"
			if useTLS {
				proto = "https"
			}

			svc := OllamaService{
				Name:     entry.Name,
				Host:     entry.Host,
				Port:     entry.Port,
				IP:       ip,
				TLS:      useTLS,
				Endpoint: fmt.Sprintf("%s://%s:%d/v1", proto, ip, entry.Port),
			}
			services = append(services, svc)
		}
		close(done)
	}()

	params := mdns.DefaultParams("_ollama._tcp")
	params.Entries = entriesCh
	params.Timeout = timeout

	err := mdns.Query(params)
	close(entriesCh)
	<-done

	if err != nil {
		return nil, err
	}

	// Sort services: HTTPS first, then by name
	sort.Slice(services, func(i, j int) bool {
		if services[i].TLS != services[j].TLS {
			return services[i].TLS // TLS services come first
		}
		return services[i].Name < services[j].Name
	})

	return services, nil
}

// VerifyEndpoint checks if an Ollama endpoint is reachable and working
func VerifyEndpoint(endpoint string) bool {
	client := getHTTPClient()
	resp, err := client.Get(endpoint + "/models")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

// AutoDiscover attempts to find an Ollama instance:
// 1. Check localhost first
// 2. If not found, use mDNS discovery (preferring HTTPS)
// Returns the endpoint URL, host, and whether TLS is used
func AutoDiscover() (endpoint string, host string, useTLS bool) {
	// First check localhost
	if CheckLocalOllama() {
		return "http://localhost:11434/v1", "localhost", false
	}

	// Try mDNS discovery
	services, err := DiscoverOllama(3 * time.Second)
	if err != nil || len(services) == 0 {
		return "", "", false
	}

	// Services are already sorted with HTTPS first
	// Only return verified Ollama endpoints
	for _, svc := range services {
		if VerifyEndpoint(svc.Endpoint) {
			return svc.Endpoint, svc.Host, svc.TLS
		}
	}

	// Don't return unverified endpoints - they might not be Ollama servers
	return "", "", false
}

// IsEncrypted returns true if the endpoint uses HTTPS
func IsEncrypted(endpoint string) bool {
	return strings.HasPrefix(endpoint, "https://")
}
