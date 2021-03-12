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

package users

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"fmt"
	"io"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"

	"github.com/netbox-community/go-netbox/netbox/models"
)

// UsersGroupsUpdateReader is a Reader for the UsersGroupsUpdate structure.
type UsersGroupsUpdateReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *UsersGroupsUpdateReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {
	case 200:
		result := NewUsersGroupsUpdateOK()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return result, nil
	default:
		result := NewUsersGroupsUpdateDefault(response.Code())
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		if response.Code()/100 == 2 {
			return result, nil
		}
		return nil, result
	}
}

// NewUsersGroupsUpdateOK creates a UsersGroupsUpdateOK with default headers values
func NewUsersGroupsUpdateOK() *UsersGroupsUpdateOK {
	return &UsersGroupsUpdateOK{}
}

/*UsersGroupsUpdateOK handles this case with default header values.

UsersGroupsUpdateOK users groups update o k
*/
type UsersGroupsUpdateOK struct {
	Payload *models.Group
}

func (o *UsersGroupsUpdateOK) Error() string {
	return fmt.Sprintf("[PUT /users/groups/{id}/][%d] usersGroupsUpdateOK  %+v", 200, o.Payload)
}

func (o *UsersGroupsUpdateOK) GetPayload() *models.Group {
	return o.Payload
}

func (o *UsersGroupsUpdateOK) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.Group)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewUsersGroupsUpdateDefault creates a UsersGroupsUpdateDefault with default headers values
func NewUsersGroupsUpdateDefault(code int) *UsersGroupsUpdateDefault {
	return &UsersGroupsUpdateDefault{
		_statusCode: code,
	}
}

/*UsersGroupsUpdateDefault handles this case with default header values.

UsersGroupsUpdateDefault users groups update default
*/
type UsersGroupsUpdateDefault struct {
	_statusCode int

	Payload interface{}
}

// Code gets the status code for the users groups update default response
func (o *UsersGroupsUpdateDefault) Code() int {
	return o._statusCode
}

func (o *UsersGroupsUpdateDefault) Error() string {
	return fmt.Sprintf("[PUT /users/groups/{id}/][%d] users_groups_update default  %+v", o._statusCode, o.Payload)
}

func (o *UsersGroupsUpdateDefault) GetPayload() interface{} {
	return o.Payload
}

func (o *UsersGroupsUpdateDefault) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	// response payload
	if err := consumer.Consume(response.Body(), &o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}
