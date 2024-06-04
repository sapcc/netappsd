package netapp

import (
	"context"
	"time"
)

type FilerClient struct {
	*RestClient
}

func NewFilerClient(host, username, password string) *FilerClient {
	return &FilerClient{
		NewRestClient(host, &ClientOptions{
			BasicAuthUser:     username,
			BasicAuthPassword: password,
		}),
	}
}

func (f *FilerClient) Probe(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := f.Get(ctx, "/api/storage/aggregates")
	return err
}
