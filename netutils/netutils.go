package netutils

import (
	"net/http"
	"net/url"
)

type ProxyWriter struct {
	W    http.ResponseWriter
	Code int
}

func (p *ProxyWriter) Header() http.Header {
	return p.W.Header()
}

func (p *ProxyWriter) Write(buf []byte) (int, error) {
	return p.W.Write(buf)
}

func (p *ProxyWriter) WriteHeader(code int) {
	p.Code = code
	p.WriteHeader(code)
}

// CopyURL provides update safe copy by avoiding shallow copying User field
func CopyURL(i *url.URL) *url.URL {
	out := *i
	if i.User != nil {
		out.User = &(*i.User)
	}
	return &out
}

// CopyHeaders copies http headers from source to destination, it
// does not overide, but adds multiple headers
func CopyHeaders(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

// HasHeaders determines whether any of the header names is present in the http headers
func HasHeaders(names []string, headers http.Header) bool {
	for _, h := range names {
		if headers.Get(h) != "" {
			return true
		}
	}
	return false
}

// RemoveHeaders removes the header with the given names from the headers map
func RemoveHeaders(headers http.Header, names ...string) {
	for _, h := range names {
		headers.Del(h)
	}
}