package netapp

import (
	"context"
	"time"
)

type Filer struct {
	*RestClient
}

func NewFiler(host, username, password string) *Filer {
	return &Filer{
		NewRestClient(host, &ClientOptions{
			BasicAuthUser:     username,
			BasicAuthPassword: password,
		}),
	}
}

func (f *Filer) Probe(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := f.Get(ctx, "/api/storage/aggregates")
	return err
}
