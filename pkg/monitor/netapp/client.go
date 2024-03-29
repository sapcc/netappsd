package netapp

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type RestClient struct {
	BaseURL   *url.URL
	UserAgent string
	client    *http.Client
	options   *ClientOptions
	// ResponseTimeout time.Duration
}

type ClientOptions struct {
	BasicAuthUser     string
	BasicAuthPassword string
	SSLVerify         bool
	Debug             bool
	Timeout           time.Duration
}

func UpdateOptions(opts *ClientOptions) *ClientOptions {
	defaultOpts := &ClientOptions{
		SSLVerify: false,
		Debug:     false,
		Timeout:   60 * time.Second,
	}
	if opts != nil {
		if opts.Debug {
			defaultOpts.Debug = true
		}
		if opts.SSLVerify {
			defaultOpts.SSLVerify = true
		}
		if opts.Timeout != 0 {
			defaultOpts.Timeout = opts.Timeout
		}
		if opts.BasicAuthUser != "" {
			defaultOpts.BasicAuthUser = opts.BasicAuthUser
		}
		if opts.BasicAuthPassword != "" {
			defaultOpts.BasicAuthPassword = opts.BasicAuthPassword
		}
	}
	return defaultOpts
}

func NewRestClient(host string, options *ClientOptions) *RestClient {
	options = UpdateOptions(options)
	httpClient := &http.Client{
		Timeout: options.Timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: !options.SSLVerify,
			},
		},
	}
	if !strings.HasSuffix(host, "/") {
		host = host + "/"
	}
	baseURL, _ := url.Parse(host)
	return &RestClient{
		BaseURL:   baseURL,
		UserAgent: "netappsd",
		client:    httpClient,
		options:   options,
	}
}

func (c *RestClient) DoRequest(uri string) (*http.Response, error) {
	url, _ := c.BaseURL.Parse(uri)
	req, err := http.NewRequest("GET", url.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", c.UserAgent)
	if c.options.BasicAuthUser != "" && c.options.BasicAuthPassword != "" {
		req.SetBasicAuth(c.options.BasicAuthUser, c.options.BasicAuthPassword)
	}
	ctx, cncl := context.WithTimeout(context.Background(), c.options.Timeout)
	defer cncl()
	return checkResp(c.client.Do(req.WithContext(ctx)))
}

func checkResp(resp *http.Response, err error) (*http.Response, error) {
	if err != nil {
		return nil, err
	}
	switch resp.StatusCode {
	case 200, 201, 202, 204, 205, 206:
		return resp, nil
	default:
	}
	return resp, fmt.Errorf("Error: HTTP code=%d, HTTP status=\"%s\"", resp.StatusCode, http.StatusText(resp.StatusCode))
}
