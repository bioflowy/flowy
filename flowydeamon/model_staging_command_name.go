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

// StagingCommandName the model 'StagingCommandName'
type StagingCommandName string

// List of StagingCommandName
const (
	WRITE_FILE_CONTENT StagingCommandName = "writeFileContent"
	RELINK StagingCommandName = "relink"
	SYMLINK StagingCommandName = "symlink"
	COPY StagingCommandName = "copy"
	MKDIR StagingCommandName = "mkdir"
)

// All allowed values of StagingCommandName enum
var AllowedStagingCommandNameEnumValues = []StagingCommandName{
	"writeFileContent",
	"relink",
	"symlink",
	"copy",
	"mkdir",
}

func (v *StagingCommandName) UnmarshalJSON(src []byte) error {
	var value string
	err := json.Unmarshal(src, &value)
	if err != nil {
		return err
	}
	enumTypeValue := StagingCommandName(value)
	for _, existing := range AllowedStagingCommandNameEnumValues {
		if existing == enumTypeValue {
			*v = enumTypeValue
			return nil
		}
	}

	return fmt.Errorf("%+v is not a valid StagingCommandName", value)
}

// NewStagingCommandNameFromValue returns a pointer to a valid StagingCommandName
// for the value passed as argument, or an error if the value passed is not allowed by the enum
func NewStagingCommandNameFromValue(v string) (*StagingCommandName, error) {
	ev := StagingCommandName(v)
	if ev.IsValid() {
		return &ev, nil
	} else {
		return nil, fmt.Errorf("invalid value '%v' for StagingCommandName: valid values are %v", v, AllowedStagingCommandNameEnumValues)
	}
}

// IsValid return true if the value is valid for the enum, false otherwise
func (v StagingCommandName) IsValid() bool {
	for _, existing := range AllowedStagingCommandNameEnumValues {
		if existing == v {
			return true
		}
	}
	return false
}

// Ptr returns reference to StagingCommandName value
func (v StagingCommandName) Ptr() *StagingCommandName {
	return &v
}

type NullableStagingCommandName struct {
	value *StagingCommandName
	isSet bool
}

func (v NullableStagingCommandName) Get() *StagingCommandName {
	return v.value
}

func (v *NullableStagingCommandName) Set(val *StagingCommandName) {
	v.value = val
	v.isSet = true
}

func (v NullableStagingCommandName) IsSet() bool {
	return v.isSet
}

func (v *NullableStagingCommandName) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableStagingCommandName(val *StagingCommandName) *NullableStagingCommandName {
	return &NullableStagingCommandName{value: val, isSet: true}
}

func (v NullableStagingCommandName) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableStagingCommandName) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}

