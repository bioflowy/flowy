/*
My API

This is the API

API version: 1.0.0
*/

// Code generated by OpenAPI Generator (https://openapi-generator.tech); DO NOT EDIT.

package api

import (
	"encoding/json"
	"fmt"
)

// ApiGetExectableJobPost200ResponseInnerCommandsInnerInner struct for ApiGetExectableJobPost200ResponseInnerCommandsInnerInner
type ApiGetExectableJobPost200ResponseInnerCommandsInnerInner struct {
	ApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf *ApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf
	ApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1 *ApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1
}

// Unmarshal JSON data into any of the pointers in the struct
func (dst *ApiGetExectableJobPost200ResponseInnerCommandsInnerInner) UnmarshalJSON(data []byte) error {
	var err error
	// try to unmarshal JSON data into ApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf
	err = json.Unmarshal(data, &dst.ApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf);
	if err == nil {
		jsonApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf, _ := json.Marshal(dst.ApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf)
		if string(jsonApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf) == "{}" { // empty struct
			dst.ApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf = nil
		} else {
			return nil // data stored in dst.ApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf, return on the first match
		}
	} else {
		dst.ApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf = nil
	}

	// try to unmarshal JSON data into ApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1
	err = json.Unmarshal(data, &dst.ApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1);
	if err == nil {
		jsonApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1, _ := json.Marshal(dst.ApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1)
		if string(jsonApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1) == "{}" { // empty struct
			dst.ApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1 = nil
		} else {
			return nil // data stored in dst.ApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1, return on the first match
		}
	} else {
		dst.ApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1 = nil
	}

	return fmt.Errorf("data failed to match schemas in anyOf(ApiGetExectableJobPost200ResponseInnerCommandsInnerInner)")
}

// Marshal data from the first non-nil pointers in the struct to JSON
func (src *ApiGetExectableJobPost200ResponseInnerCommandsInnerInner) MarshalJSON() ([]byte, error) {
	if src.ApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf != nil {
		return json.Marshal(&src.ApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf)
	}

	if src.ApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1 != nil {
		return json.Marshal(&src.ApiGetExectableJobPost200ResponseInnerCommandsInnerInnerAnyOf1)
	}

	return nil, nil // no data in anyOf schemas
}

type NullableApiGetExectableJobPost200ResponseInnerCommandsInnerInner struct {
	value *ApiGetExectableJobPost200ResponseInnerCommandsInnerInner
	isSet bool
}

func (v NullableApiGetExectableJobPost200ResponseInnerCommandsInnerInner) Get() *ApiGetExectableJobPost200ResponseInnerCommandsInnerInner {
	return v.value
}

func (v *NullableApiGetExectableJobPost200ResponseInnerCommandsInnerInner) Set(val *ApiGetExectableJobPost200ResponseInnerCommandsInnerInner) {
	v.value = val
	v.isSet = true
}

func (v NullableApiGetExectableJobPost200ResponseInnerCommandsInnerInner) IsSet() bool {
	return v.isSet
}

func (v *NullableApiGetExectableJobPost200ResponseInnerCommandsInnerInner) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableApiGetExectableJobPost200ResponseInnerCommandsInnerInner(val *ApiGetExectableJobPost200ResponseInnerCommandsInnerInner) *NullableApiGetExectableJobPost200ResponseInnerCommandsInnerInner {
	return &NullableApiGetExectableJobPost200ResponseInnerCommandsInnerInner{value: val, isSet: true}
}

func (v NullableApiGetExectableJobPost200ResponseInnerCommandsInnerInner) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableApiGetExectableJobPost200ResponseInnerCommandsInnerInner) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}

