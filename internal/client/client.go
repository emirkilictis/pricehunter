// Package client provides a stealth HTTP client with uTLS fingerprinting,
// proxy rotation, and User-Agent rotation for polite web scraping.
package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	utls "github.com/refraction-networking/utls"
)

// StealthClient wraps http.Client with browser TLS fingerprinting and proxy rotation.
type StealthClient struct {
	proxies    []string
	userAgents []string
	proxyIndex uint64
	timeout    time.Duration
	mu         sync.RWMutex
}

// NewStealthClient creates a new StealthClient with the given proxies, user agents, and timeout.
func NewStealthClient(proxies []string, userAgents []string, timeoutSec int) *StealthClient {
	if timeoutSec <= 0 {
		timeoutSec = 15
	}
	return &StealthClient{
		proxies:    proxies,
		userAgents: userAgents,
		timeout:    time.Duration(timeoutSec) * time.Second,
	}
}

// nextProxy returns the next proxy in round-robin fashion. Returns empty string if no proxies.
func (sc *StealthClient) nextProxy() string {
	if len(sc.proxies) == 0 {
		return ""
	}
	idx := atomic.AddUint64(&sc.proxyIndex, 1)
	return sc.proxies[idx%uint64(len(sc.proxies))]
}

// RandomUserAgent returns a random user agent string from the configured list.
func (sc *StealthClient) RandomUserAgent() string {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	if len(sc.userAgents) == 0 {
		return "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36"
	}
	return sc.userAgents[rand.Intn(len(sc.userAgents))]
}

// utlsDialTLS performs a TLS handshake mimicking a Chrome browser fingerprint.
// ALPN is restricted to HTTP/1.1 to match Go's http.Transport expectations.
func utlsDialTLS(ctx context.Context, network, addr string) (net.Conn, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}

	dialer := &net.Dialer{Timeout: 10 * time.Second}
	conn, err := dialer.DialContext(ctx, network, addr)
	if err != nil {
		return nil, fmt.Errorf("dial failed: %w", err)
	}

	// Use HelloCustom with Chrome-like spec but force HTTP/1.1 only ALPN
	spec := &utls.ClientHelloSpec{
		TLSVersMax: utls.VersionTLS13,
		TLSVersMin: utls.VersionTLS12,
		CipherSuites: []uint16{
			utls.GREASE_PLACEHOLDER,
			utls.TLS_AES_128_GCM_SHA256,
			utls.TLS_AES_256_GCM_SHA384,
			utls.TLS_CHACHA20_POLY1305_SHA256,
			utls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			utls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			utls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			utls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			utls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			utls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			utls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
			utls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
			utls.TLS_RSA_WITH_AES_128_GCM_SHA256,
			utls.TLS_RSA_WITH_AES_256_GCM_SHA384,
			utls.TLS_RSA_WITH_AES_128_CBC_SHA,
			utls.TLS_RSA_WITH_AES_256_CBC_SHA,
		},
		Extensions: []utls.TLSExtension{
			&utls.UtlsGREASEExtension{},
			&utls.SNIExtension{},
			&utls.ExtendedMasterSecretExtension{},
			&utls.RenegotiationInfoExtension{Renegotiation: utls.RenegotiateOnceAsClient},
			&utls.SupportedCurvesExtension{Curves: []utls.CurveID{
				utls.GREASE_PLACEHOLDER,
				utls.X25519,
				utls.CurveP256,
				utls.CurveP384,
			}},
			&utls.SupportedPointsExtension{SupportedPoints: []byte{0}},
			&utls.SessionTicketExtension{},
			// Only HTTP/1.1 — no h2 to avoid transport framing issues
			&utls.ALPNExtension{AlpnProtocols: []string{"http/1.1"}},
			&utls.StatusRequestExtension{},
			&utls.SignatureAlgorithmsExtension{SupportedSignatureAlgorithms: []utls.SignatureScheme{
				utls.ECDSAWithP256AndSHA256,
				utls.PSSWithSHA256,
				utls.PKCS1WithSHA256,
				utls.ECDSAWithP384AndSHA384,
				utls.PSSWithSHA384,
				utls.PKCS1WithSHA384,
				utls.PSSWithSHA512,
				utls.PKCS1WithSHA512,
			}},
			&utls.SCTExtension{},
			&utls.SupportedVersionsExtension{Versions: []uint16{
				utls.GREASE_PLACEHOLDER,
				utls.VersionTLS13,
				utls.VersionTLS12,
			}},
			&utls.KeyShareExtension{KeyShares: []utls.KeyShare{
				{Group: utls.CurveID(utls.GREASE_PLACEHOLDER), Data: []byte{0}},
				{Group: utls.X25519},
			}},
			&utls.PSKKeyExchangeModesExtension{Modes: []uint8{utls.PskModeDHE}},
			&utls.UtlsGREASEExtension{},
			&utls.UtlsPaddingExtension{GetPaddingLen: utls.BoringPaddingStyle},
		},
	}

	tlsConn := utls.UClient(conn, &utls.Config{
		ServerName:         host,
		InsecureSkipVerify: false,
	}, utls.HelloCustom)

	if err := tlsConn.ApplyPreset(spec); err != nil {
		// Fallback: use basic Chrome hello if custom spec fails
		log.Printf("[uTLS] Custom spec failed, using HelloChrome_Auto: %v", err)
		conn.Close()
		// Retry with simpler approach
		conn2, err2 := dialer.DialContext(ctx, network, addr)
		if err2 != nil {
			return nil, fmt.Errorf("dial retry failed: %w", err2)
		}
		tlsConn = utls.UClient(conn2, &utls.Config{
			ServerName:         host,
			InsecureSkipVerify: false,
		}, utls.HelloChrome_Auto)
	}

	if err := tlsConn.Handshake(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("uTLS handshake failed: %w", err)
	}

	return tlsConn, nil
}

