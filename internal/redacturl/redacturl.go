// Package redacturl strips userinfo from a URL before it is reported.
package redacturl

import (
	"net/url"
	"regexp"
)

var authority = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9+.-]*://|//)?[^/?#@]*@`)

// URL redacts userinfo carried by a scheme://, //, or schemeless user[:pass]@host prefix, leaving everything else — including any other "@" — untouched.
func URL(rawURL string) string {
	if parsed, err := url.Parse(rawURL); err == nil && parsed.User != nil {
		redacted := *parsed
		redacted.User = url.User("redacted")
		return redacted.String()
	}
	return authority.ReplaceAllString(rawURL, "${1}redacted@")
}

// Error returns a copy of err with a bare *url.Error's URL field redacted, or err itself unchanged otherwise.
func Error(err error) error {
	urlErr, ok := err.(*url.Error)
	if !ok {
		return err
	}
	redacted := *urlErr
	redacted.URL = URL(urlErr.URL)
	return &redacted
}
