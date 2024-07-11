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

// checks if the OutputBinding type satisfies the MappedNullable interface at compile time
var _ MappedNullable = &OutputBinding{}

// OutputBinding struct for OutputBinding
type OutputBinding struct {
	Name string `json:"name"`
	SecondaryFiles []OutputBindingSecondaryFilesInner `json:"secondaryFiles"`
	LoadContents *bool `json:"loadContents,omitempty"`
	LoadListing *LoadListingEnum `json:"loadListing,omitempty"`
	Glob []string `json:"glob,omitempty"`
	OutputEval *string `json:"outputEval,omitempty"`
	Streamable *bool `json:"streamable,omitempty"`
}

type _OutputBinding OutputBinding

// NewOutputBinding instantiates a new OutputBinding object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewOutputBinding(name string, secondaryFiles []OutputBindingSecondaryFilesInner) *OutputBinding {
	this := OutputBinding{}
	this.Name = name
	this.SecondaryFiles = secondaryFiles
	var streamable bool = false
	this.Streamable = &streamable
	return &this
}

// NewOutputBindingWithDefaults instantiates a new OutputBinding object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewOutputBindingWithDefaults() *OutputBinding {
	this := OutputBinding{}
	var streamable bool = false
	this.Streamable = &streamable
	return &this
}

// GetName returns the Name field value
func (o *OutputBinding) GetName() string {
	if o == nil {
		var ret string
		return ret
	}

	return o.Name
}

// GetNameOk returns a tuple with the Name field value
// and a boolean to check if the value has been set.
func (o *OutputBinding) GetNameOk() (*string, bool) {
	if o == nil {
		return nil, false
	}
	return &o.Name, true
}

// SetName sets field value
func (o *OutputBinding) SetName(v string) {
	o.Name = v
}

// GetSecondaryFiles returns the SecondaryFiles field value
func (o *OutputBinding) GetSecondaryFiles() []OutputBindingSecondaryFilesInner {
	if o == nil {
		var ret []OutputBindingSecondaryFilesInner
		return ret
	}

	return o.SecondaryFiles
}

// GetSecondaryFilesOk returns a tuple with the SecondaryFiles field value
// and a boolean to check if the value has been set.
func (o *OutputBinding) GetSecondaryFilesOk() ([]OutputBindingSecondaryFilesInner, bool) {
	if o == nil {
		return nil, false
	}
	return o.SecondaryFiles, true
}

// SetSecondaryFiles sets field value
func (o *OutputBinding) SetSecondaryFiles(v []OutputBindingSecondaryFilesInner) {
	o.SecondaryFiles = v
}

// GetLoadContents returns the LoadContents field value if set, zero value otherwise.
func (o *OutputBinding) GetLoadContents() bool {
	if o == nil || IsNil(o.LoadContents) {
		var ret bool
		return ret
	}
	return *o.LoadContents
}

// GetLoadContentsOk returns a tuple with the LoadContents field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *OutputBinding) GetLoadContentsOk() (*bool, bool) {
	if o == nil || IsNil(o.LoadContents) {
		return nil, false
	}
	return o.LoadContents, true
}

// HasLoadContents returns a boolean if a field has been set.
func (o *OutputBinding) HasLoadContents() bool {
	if o != nil && !IsNil(o.LoadContents) {
		return true
	}

	return false
}

// SetLoadContents gets a reference to the given bool and assigns it to the LoadContents field.
func (o *OutputBinding) SetLoadContents(v bool) {
	o.LoadContents = &v
}

// GetLoadListing returns the LoadListing field value if set, zero value otherwise.
func (o *OutputBinding) GetLoadListing() LoadListingEnum {
	if o == nil || IsNil(o.LoadListing) {
		var ret LoadListingEnum
		return ret
	}
	return *o.LoadListing
}

// GetLoadListingOk returns a tuple with the LoadListing field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *OutputBinding) GetLoadListingOk() (*LoadListingEnum, bool) {
	if o == nil || IsNil(o.LoadListing) {
		return nil, false
	}
	return o.LoadListing, true
}

// HasLoadListing returns a boolean if a field has been set.
func (o *OutputBinding) HasLoadListing() bool {
	if o != nil && !IsNil(o.LoadListing) {
		return true
	}

	return false
}

// SetLoadListing gets a reference to the given LoadListingEnum and assigns it to the LoadListing field.
func (o *OutputBinding) SetLoadListing(v LoadListingEnum) {
	o.LoadListing = &v
}

