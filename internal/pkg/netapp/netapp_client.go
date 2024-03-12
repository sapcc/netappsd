package netapp

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type RestClient struct {
	client  *http.Client
	options *ClientOptions

	BaseURL   *url.URL
	UserAgent string
}

type ClientOptions struct {
	BasicAuthUser     string
	BasicAuthPassword string
	SSLVerify         bool
	Debug             bool
}

func NewRestClient(host string, options *ClientOptions) *RestClient {
	options = mergeOptions(options)
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: !options.SSLVerify,
			},
		},
	}
	if !strings.HasSuffix(host, "/") {
		host = host + "/"
	}
	if !strings.HasPrefix(host, "http") {
		host = "https://" + host
	}
	baseURL, _ := url.Parse(host)
	return &RestClient{
		BaseURL:   baseURL,
		UserAgent: "netappsd",
		client:    httpClient,
		options:   options,
	}
}

func (c *RestClient) Get(ctx context.Context, uri string) (*http.Response, error) {
	if c.options.Debug {
		fmt.Printf("GET %s\n", uri)
	}
	return c.DoRequest(ctx, uri)
}

func (c *RestClient) DoRequest(ctx context.Context, uri string) (*http.Response, error) {
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
	return resp, fmt.Errorf("HTTP code=%d, HTTP status=\"%s\"", resp.StatusCode, http.StatusText(resp.StatusCode))
}

func mergeOptions(opts *ClientOptions) *ClientOptions {
	defaultOpts := &ClientOptions{
		SSLVerify: false,
		Debug:     false,
	}
	if opts != nil {
		if opts.Debug {
			defaultOpts.Debug = true
		}
		if opts.SSLVerify {
			defaultOpts.SSLVerify = true
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
