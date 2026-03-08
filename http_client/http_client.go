package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/net/http2"
)

// headers is a flag.Value that can be specified multiple times.
type headers []string

func (h *headers) String() string  { return strings.Join(*h, ", ") }
func (h *headers) Set(v string) error {
	*h = append(*h, v)
	return nil
}

func main() {
	var (
		method   = flag.String("method", "GET", "HTTP method")
		url      = flag.String("url", "", "Target URL (required)")
		body     = flag.String("body", "", "Request body. Use '-' to read from stdin")
		timeout  = flag.Duration("timeout", 30*time.Second, "Request timeout")
		http1    = flag.Bool("http1.1", false, "Use HTTP/1.1 instead of HTTP/2")
		insecure = flag.Bool("insecure", false, "Skip TLS certificate verification")
		cert     = flag.String("cert", "", "Path to client TLS certificate file")
		key      = flag.String("key", "", "Path to client TLS key file")
		ca       = flag.String("ca", "", "Path to CA certificate file")
	)
	var hdrs headers
	flag.Var(&hdrs, "header", `Request header in \"Name: Value\" format. May be repeated.`)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: http_client [flags]\n\nFlags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  http_client -url https://api.example.com/users\n")
		fmt.Fprintf(os.Stderr, "  http_client -method POST -url https://api.example.com/users -header 'Content-Type: application/json' -body '{\"name\":\"alice\"}'\n")
		fmt.Fprintf(os.Stderr, "  echo '{\"name\":\"alice\"}' | http_client -method POST -url https://api.example.com/users -body -\n")
	}
	flag.Parse()

	if *url == "" {
		fmt.Fprintln(os.Stderr, "error: -url is required")
		flag.Usage()
		os.Exit(1)
	}

	bodyBytes, err := readBody(*body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading body: %v\n", err)
		os.Exit(1)
	}

	client, err := buildClient(*timeout, *http1, *insecure, *cert, *key, *ca)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error building client: %v\n", err)
		os.Exit(1)
	}

	req, err := http.NewRequest(*method, *url, bytes.NewReader(bodyBytes))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error building request: %v\n", err)
		os.Exit(1)
	}

	for _, h := range hdrs {
		name, value, ok := strings.Cut(h, ":")
		if !ok {
			fmt.Fprintf(os.Stderr, "error: invalid header %q (expected \"Name: Value\")\n", h)
			os.Exit(1)
		}
		req.Header.Set(strings.TrimSpace(name), strings.TrimSpace(value))
	}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	fmt.Fprintf(os.Stderr, "%s %s\n", resp.Proto, resp.Status)
	for k, vals := range resp.Header {
		for _, v := range vals {
			fmt.Fprintf(os.Stderr, "%s: %s\n", k, v)
		}
	}
	fmt.Fprintln(os.Stderr)

	if _, err := io.Copy(os.Stdout, resp.Body); err != nil {
		fmt.Fprintf(os.Stderr, "error reading response: %v\n", err)
		os.Exit(1)
	}

	if resp.StatusCode >= 400 {
		os.Exit(1)
	}
}

func readBody(s string) ([]byte, error) {
	if s == "" {
		return nil, nil
	}
	if s == "-" {
		return io.ReadAll(os.Stdin)
	}
	return []byte(s), nil
}

func buildClient(timeout time.Duration, forceHTTP1 bool, insecure bool, certFile, keyFile, caFile string) (*http.Client, error) {
	tlsCfg := &tls.Config{InsecureSkipVerify: insecure} //nolint:gosec

	if caFile != "" {
		pem, err := os.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("reading CA cert: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("no valid certificates in %q", caFile)
		}
		tlsCfg.RootCAs = pool
	}

	if certFile != "" && keyFile != "" {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, fmt.Errorf("loading client certificate: %w", err)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}

	transport := &http.Transport{TLSClientConfig: tlsCfg}
	if !forceHTTP1 {
		if err := http2.ConfigureTransport(transport); err != nil {
			return nil, fmt.Errorf("configuring HTTP/2: %w", err)
		}
	}

	return &http.Client{Transport: transport, Timeout: timeout}, nil
}