// buildTransport creates an http.Transport with optional proxy and uTLS support.
func (sc *StealthClient) buildTransport(proxyURL string, useUTLS bool) *http.Transport {
	if useUTLS {
		transport := &http.Transport{
			DialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return utlsDialTLS(ctx, network, addr)
			},
			MaxIdleConns:        100,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
			ForceAttemptHTTP2:   false,
		}
		if proxyURL != "" {
			if parsed, err := url.Parse(proxyURL); err == nil {
				transport.Proxy = http.ProxyURL(parsed)
			}
		}
		return transport
	}

	// Standard TLS transport as fallback
	transport := &http.Transport{
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false,
		},
		ForceAttemptHTTP2: true,
	}
	if proxyURL != "" {
		if parsed, err := url.Parse(proxyURL); err == nil {
			transport.Proxy = http.ProxyURL(parsed)
		}
	}
	return transport
}

// Do executes an HTTP request with stealth features (uTLS, proxy rotation, random UA).
// Falls back to standard TLS if uTLS fails.
func (sc *StealthClient) Do(ctx context.Context, reqURL string) (*http.Response, error) {
	proxy := sc.nextProxy()
	ua := sc.RandomUserAgent()

	// Try uTLS first
	resp, err := sc.doRequest(ctx, reqURL, proxy, ua, true)
	if err != nil {
		// Fallback to standard TLS on certain errors
		errStr := err.Error()
		if strings.Contains(errStr, "malformed") || strings.Contains(errStr, "handshake") || strings.Contains(errStr, "transport connection broken") {
			log.Printf("[Client] uTLS failed for %s, falling back to standard TLS", extractDomain(reqURL))
			resp, err = sc.doRequest(ctx, reqURL, proxy, ua, false)
		}
	}

	return resp, err
}

func (sc *StealthClient) doRequest(ctx context.Context, reqURL, proxy, ua string, useUTLS bool) (*http.Response, error) {
	transport := sc.buildTransport(proxy, useUTLS)

	client := &http.Client{
		Transport: transport,
		Timeout:   sc.timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("client: failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "tr-TR,tr;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("client: request to %s failed (proxy=%s, utls=%v): %w", reqURL, proxy, useUTLS, err)
	}

	return resp, nil
}

// extractDomain returns the domain from a URL for logging.
func extractDomain(rawURL string) string {
	if u, err := url.Parse(rawURL); err == nil {
		return u.Hostname()
	}
	return rawURL
}
