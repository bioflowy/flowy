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

// checks if the ApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1 type satisfies the MappedNullable interface at compile time
var _ MappedNullable = &ApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1{}

// ApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1 struct for ApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1
type ApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1 struct {
	Type string `json:"type"`
	Key string `json:"key"`
}

type _ApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1 ApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1

// NewApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1 instantiates a new ApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1 object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1(type_ string, key string) *ApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1 {
	this := ApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1{}
	this.Type = type_
	this.Key = key
	return &this
}

// NewApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1WithDefaults instantiates a new ApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1 object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1WithDefaults() *ApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1 {
	this := ApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1{}
	return &this
}

// GetType returns the Type field value
func (o *ApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1) GetType() string {
	if o == nil {
		var ret string
		return ret
	}

	return o.Type
}

// GetTypeOk returns a tuple with the Type field value
// and a boolean to check if the value has been set.
func (o *ApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1) GetTypeOk() (*string, bool) {
	if o == nil {
		return nil, false
	}
	return &o.Type, true
}

// SetType sets field value
func (o *ApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1) SetType(v string) {
	o.Type = v
}

// GetKey returns the Key field value
func (o *ApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1) GetKey() string {
	if o == nil {
		var ret string
		return ret
	}

	return o.Key
}

// GetKeyOk returns a tuple with the Key field value
// and a boolean to check if the value has been set.
func (o *ApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1) GetKeyOk() (*string, bool) {
	if o == nil {
		return nil, false
	}
	return &o.Key, true
}

// SetKey sets field value
func (o *ApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1) SetKey(v string) {
	o.Key = v
}

func (o ApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1) MarshalJSON() ([]byte, error) {
	toSerialize,err := o.ToMap()
	if err != nil {
		return []byte{}, err
	}
	return json.Marshal(toSerialize)
}

func (o ApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1) ToMap() (map[string]interface{}, error) {
	toSerialize := map[string]interface{}{}
	toSerialize["type"] = o.Type
	toSerialize["key"] = o.Key
	return toSerialize, nil
}

func (o *ApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1) UnmarshalJSON(data []byte) (err error) {
	// This validates that all required properties are included in the JSON object
	// by unmarshalling the object into a generic map with string keys and checking
	// that every required field exists as a key in the generic map.
	requiredProperties := []string{
		"type",
		"key",
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

	varApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1 := _ApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1{}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&varApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1)

	if err != nil {
		return err
	}

	*o = ApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1(varApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1)

	return err
}

type NullableApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1 struct {
	value *ApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1
	isSet bool
}

func (v NullableApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1) Get() *ApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1 {
	return v.value
}

func (v *NullableApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1) Set(val *ApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1) {
	v.value = val
	v.isSet = true
}

func (v NullableApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1) IsSet() bool {
	return v.isSet
}

func (v *NullableApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1(val *ApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1) *NullableApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1 {
	return &NullableApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1{value: val, isSet: true}
}

func (v NullableApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}


