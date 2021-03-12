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

package virtualization

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"fmt"
	"io"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"

	"github.com/netbox-community/go-netbox/netbox/models"
)

// VirtualizationClustersPartialUpdateReader is a Reader for the VirtualizationClustersPartialUpdate structure.
type VirtualizationClustersPartialUpdateReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *VirtualizationClustersPartialUpdateReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {
	case 200:
		result := NewVirtualizationClustersPartialUpdateOK()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return result, nil
	default:
		result := NewVirtualizationClustersPartialUpdateDefault(response.Code())
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		if response.Code()/100 == 2 {
			return result, nil
		}
		return nil, result
	}
}

// NewVirtualizationClustersPartialUpdateOK creates a VirtualizationClustersPartialUpdateOK with default headers values
func NewVirtualizationClustersPartialUpdateOK() *VirtualizationClustersPartialUpdateOK {
	return &VirtualizationClustersPartialUpdateOK{}
}

/*VirtualizationClustersPartialUpdateOK handles this case with default header values.

VirtualizationClustersPartialUpdateOK virtualization clusters partial update o k
*/
type VirtualizationClustersPartialUpdateOK struct {
	Payload *models.Cluster
}

func (o *VirtualizationClustersPartialUpdateOK) Error() string {
	return fmt.Sprintf("[PATCH /virtualization/clusters/{id}/][%d] virtualizationClustersPartialUpdateOK  %+v", 200, o.Payload)
}

func (o *VirtualizationClustersPartialUpdateOK) GetPayload() *models.Cluster {
	return o.Payload
}

func (o *VirtualizationClustersPartialUpdateOK) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.Cluster)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewVirtualizationClustersPartialUpdateDefault creates a VirtualizationClustersPartialUpdateDefault with default headers values
func NewVirtualizationClustersPartialUpdateDefault(code int) *VirtualizationClustersPartialUpdateDefault {
	return &VirtualizationClustersPartialUpdateDefault{
		_statusCode: code,
	}
}

/*VirtualizationClustersPartialUpdateDefault handles this case with default header values.

VirtualizationClustersPartialUpdateDefault virtualization clusters partial update default
*/
type VirtualizationClustersPartialUpdateDefault struct {
	_statusCode int

	Payload interface{}
}

// Code gets the status code for the virtualization clusters partial update default response
func (o *VirtualizationClustersPartialUpdateDefault) Code() int {
	return o._statusCode
}

func (o *VirtualizationClustersPartialUpdateDefault) Error() string {
	return fmt.Sprintf("[PATCH /virtualization/clusters/{id}/][%d] virtualization_clusters_partial_update default  %+v", o._statusCode, o.Payload)
}

func (o *VirtualizationClustersPartialUpdateDefault) GetPayload() interface{} {
	return o.Payload
}

func (o *VirtualizationClustersPartialUpdateDefault) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	// response payload
	if err := consumer.Consume(response.Body(), &o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}
