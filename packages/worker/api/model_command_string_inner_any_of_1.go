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

// checks if the CommandStringInnerAnyOf1 type satisfies the MappedNullable interface at compile time
var _ MappedNullable = &CommandStringInnerAnyOf1{}

// CommandStringInnerAnyOf1 struct for CommandStringInnerAnyOf1
type CommandStringInnerAnyOf1 struct {
	Type string `json:"type"`
	Value string `json:"value"`
}

type _CommandStringInnerAnyOf1 CommandStringInnerAnyOf1

// NewCommandStringInnerAnyOf1 instantiates a new CommandStringInnerAnyOf1 object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewCommandStringInnerAnyOf1(type_ string, value string) *CommandStringInnerAnyOf1 {
	this := CommandStringInnerAnyOf1{}
	this.Type = type_
	this.Value = value
	return &this
}

// NewCommandStringInnerAnyOf1WithDefaults instantiates a new CommandStringInnerAnyOf1 object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewCommandStringInnerAnyOf1WithDefaults() *CommandStringInnerAnyOf1 {
	this := CommandStringInnerAnyOf1{}
	return &this
}

// GetType returns the Type field value
func (o *CommandStringInnerAnyOf1) GetType() string {
	if o == nil {
		var ret string
		return ret
	}

	return o.Type
}

// GetTypeOk returns a tuple with the Type field value
// and a boolean to check if the value has been set.
func (o *CommandStringInnerAnyOf1) GetTypeOk() (*string, bool) {
	if o == nil {
		return nil, false
	}
	return &o.Type, true
}

// SetType sets field value
func (o *CommandStringInnerAnyOf1) SetType(v string) {
	o.Type = v
}

// GetValue returns the Value field value
func (o *CommandStringInnerAnyOf1) GetValue() string {
	if o == nil {
		var ret string
		return ret
	}

	return o.Value
}

// GetValueOk returns a tuple with the Value field value
// and a boolean to check if the value has been set.
func (o *CommandStringInnerAnyOf1) GetValueOk() (*string, bool) {
	if o == nil {
		return nil, false
	}
	return &o.Value, true
}

// SetValue sets field value
func (o *CommandStringInnerAnyOf1) SetValue(v string) {
	o.Value = v
}

func (o CommandStringInnerAnyOf1) MarshalJSON() ([]byte, error) {
	toSerialize,err := o.ToMap()
	if err != nil {
		return []byte{}, err
	}
	return json.Marshal(toSerialize)
}

func (o CommandStringInnerAnyOf1) ToMap() (map[string]interface{}, error) {
	toSerialize := map[string]interface{}{}
	toSerialize["type"] = o.Type
	toSerialize["value"] = o.Value
	return toSerialize, nil
}

func (o *CommandStringInnerAnyOf1) UnmarshalJSON(data []byte) (err error) {
	// This validates that all required properties are included in the JSON object
	// by unmarshalling the object into a generic map with string keys and checking
	// that every required field exists as a key in the generic map.
	requiredProperties := []string{
		"type",
		"value",
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

	varCommandStringInnerAnyOf1 := _CommandStringInnerAnyOf1{}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&varCommandStringInnerAnyOf1)

	if err != nil {
		return err
	}

	*o = CommandStringInnerAnyOf1(varCommandStringInnerAnyOf1)

	return err
}

type NullableCommandStringInnerAnyOf1 struct {
	value *CommandStringInnerAnyOf1
	isSet bool
}

func (v NullableCommandStringInnerAnyOf1) Get() *CommandStringInnerAnyOf1 {
	return v.value
}

func (v *NullableCommandStringInnerAnyOf1) Set(val *CommandStringInnerAnyOf1) {
	v.value = val
	v.isSet = true
}

func (v NullableCommandStringInnerAnyOf1) IsSet() bool {
	return v.isSet
}

func (v *NullableCommandStringInnerAnyOf1) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableCommandStringInnerAnyOf1(val *CommandStringInnerAnyOf1) *NullableCommandStringInnerAnyOf1 {
	return &NullableCommandStringInnerAnyOf1{value: val, isSet: true}
}

func (v NullableCommandStringInnerAnyOf1) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableCommandStringInnerAnyOf1) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}

