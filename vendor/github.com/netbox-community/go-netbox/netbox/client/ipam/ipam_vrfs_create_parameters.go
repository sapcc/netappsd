// Code generated by go-swagger; DO NOT EDIT.

// Copyright 2020 The go-netbox Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package ipam

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"context"
	"net/http"
	"time"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/runtime"
	cr "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"

	"github.com/netbox-community/go-netbox/netbox/models"
)

// NewIpamVrfsCreateParams creates a new IpamVrfsCreateParams object
// with the default values initialized.
func NewIpamVrfsCreateParams() *IpamVrfsCreateParams {
	var ()
	return &IpamVrfsCreateParams{

		timeout: cr.DefaultTimeout,
	}
}

// NewIpamVrfsCreateParamsWithTimeout creates a new IpamVrfsCreateParams object
// with the default values initialized, and the ability to set a timeout on a request
func NewIpamVrfsCreateParamsWithTimeout(timeout time.Duration) *IpamVrfsCreateParams {
	var ()
	return &IpamVrfsCreateParams{

		timeout: timeout,
	}
}

// NewIpamVrfsCreateParamsWithContext creates a new IpamVrfsCreateParams object
// with the default values initialized, and the ability to set a context for a request
func NewIpamVrfsCreateParamsWithContext(ctx context.Context) *IpamVrfsCreateParams {
	var ()
	return &IpamVrfsCreateParams{

		Context: ctx,
	}
}

// NewIpamVrfsCreateParamsWithHTTPClient creates a new IpamVrfsCreateParams object
// with the default values initialized, and the ability to set a custom HTTPClient for a request
func NewIpamVrfsCreateParamsWithHTTPClient(client *http.Client) *IpamVrfsCreateParams {
	var ()
	return &IpamVrfsCreateParams{
		HTTPClient: client,
	}
}

/*IpamVrfsCreateParams contains all the parameters to send to the API endpoint
for the ipam vrfs create operation typically these are written to a http.Request
*/
type IpamVrfsCreateParams struct {

	/*Data*/
	Data *models.WritableVRF

	timeout    time.Duration
	Context    context.Context
	HTTPClient *http.Client
}

// WithTimeout adds the timeout to the ipam vrfs create params
func (o *IpamVrfsCreateParams) WithTimeout(timeout time.Duration) *IpamVrfsCreateParams {
	o.SetTimeout(timeout)
	return o
}

// SetTimeout adds the timeout to the ipam vrfs create params
func (o *IpamVrfsCreateParams) SetTimeout(timeout time.Duration) {
	o.timeout = timeout
}

// WithContext adds the context to the ipam vrfs create params
func (o *IpamVrfsCreateParams) WithContext(ctx context.Context) *IpamVrfsCreateParams {
	o.SetContext(ctx)
	return o
}

// SetContext adds the context to the ipam vrfs create params
func (o *IpamVrfsCreateParams) SetContext(ctx context.Context) {
	o.Context = ctx
}

// WithHTTPClient adds the HTTPClient to the ipam vrfs create params
func (o *IpamVrfsCreateParams) WithHTTPClient(client *http.Client) *IpamVrfsCreateParams {
	o.SetHTTPClient(client)
	return o
}

// SetHTTPClient adds the HTTPClient to the ipam vrfs create params
func (o *IpamVrfsCreateParams) SetHTTPClient(client *http.Client) {
	o.HTTPClient = client
}

// WithData adds the data to the ipam vrfs create params
func (o *IpamVrfsCreateParams) WithData(data *models.WritableVRF) *IpamVrfsCreateParams {
	o.SetData(data)
	return o
}

// SetData adds the data to the ipam vrfs create params
func (o *IpamVrfsCreateParams) SetData(data *models.WritableVRF) {
	o.Data = data
}

// WriteToRequest writes these params to a swagger request
func (o *IpamVrfsCreateParams) WriteToRequest(r runtime.ClientRequest, reg strfmt.Registry) error {

	if err := r.SetTimeout(o.timeout); err != nil {
		return err
	}
	var res []error

	if o.Data != nil {
		if err := r.SetBodyParam(o.Data); err != nil {
			return err
		}
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}