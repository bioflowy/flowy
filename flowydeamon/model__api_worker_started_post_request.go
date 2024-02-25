/*
My API

This is the API

API version: 1.0.0
*/

// Code generated by OpenAPI Generator (https://openapi-generator.tech); DO NOT EDIT.

package main

import (
	"encoding/json"
	"fmt"
)

// checks if the ApiWorkerStartedPostRequest type satisfies the MappedNullable interface at compile time
var _ MappedNullable = &ApiWorkerStartedPostRequest{}

// ApiWorkerStartedPostRequest struct for ApiWorkerStartedPostRequest
type ApiWorkerStartedPostRequest struct {
	Hostname string `json:"hostname"`
	Cpu int32 `json:"cpu"`
	// memory in MB
	Memory int32 `json:"memory"`
}

type _ApiWorkerStartedPostRequest ApiWorkerStartedPostRequest

// NewApiWorkerStartedPostRequest instantiates a new ApiWorkerStartedPostRequest object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewApiWorkerStartedPostRequest(hostname string, cpu int32, memory int32) *ApiWorkerStartedPostRequest {
	this := ApiWorkerStartedPostRequest{}
	this.Hostname = hostname
	this.Cpu = cpu
	this.Memory = memory
	return &this
}

// NewApiWorkerStartedPostRequestWithDefaults instantiates a new ApiWorkerStartedPostRequest object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewApiWorkerStartedPostRequestWithDefaults() *ApiWorkerStartedPostRequest {
	this := ApiWorkerStartedPostRequest{}
	return &this
}

// GetHostname returns the Hostname field value
func (o *ApiWorkerStartedPostRequest) GetHostname() string {
	if o == nil {
		var ret string
		return ret
	}

	return o.Hostname
}

// GetHostnameOk returns a tuple with the Hostname field value
// and a boolean to check if the value has been set.
func (o *ApiWorkerStartedPostRequest) GetHostnameOk() (*string, bool) {
	if o == nil {
		return nil, false
	}
	return &o.Hostname, true
}

// SetHostname sets field value
func (o *ApiWorkerStartedPostRequest) SetHostname(v string) {
	o.Hostname = v
}

// GetCpu returns the Cpu field value
func (o *ApiWorkerStartedPostRequest) GetCpu() int32 {
	if o == nil {
		var ret int32
		return ret
	}

	return o.Cpu
}

// GetCpuOk returns a tuple with the Cpu field value
// and a boolean to check if the value has been set.
func (o *ApiWorkerStartedPostRequest) GetCpuOk() (*int32, bool) {
	if o == nil {
		return nil, false
	}
	return &o.Cpu, true
}

// SetCpu sets field value
func (o *ApiWorkerStartedPostRequest) SetCpu(v int32) {
	o.Cpu = v
}

// GetMemory returns the Memory field value
func (o *ApiWorkerStartedPostRequest) GetMemory() int32 {
	if o == nil {
		var ret int32
		return ret
	}

	return o.Memory
}

// GetMemoryOk returns a tuple with the Memory field value
// and a boolean to check if the value has been set.
func (o *ApiWorkerStartedPostRequest) GetMemoryOk() (*int32, bool) {
	if o == nil {
		return nil, false
	}
	return &o.Memory, true
}

// SetMemory sets field value
func (o *ApiWorkerStartedPostRequest) SetMemory(v int32) {
	o.Memory = v
}

func (o ApiWorkerStartedPostRequest) MarshalJSON() ([]byte, error) {
	toSerialize,err := o.ToMap()
	if err != nil {
		return []byte{}, err
	}
	return json.Marshal(toSerialize)
}

func (o ApiWorkerStartedPostRequest) ToMap() (map[string]interface{}, error) {
	toSerialize := map[string]interface{}{}
	toSerialize["hostname"] = o.Hostname
	toSerialize["cpu"] = o.Cpu
	toSerialize["memory"] = o.Memory
	return toSerialize, nil
}

func (o *ApiWorkerStartedPostRequest) UnmarshalJSON(bytes []byte) (err error) {
    // This validates that all required properties are included in the JSON object
	// by unmarshalling the object into a generic map with string keys and checking
	// that every required field exists as a key in the generic map.
	requiredProperties := []string{
		"hostname",
		"cpu",
		"memory",
	}

	allProperties := make(map[string]interface{})

	err = json.Unmarshal(bytes, &allProperties)

	if err != nil {
		return err;
	}

	for _, requiredProperty := range(requiredProperties) {
		if _, exists := allProperties[requiredProperty]; !exists {
			return fmt.Errorf("no value given for required property %v", requiredProperty)
		}
	}

	varApiWorkerStartedPostRequest := _ApiWorkerStartedPostRequest{}

	err = json.Unmarshal(bytes, &varApiWorkerStartedPostRequest)

	if err != nil {
		return err
	}

	*o = ApiWorkerStartedPostRequest(varApiWorkerStartedPostRequest)

	return err
}

type NullableApiWorkerStartedPostRequest struct {
	value *ApiWorkerStartedPostRequest
	isSet bool
}

func (v NullableApiWorkerStartedPostRequest) Get() *ApiWorkerStartedPostRequest {
	return v.value
}

func (v *NullableApiWorkerStartedPostRequest) Set(val *ApiWorkerStartedPostRequest) {
	v.value = val
	v.isSet = true
}

func (v NullableApiWorkerStartedPostRequest) IsSet() bool {
	return v.isSet
}

func (v *NullableApiWorkerStartedPostRequest) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableApiWorkerStartedPostRequest(val *ApiWorkerStartedPostRequest) *NullableApiWorkerStartedPostRequest {
	return &NullableApiWorkerStartedPostRequest{value: val, isSet: true}
}

func (v NullableApiWorkerStartedPostRequest) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableApiWorkerStartedPostRequest) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}


