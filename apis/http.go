package apis

import (
	"io"
	"net/http"
)

type HTTPCredentials struct {
	BaseURL  string `yaml:"url"`
	Username string
	Password string
}

func newRequest(method, path string, body io.Reader, credentials HTTPCredentials) (*http.Request, error) {
	req, err := http.NewRequest(method, credentials.BaseURL+path, body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if credentials.Username != "" && credentials.Password != "" {
		req.SetBasicAuth(credentials.Username, credentials.Password)
	}
	return req, nil
}
