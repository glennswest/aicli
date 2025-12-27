package discovery

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"sort"
	"strconv"
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
	// Suppress mdns library's debug logging
	origOutput := log.Writer()
	log.SetOutput(io.Discard)
	defer log.SetOutput(origOutput)

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

// DiscoverOllamaAvahi uses avahi-browse as fallback for cross-subnet mDNS
// This works better than the Go mDNS library in complex network setups
func DiscoverOllamaAvahi(timeout time.Duration) ([]OllamaService, error) {
	// Check if avahi-browse is available
	_, err := exec.LookPath("avahi-browse")
	if err != nil {
		return nil, fmt.Errorf("avahi-browse not found")
	}

	// Run avahi-browse with parseable output
	// -r: resolve, -p: parseable, -t: terminate after query
	cmd := exec.Command("avahi-browse", "-rpt", "_ollama._tcp")

	// Use CombinedOutput to capture both stdout and stderr
	// avahi-browse may write info to stderr even on success
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Check if we got any valid output despite the error
		// avahi-browse sometimes exits with error but has valid data
		if len(output) == 0 {
			return nil, fmt.Errorf("avahi-browse failed: %w", err)
		}
	}

	var services []OllamaService
	scanner := bufio.NewScanner(strings.NewReader(string(output)))

	for scanner.Scan() {
		line := scanner.Text()
		// Parse resolved entries (start with =)
		// Format: =;interface;protocol;name;type;domain;hostname;address;port;txt
		if !strings.HasPrefix(line, "=") {
			continue
		}

		fields := strings.Split(line, ";")
		if len(fields) < 10 {
			continue
		}

		name := fields[3]
		host := fields[6]
		ip := fields[7]
		port, err := strconv.Atoi(fields[8])
		if err != nil {
			continue
		}

		// Parse TXT records for proto=https
		txtFields := fields[9:]
		useTLS := false
		for _, txt := range txtFields {
			if strings.Contains(txt, "proto=https") {
				useTLS = true
				break
			}
		}

		proto := "http"
		if useTLS {
			proto = "https"
		}

		svc := OllamaService{
			Name:     name,
			Host:     host,
			Port:     port,
			IP:       ip,
			TLS:      useTLS,
			Endpoint: fmt.Sprintf("%s://%s:%d/v1", proto, ip, port),
		}
		services = append(services, svc)
	}

	// Sort services: HTTPS first, then by name
	sort.Slice(services, func(i, j int) bool {
		if services[i].TLS != services[j].TLS {
			return services[i].TLS
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

// VerifyEndpointWithCertCheck checks an endpoint, trying secure first then insecure
// Returns: (success, needsInsecure)
func VerifyEndpointWithCertCheck(endpoint string) (bool, bool) {
	// First try with certificate verification (secure)
	secureClient := &http.Client{
		Timeout: 3 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: false,
			},
		},
	}

	resp, err := secureClient.Get(endpoint + "/models")
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == 200 {
			return true, false // Works with valid cert
		}
	}

	// Check if it's a TLS/certificate error - try insecure mode
	// Error messages vary: "certificate", "x509", "tls"
	if err != nil {
		errStr := err.Error()
		isCertError := strings.Contains(errStr, "certificate") ||
			strings.Contains(errStr, "x509") ||
			strings.Contains(errStr, "tls:")
		if isCertError {
			// Try with insecure mode
			insecureClient := &http.Client{
				Timeout: 3 * time.Second,
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: true,
					},
				},
			}

			resp, err := insecureClient.Get(endpoint + "/models")
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == 200 {
					return true, true // Works but needs insecure mode
				}
			}
		}
	}

	return false, false
}

// AutoDiscover attempts to find an Ollama instance:
// 1. Check localhost first
// 2. If not found, use mDNS discovery (Go library first, then avahi fallback)
// Returns the endpoint URL, host, whether TLS is used, and whether insecure mode is needed
func AutoDiscover() (endpoint string, host string, useTLS bool, needsInsecure bool) {
	// First check localhost
	if CheckLocalOllama() {
		return "http://localhost:11434/v1", "localhost", false, false
	}

	// Try Go mDNS library first
	services, err := DiscoverOllama(3 * time.Second)

	// If Go mDNS fails or finds nothing, try avahi-browse as fallback
	// avahi works better for cross-subnet mDNS discovery
	if err != nil || len(services) == 0 {
		avahiServices, avahiErr := DiscoverOllamaAvahi(5 * time.Second)
		if avahiErr == nil && len(avahiServices) > 0 {
			services = avahiServices
		}
	}

	if len(services) == 0 {
		return "", "", false, false
	}

	// Services are already sorted with HTTPS first
	// Only return verified Ollama endpoints
	// For HTTPS endpoints, check certificate validity
	for _, svc := range services {
		if svc.TLS {
			// Check with certificate validation first
			ok, insecure := VerifyEndpointWithCertCheck(svc.Endpoint)
			if ok {
				return svc.Endpoint, svc.Host, svc.TLS, insecure
			}
		} else {
			// HTTP endpoint - just verify it works
			if VerifyEndpoint(svc.Endpoint) {
				return svc.Endpoint, svc.Host, svc.TLS, false
			}
		}
	}

	// Don't return unverified endpoints - they might not be Ollama servers
	return "", "", false, false
}

// IsEncrypted returns true if the endpoint uses HTTPS
func IsEncrypted(endpoint string) bool {
	return strings.HasPrefix(endpoint, "https://")
}
