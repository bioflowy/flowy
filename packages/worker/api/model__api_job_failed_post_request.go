/*
My API

This is the API

API version: 1.0.0
*/

// Code generated by OpenAPI Generator (https://openapi-generator.tech); DO NOT EDIT.

package api

import (
	"encoding/json"
	"bytes"
	"fmt"
)

// checks if the ApiJobFailedPostRequest type satisfies the MappedNullable interface at compile time
var _ MappedNullable = &ApiJobFailedPostRequest{}

// ApiJobFailedPostRequest struct for ApiJobFailedPostRequest
type ApiJobFailedPostRequest struct {
	Id string `json:"id"`
	ErrorMsg string `json:"errorMsg"`
}

type _ApiJobFailedPostRequest ApiJobFailedPostRequest

// NewApiJobFailedPostRequest instantiates a new ApiJobFailedPostRequest object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewApiJobFailedPostRequest(id string, errorMsg string) *ApiJobFailedPostRequest {
	this := ApiJobFailedPostRequest{}
	this.Id = id
	this.ErrorMsg = errorMsg
	return &this
}

// NewApiJobFailedPostRequestWithDefaults instantiates a new ApiJobFailedPostRequest object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewApiJobFailedPostRequestWithDefaults() *ApiJobFailedPostRequest {
	this := ApiJobFailedPostRequest{}
	return &this
}

// GetId returns the Id field value
func (o *ApiJobFailedPostRequest) GetId() string {
	if o == nil {
		var ret string
		return ret
	}

	return o.Id
}

// GetIdOk returns a tuple with the Id field value
// and a boolean to check if the value has been set.
func (o *ApiJobFailedPostRequest) GetIdOk() (*string, bool) {
	if o == nil {
		return nil, false
	}
	return &o.Id, true
}

// SetId sets field value
func (o *ApiJobFailedPostRequest) SetId(v string) {
	o.Id = v
}

// GetErrorMsg returns the ErrorMsg field value
func (o *ApiJobFailedPostRequest) GetErrorMsg() string {
	if o == nil {
		var ret string
		return ret
	}

	return o.ErrorMsg
}

// GetErrorMsgOk returns a tuple with the ErrorMsg field value
// and a boolean to check if the value has been set.
func (o *ApiJobFailedPostRequest) GetErrorMsgOk() (*string, bool) {
	if o == nil {
		return nil, false
	}
	return &o.ErrorMsg, true
}

// SetErrorMsg sets field value
func (o *ApiJobFailedPostRequest) SetErrorMsg(v string) {
	o.ErrorMsg = v
}

func (o ApiJobFailedPostRequest) MarshalJSON() ([]byte, error) {
	toSerialize,err := o.ToMap()
	if err != nil {
		return []byte{}, err
	}
	return json.Marshal(toSerialize)
}

func (o ApiJobFailedPostRequest) ToMap() (map[string]interface{}, error) {
	toSerialize := map[string]interface{}{}
	toSerialize["id"] = o.Id
	toSerialize["errorMsg"] = o.ErrorMsg
	return toSerialize, nil
}

func (o *ApiJobFailedPostRequest) UnmarshalJSON(data []byte) (err error) {
	// This validates that all required properties are included in the JSON object
	// by unmarshalling the object into a generic map with string keys and checking
	// that every required field exists as a key in the generic map.
	requiredProperties := []string{
		"id",
		"errorMsg",
	}

	allProperties := make(map[string]interface{})

	err = json.Unmarshal(data, &allProperties)

	if err != nil {
		return err;
	}

	for _, requiredProperty := range(requiredProperties) {
		if _, exists := allProperties[requiredProperty]; !exists {
			return fmt.Errorf("no value given for required property %v", requiredProperty)
		}
	}

	varApiJobFailedPostRequest := _ApiJobFailedPostRequest{}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&varApiJobFailedPostRequest)

	if err != nil {
		return err
	}

	*o = ApiJobFailedPostRequest(varApiJobFailedPostRequest)

	return err
}

type NullableApiJobFailedPostRequest struct {
	value *ApiJobFailedPostRequest
	isSet bool
}

func (v NullableApiJobFailedPostRequest) Get() *ApiJobFailedPostRequest {
	return v.value
}

func (v *NullableApiJobFailedPostRequest) Set(val *ApiJobFailedPostRequest) {
	v.value = val
	v.isSet = true
}

func (v NullableApiJobFailedPostRequest) IsSet() bool {
	return v.isSet
}

func (v *NullableApiJobFailedPostRequest) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableApiJobFailedPostRequest(val *ApiJobFailedPostRequest) *NullableApiJobFailedPostRequest {
	return &NullableApiJobFailedPostRequest{value: val, isSet: true}
}

func (v NullableApiJobFailedPostRequest) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableApiJobFailedPostRequest) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}

