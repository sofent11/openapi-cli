package openapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
)

const hidden = "[REDACTED]"

type mask struct {
	header  string
	secrets []string
}

func sensitive(key string) bool {
	key = strings.ToLower(strings.NewReplacer("-", "", "_", "", ".", "").Replace(key))
	switch key {
	case "token", "authorization", "proxyauthorization", "cookie", "setcookie", "signature", "xamzsignature", "xgoogsignature", "sig", "key", "passwd", "pwd", "credential", "credentials", "auth":
		return true
	}
	for _, suffix := range []string{"password", "secret", "secretkey", "apikey", "authtoken", "accesstoken", "refreshtoken", "sessiontoken", "securitytoken", "idtoken", "privatekey"} {
		if strings.HasSuffix(key, suffix) {
			return true
		}
	}
	return false
}

func newMask(req *http.Request, headers ...string) mask {
	var m mask
	if len(headers) > 0 {
		m.header = headers[0]
	}
	for k, vs := range req.Header {
		if sensitive(k) || strings.EqualFold(k, m.header) {
			for _, v := range vs {
				m.secrets = append(m.secrets, v)
				if _, value, ok := strings.Cut(v, " "); ok {
					m.secrets = append(m.secrets, value)
				}
			}
		}
	}
	return m
}

func (m mask) text(s string) string {
	for _, secret := range m.secrets {
		if secret != "" {
			s = strings.ReplaceAll(s, secret, hidden)
		}
	}
	return s
}
func (m mask) field(key, value string) string {
	if sensitive(key) || strings.EqualFold(key, m.header) {
		return hidden
	}
	return m.text(value)
}
func (m mask) url(u *url.URL) string {
	copy := *u
	if copy.User != nil {
		copy.User = url.User(hidden)
	}
	q := copy.Query()
	for k, vs := range q {
		for i, v := range vs {
			vs[i] = m.field(k, v)
		}
	}
	copy.RawQuery = q.Encode()
	return m.text(copy.String())
}
func (m mask) jsonValue(v any) any {
	switch val := v.(type) {
	case map[string]any:
		for key, child := range val {
			if sensitive(key) || strings.EqualFold(key, m.header) {
				val[key] = hidden
			} else {
				val[key] = m.jsonValue(child)
			}
		}
	case []any:
		for i, child := range val {
			val[i] = m.jsonValue(child)
		}
	case string:
		return m.text(val)
	}
	return v
}
func (m mask) body(data []byte) []byte {
	if !json.Valid(data) {
		return []byte(m.text(string(data)))
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return []byte(hidden)
	}
	out, _ := json.Marshal(m.jsonValue(v))
	return out
}
func RedactResult(r *CallResult, req *http.Request, authHeader string) {
	m := newMask(req, authHeader)
	for k, vs := range r.Headers {
		for i, v := range vs {
			vs[i] = m.field(k, v)
		}
	}
	r.Body = m.body(r.Body)
	r.Status = m.text(r.Status)
}
func DisplayConfig(cfg Config) Config {
	m := mask{}
	if cfg.AuthToken != "" {
		m.secrets = []string{cfg.AuthToken}
		cfg.AuthToken = hidden
	}
	for _, p := range []*string{&cfg.BaseURL, &cfg.UpdateURL} {
		if u, err := url.Parse(*p); err == nil {
			*p = m.url(u)
		} else {
			*p = hidden
		}
	}
	return cfg
}