// GetGlob returns the Glob field value if set, zero value otherwise.
func (o *OutputBinding) GetGlob() []string {
	if o == nil || IsNil(o.Glob) {
		var ret []string
		return ret
	}
	return o.Glob
}

// GetGlobOk returns a tuple with the Glob field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *OutputBinding) GetGlobOk() ([]string, bool) {
	if o == nil || IsNil(o.Glob) {
		return nil, false
	}
	return o.Glob, true
}

// HasGlob returns a boolean if a field has been set.
func (o *OutputBinding) HasGlob() bool {
	if o != nil && !IsNil(o.Glob) {
		return true
	}

	return false
}

// SetGlob gets a reference to the given []string and assigns it to the Glob field.
func (o *OutputBinding) SetGlob(v []string) {
	o.Glob = v
}

// GetOutputEval returns the OutputEval field value if set, zero value otherwise.
func (o *OutputBinding) GetOutputEval() string {
	if o == nil || IsNil(o.OutputEval) {
		var ret string
		return ret
	}
	return *o.OutputEval
}

// GetOutputEvalOk returns a tuple with the OutputEval field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *OutputBinding) GetOutputEvalOk() (*string, bool) {
	if o == nil || IsNil(o.OutputEval) {
		return nil, false
	}
	return o.OutputEval, true
}

// HasOutputEval returns a boolean if a field has been set.
func (o *OutputBinding) HasOutputEval() bool {
	if o != nil && !IsNil(o.OutputEval) {
		return true
	}

	return false
}

// SetOutputEval gets a reference to the given string and assigns it to the OutputEval field.
func (o *OutputBinding) SetOutputEval(v string) {
	o.OutputEval = &v
}

// GetStreamable returns the Streamable field value if set, zero value otherwise.
func (o *OutputBinding) GetStreamable() bool {
	if o == nil || IsNil(o.Streamable) {
		var ret bool
		return ret
	}
	return *o.Streamable
}

// GetStreamableOk returns a tuple with the Streamable field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *OutputBinding) GetStreamableOk() (*bool, bool) {
	if o == nil || IsNil(o.Streamable) {
		return nil, false
	}
	return o.Streamable, true
}

// HasStreamable returns a boolean if a field has been set.
func (o *OutputBinding) HasStreamable() bool {
	if o != nil && !IsNil(o.Streamable) {
		return true
	}

	return false
}

// SetStreamable gets a reference to the given bool and assigns it to the Streamable field.
func (o *OutputBinding) SetStreamable(v bool) {
	o.Streamable = &v
}

func (o OutputBinding) MarshalJSON() ([]byte, error) {
	toSerialize,err := o.ToMap()
	if err != nil {
		return []byte{}, err
	}
	return json.Marshal(toSerialize)
}

func (o OutputBinding) ToMap() (map[string]interface{}, error) {
	toSerialize := map[string]interface{}{}
	toSerialize["name"] = o.Name
	toSerialize["secondaryFiles"] = o.SecondaryFiles
	if !IsNil(o.LoadContents) {
		toSerialize["loadContents"] = o.LoadContents
	}
	if !IsNil(o.LoadListing) {
		toSerialize["loadListing"] = o.LoadListing
	}
	if !IsNil(o.Glob) {
		toSerialize["glob"] = o.Glob
	}
	if !IsNil(o.OutputEval) {
		toSerialize["outputEval"] = o.OutputEval
	}
	if !IsNil(o.Streamable) {
		toSerialize["streamable"] = o.Streamable
	}
	return toSerialize, nil
}

func (o *OutputBinding) UnmarshalJSON(data []byte) (err error) {
	// This validates that all required properties are included in the JSON object
	// by unmarshalling the object into a generic map with string keys and checking
	// that every required field exists as a key in the generic map.
	requiredProperties := []string{
		"name",
		"secondaryFiles",
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

	varOutputBinding := _OutputBinding{}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&varOutputBinding)

	if err != nil {
		return err
	}

	*o = OutputBinding(varOutputBinding)

	return err
}

type NullableOutputBinding struct {
	value *OutputBinding
	isSet bool
}

func (v NullableOutputBinding) Get() *OutputBinding {
	return v.value
}

func (v *NullableOutputBinding) Set(val *OutputBinding) {
	v.value = val
	v.isSet = true
}

func (v NullableOutputBinding) IsSet() bool {
	return v.isSet
}

func (v *NullableOutputBinding) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableOutputBinding(val *OutputBinding) *NullableOutputBinding {
	return &NullableOutputBinding{value: val, isSet: true}
}

func (v NullableOutputBinding) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableOutputBinding) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}


