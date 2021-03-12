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

package dcim

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"fmt"
	"io"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"

	"github.com/netbox-community/go-netbox/netbox/models"
)

// DcimRackGroupsUpdateReader is a Reader for the DcimRackGroupsUpdate structure.
type DcimRackGroupsUpdateReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *DcimRackGroupsUpdateReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {
	case 200:
		result := NewDcimRackGroupsUpdateOK()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return result, nil
	default:
		result := NewDcimRackGroupsUpdateDefault(response.Code())
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		if response.Code()/100 == 2 {
			return result, nil
		}
		return nil, result
	}
}

// NewDcimRackGroupsUpdateOK creates a DcimRackGroupsUpdateOK with default headers values
func NewDcimRackGroupsUpdateOK() *DcimRackGroupsUpdateOK {
	return &DcimRackGroupsUpdateOK{}
}

/*DcimRackGroupsUpdateOK handles this case with default header values.

DcimRackGroupsUpdateOK dcim rack groups update o k
*/
type DcimRackGroupsUpdateOK struct {
	Payload *models.RackGroup
}

func (o *DcimRackGroupsUpdateOK) Error() string {
	return fmt.Sprintf("[PUT /dcim/rack-groups/{id}/][%d] dcimRackGroupsUpdateOK  %+v", 200, o.Payload)
}

func (o *DcimRackGroupsUpdateOK) GetPayload() *models.RackGroup {
	return o.Payload
}

func (o *DcimRackGroupsUpdateOK) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.RackGroup)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewDcimRackGroupsUpdateDefault creates a DcimRackGroupsUpdateDefault with default headers values
func NewDcimRackGroupsUpdateDefault(code int) *DcimRackGroupsUpdateDefault {
	return &DcimRackGroupsUpdateDefault{
		_statusCode: code,
	}
}

/*DcimRackGroupsUpdateDefault handles this case with default header values.

DcimRackGroupsUpdateDefault dcim rack groups update default
*/
type DcimRackGroupsUpdateDefault struct {
	_statusCode int

	Payload interface{}
}

// Code gets the status code for the dcim rack groups update default response
func (o *DcimRackGroupsUpdateDefault) Code() int {
	return o._statusCode
}

func (o *DcimRackGroupsUpdateDefault) Error() string {
	return fmt.Sprintf("[PUT /dcim/rack-groups/{id}/][%d] dcim_rack-groups_update default  %+v", o._statusCode, o.Payload)
}

func (o *DcimRackGroupsUpdateDefault) GetPayload() interface{} {
	return o.Payload
}

func (o *DcimRackGroupsUpdateDefault) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	// response payload
	if err := consumer.Consume(response.Body(), &o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}
