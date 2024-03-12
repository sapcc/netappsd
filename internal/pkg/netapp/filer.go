package netapp

import (
	"context"
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
	_, err := f.Get(ctx, "/api/storage/aggregates")
	return err
}
