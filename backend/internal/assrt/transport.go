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

func newAssrtHTTPClient(timeout time.Duration) *http.Client {
	primary := http.DefaultTransport.(*http.Transport).Clone()
	mirror := primary.Clone()
	mirror.TLSClientConfig = makedieTLS()
	return &http.Client{Timeout: timeout, Transport: &assrtTransport{primary: primary, mirror: mirror}}
}

type assrtTransport struct {
	primary http.RoundTripper
	mirror  http.RoundTripper
}

func (t *assrtTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if isMakedieHost(request.URL.Hostname()) {
		return t.mirror.RoundTrip(request)
	}
	response, err := t.primary.RoundTrip(request)
	if !shouldMirrorAssrt(request, response, err) {
		return response, err
	}
	if response != nil {
		response.Body.Close()
	}
	clone := request.Clone(request.Context())
	rewriteAssrtURLToMakedie(clone.URL)
	clone.Host = clone.URL.Host
	return t.mirror.RoundTrip(clone)
}

func shouldMirrorAssrt(request *http.Request, response *http.Response, err error) bool {
	if !isAssrtHost(request.URL.Hostname()) {
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
