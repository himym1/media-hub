package assrt

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func newAssrtHTTPClient(timeout time.Duration, proxyURL *url.URL) *http.Client {
	direct := assrtBaseTransport(nil)
	mirror := direct.Clone()
	mirror.TLSClientConfig = makedieTLS()
	files := direct
	filesMirror := mirror
	if proxyURL != nil {
		files = assrtBaseTransport(proxyURL)
		filesMirror = files.Clone()
		filesMirror.TLSClientConfig = makedieTLS()
	}
	return &http.Client{
		Timeout: timeout,
		Transport: &assrtTransport{
			primary:     direct,
			mirror:      mirror,
			files:       files,
			filesMirror: filesMirror,
		},
		CheckRedirect: rejectFailedAssrtDownload,
	}
}

func assrtBaseTransport(proxyURL *url.URL) *http.Transport {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if proxyURL != nil {
		transport.Proxy = http.ProxyURL(proxyURL)
	} else {
		transport.Proxy = nil
	}
	transport.DialContext = (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	transport.TLSHandshakeTimeout = 5 * time.Second
	transport.ResponseHeaderTimeout = 8 * time.Second
	return transport
}

func rejectFailedAssrtDownload(request *http.Request, via []*http.Request) error {
	if request.URL != nil && strings.Contains(request.URL.Path, "/download/failed/") {
		return ErrUpstreamResponse
	}
	if len(via) >= 8 {
		return ErrUpstreamResponse
	}
	return nil
}

type assrtTransport struct {
	primary     http.RoundTripper
	mirror      http.RoundTripper
	files       http.RoundTripper
	filesMirror http.RoundTripper
}

func (t *assrtTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if request.URL != nil && (isAssrtFileHost(request.URL.Hostname()) || isAssrtFileHost(assrtHost(request.URL.Hostname()))) {
		return t.roundTripWithFallback(request, firstTripper(t.files, t.primary), firstTripper(t.filesMirror, t.mirror))
	}
	if request.URL != nil && isMakedieHost(request.URL.Hostname()) {
		return firstTripper(t.mirror, t.primary).RoundTrip(request)
	}
	return t.roundTripWithFallback(request, t.primary, t.mirror)
}

func (t *assrtTransport) roundTripWithFallback(request *http.Request, first, second http.RoundTripper) (*http.Response, error) {
	response, err := first.RoundTrip(request)
	if second == nil || second == first || !shouldMirrorAssrt(request, response, err) {
		return response, err
	}
	if response != nil {
		response.Body.Close()
	}
	clone := request.Clone(request.Context())
	rewriteAssrtURLToMakedie(clone.URL)
	clone.Host = clone.URL.Host
	return second.RoundTrip(clone)
}

func firstTripper(preferred, fallback http.RoundTripper) http.RoundTripper {
	if preferred != nil {
		return preferred
	}
	return fallback
}

func shouldMirrorAssrt(request *http.Request, response *http.Response, err error) bool {
	if request.URL == nil || !isAssrtHost(request.URL.Hostname()) {
		return false
	}
	if err != nil {
		return true
	}
	return response != nil && response.StatusCode == http.StatusBadGateway
}

func rewriteAssrtURLToMakedie(endpoint *url.URL) {
	if endpoint == nil || !isAssrtHost(endpoint.Hostname()) {
		return
	}
	host := makedieHost(endpoint.Hostname())
	if port := endpoint.Port(); port != "" && port != "80" && port != "443" {
		endpoint.Host = net.JoinHostPort(host, port)
		return
	}
	endpoint.Host = host
}

func makedieTLS() *tls.Config {
	return &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: true,
		VerifyConnection:   verifyMakedieCertificate,
	}
}

func verifyMakedieCertificate(state tls.ConnectionState) error {
	if len(state.PeerCertificates) == 0 {
		return ErrUpstreamResponse
	}
	verifyHost := assrtHost(state.ServerName)
	if !isAssrtHost(verifyHost) {
		verifyHost = "api.assrt.net"
	}
	intermediates := x509.NewCertPool()
	for _, certificate := range state.PeerCertificates[1:] {
		intermediates.AddCert(certificate)
	}
	_, err := state.PeerCertificates[0].Verify(x509.VerifyOptions{DNSName: verifyHost, Intermediates: intermediates})
	return err
}

func isAssrtHost(host string) bool {
	host = strings.ToLower(host)
	return host == "assrt.net" || strings.HasSuffix(host, ".assrt.net")
}

func isMakedieHost(host string) bool {
	host = strings.ToLower(host)
	return host == "makedie.me" || strings.HasSuffix(host, ".makedie.me")
}

func isAssrtFileHost(host string) bool {
	host = strings.ToLower(host)
	return strings.HasPrefix(host, "file") && (isAssrtHost(host) || isMakedieHost(host))
}

func makedieHost(host string) string {
	host = strings.ToLower(host)
	switch {
	case host == "assrt.net":
		return "makedie.me"
	case strings.HasSuffix(host, ".assrt.net"):
		return strings.TrimSuffix(host, ".assrt.net") + ".makedie.me"
	default:
		return host
	}
}

func assrtHost(host string) string {
	host = strings.ToLower(host)
	switch {
	case host == "makedie.me":
		return "assrt.net"
	case strings.HasSuffix(host, ".makedie.me"):
		return strings.TrimSuffix(host, ".makedie.me") + ".assrt.net"
	default:
		return host
	}
}
