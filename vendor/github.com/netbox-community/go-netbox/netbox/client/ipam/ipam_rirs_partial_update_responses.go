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
	"fmt"
	"io"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"

	"github.com/netbox-community/go-netbox/netbox/models"
)

// IpamRirsPartialUpdateReader is a Reader for the IpamRirsPartialUpdate structure.
type IpamRirsPartialUpdateReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *IpamRirsPartialUpdateReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {
	case 200:
		result := NewIpamRirsPartialUpdateOK()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return result, nil
	default:
		result := NewIpamRirsPartialUpdateDefault(response.Code())
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		if response.Code()/100 == 2 {
			return result, nil
		}
		return nil, result
	}
}

// NewIpamRirsPartialUpdateOK creates a IpamRirsPartialUpdateOK with default headers values
func NewIpamRirsPartialUpdateOK() *IpamRirsPartialUpdateOK {
	return &IpamRirsPartialUpdateOK{}
}

/*IpamRirsPartialUpdateOK handles this case with default header values.

IpamRirsPartialUpdateOK ipam rirs partial update o k
*/
type IpamRirsPartialUpdateOK struct {
	Payload *models.RIR
}

func (o *IpamRirsPartialUpdateOK) Error() string {
	return fmt.Sprintf("[PATCH /ipam/rirs/{id}/][%d] ipamRirsPartialUpdateOK  %+v", 200, o.Payload)
}

func (o *IpamRirsPartialUpdateOK) GetPayload() *models.RIR {
	return o.Payload
}

func (o *IpamRirsPartialUpdateOK) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.RIR)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewIpamRirsPartialUpdateDefault creates a IpamRirsPartialUpdateDefault with default headers values
func NewIpamRirsPartialUpdateDefault(code int) *IpamRirsPartialUpdateDefault {
	return &IpamRirsPartialUpdateDefault{
		_statusCode: code,
	}
}

/*IpamRirsPartialUpdateDefault handles this case with default header values.

IpamRirsPartialUpdateDefault ipam rirs partial update default
*/
type IpamRirsPartialUpdateDefault struct {
	_statusCode int

	Payload interface{}
}

// Code gets the status code for the ipam rirs partial update default response
func (o *IpamRirsPartialUpdateDefault) Code() int {
	return o._statusCode
}

func (o *IpamRirsPartialUpdateDefault) Error() string {
	return fmt.Sprintf("[PATCH /ipam/rirs/{id}/][%d] ipam_rirs_partial_update default  %+v", o._statusCode, o.Payload)
}

func (o *IpamRirsPartialUpdateDefault) GetPayload() interface{} {
	return o.Payload
}

func (o *IpamRirsPartialUpdateDefault) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	// response payload
	if err := consumer.Consume(response.Body(), &o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}
