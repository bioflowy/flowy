/*
Flowy Client API

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

// checks if the ApiGetJobInfoGet200Response type satisfies the MappedNullable interface at compile time
var _ MappedNullable = &ApiGetJobInfoGet200Response{}

// ApiGetJobInfoGet200Response struct for ApiGetJobInfoGet200Response
type ApiGetJobInfoGet200Response struct {
	Result interface{} `json:"result,omitempty"`
	Status string `json:"status"`
}

type _ApiGetJobInfoGet200Response ApiGetJobInfoGet200Response

// NewApiGetJobInfoGet200Response instantiates a new ApiGetJobInfoGet200Response object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewApiGetJobInfoGet200Response(status string) *ApiGetJobInfoGet200Response {
	this := ApiGetJobInfoGet200Response{}
	this.Status = status
	return &this
}

// NewApiGetJobInfoGet200ResponseWithDefaults instantiates a new ApiGetJobInfoGet200Response object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewApiGetJobInfoGet200ResponseWithDefaults() *ApiGetJobInfoGet200Response {
	this := ApiGetJobInfoGet200Response{}
	return &this
}

// GetResult returns the Result field value if set, zero value otherwise (both if not set or set to explicit null).
func (o *ApiGetJobInfoGet200Response) GetResult() interface{} {
	if o == nil {
		var ret interface{}
		return ret
	}
	return o.Result
}

// GetResultOk returns a tuple with the Result field value if set, nil otherwise
// and a boolean to check if the value has been set.
// NOTE: If the value is an explicit nil, `nil, true` will be returned
func (o *ApiGetJobInfoGet200Response) GetResultOk() (*interface{}, bool) {
	if o == nil || IsNil(o.Result) {
		return nil, false
	}
	return &o.Result, true
}

// HasResult returns a boolean if a field has been set.
func (o *ApiGetJobInfoGet200Response) HasResult() bool {
	if o != nil && !IsNil(o.Result) {
		return true
	}

	return false
}

// SetResult gets a reference to the given interface{} and assigns it to the Result field.
func (o *ApiGetJobInfoGet200Response) SetResult(v interface{}) {
	o.Result = v
}

// GetStatus returns the Status field value
func (o *ApiGetJobInfoGet200Response) GetStatus() string {
	if o == nil {
		var ret string
		return ret
	}

	return o.Status
}

// GetStatusOk returns a tuple with the Status field value
// and a boolean to check if the value has been set.
func (o *ApiGetJobInfoGet200Response) GetStatusOk() (*string, bool) {
	if o == nil {
		return nil, false
	}
	return &o.Status, true
}

// SetStatus sets field value
func (o *ApiGetJobInfoGet200Response) SetStatus(v string) {
	o.Status = v
}

func (o ApiGetJobInfoGet200Response) MarshalJSON() ([]byte, error) {
	toSerialize,err := o.ToMap()
	if err != nil {
		return []byte{}, err
	}
	return json.Marshal(toSerialize)
}

func (o ApiGetJobInfoGet200Response) ToMap() (map[string]interface{}, error) {
	toSerialize := map[string]interface{}{}
	if o.Result != nil {
		toSerialize["result"] = o.Result
	}
	toSerialize["status"] = o.Status
	return toSerialize, nil
}

func (o *ApiGetJobInfoGet200Response) UnmarshalJSON(data []byte) (err error) {
	// This validates that all required properties are included in the JSON object
	// by unmarshalling the object into a generic map with string keys and checking
	// that every required field exists as a key in the generic map.
	requiredProperties := []string{
		"status",
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

	varApiGetJobInfoGet200Response := _ApiGetJobInfoGet200Response{}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&varApiGetJobInfoGet200Response)

	if err != nil {
		return err
	}

	*o = ApiGetJobInfoGet200Response(varApiGetJobInfoGet200Response)

	return err
}

type NullableApiGetJobInfoGet200Response struct {
	value *ApiGetJobInfoGet200Response
	isSet bool
}

func (v NullableApiGetJobInfoGet200Response) Get() *ApiGetJobInfoGet200Response {
	return v.value
}

func (v *NullableApiGetJobInfoGet200Response) Set(val *ApiGetJobInfoGet200Response) {
	v.value = val
	v.isSet = true
}

func (v NullableApiGetJobInfoGet200Response) IsSet() bool {
	return v.isSet
}

func (v *NullableApiGetJobInfoGet200Response) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableApiGetJobInfoGet200Response(val *ApiGetJobInfoGet200Response) *NullableApiGetJobInfoGet200Response {
	return &NullableApiGetJobInfoGet200Response{value: val, isSet: true}
}

func (v NullableApiGetJobInfoGet200Response) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableApiGetJobInfoGet200Response) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}

