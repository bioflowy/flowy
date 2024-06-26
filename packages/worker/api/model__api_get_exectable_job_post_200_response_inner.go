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

// checks if the ApiGetExectableJobPost200ResponseInner type satisfies the MappedNullable interface at compile time
var _ MappedNullable = &ApiGetExectableJobPost200ResponseInner{}

// ApiGetExectableJobPost200ResponseInner struct for ApiGetExectableJobPost200ResponseInner
type ApiGetExectableJobPost200ResponseInner struct {
	Id string `json:"id"`
	Staging []StagingCommand `json:"staging"`
	Commands []string `json:"commands"`
	StdinPath *string `json:"stdin_path,omitempty"`
	StdoutPath *string `json:"stdout_path,omitempty"`
	StderrPath *string `json:"stderr_path,omitempty"`
	Env map[string]string `json:"env"`
	Cwd string `json:"cwd"`
	BuilderOutdir string `json:"builderOutdir"`
	Timelimit *int32 `json:"timelimit,omitempty"`
	OutputBindings []OutputBinding `json:"outputBindings"`
	Fileitems []MapperEnt `json:"fileitems"`
	Generatedlist []MapperEnt `json:"generatedlist"`
	InplaceUpdate bool `json:"inplace_update"`
	OutputBaseDir *string `json:"outputBaseDir,omitempty"`
}

type _ApiGetExectableJobPost200ResponseInner ApiGetExectableJobPost200ResponseInner

// NewApiGetExectableJobPost200ResponseInner instantiates a new ApiGetExectableJobPost200ResponseInner object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewApiGetExectableJobPost200ResponseInner(id string, staging []StagingCommand, commands []string, env map[string]string, cwd string, builderOutdir string, outputBindings []OutputBinding, fileitems []MapperEnt, generatedlist []MapperEnt, inplaceUpdate bool) *ApiGetExectableJobPost200ResponseInner {
	this := ApiGetExectableJobPost200ResponseInner{}
	this.Id = id
	this.Staging = staging
	this.Commands = commands
	this.Env = env
	this.Cwd = cwd
	this.BuilderOutdir = builderOutdir
	this.OutputBindings = outputBindings
	this.Fileitems = fileitems
	this.Generatedlist = generatedlist
	this.InplaceUpdate = inplaceUpdate
	return &this
}

// NewApiGetExectableJobPost200ResponseInnerWithDefaults instantiates a new ApiGetExectableJobPost200ResponseInner object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewApiGetExectableJobPost200ResponseInnerWithDefaults() *ApiGetExectableJobPost200ResponseInner {
	this := ApiGetExectableJobPost200ResponseInner{}
	return &this
}

// GetId returns the Id field value
func (o *ApiGetExectableJobPost200ResponseInner) GetId() string {
	if o == nil {
		var ret string
		return ret
	}

	return o.Id
}

// GetIdOk returns a tuple with the Id field value
// and a boolean to check if the value has been set.
func (o *ApiGetExectableJobPost200ResponseInner) GetIdOk() (*string, bool) {
	if o == nil {
		return nil, false
	}
	return &o.Id, true
}

// SetId sets field value
func (o *ApiGetExectableJobPost200ResponseInner) SetId(v string) {
	o.Id = v
}

// GetStaging returns the Staging field value
func (o *ApiGetExectableJobPost200ResponseInner) GetStaging() []StagingCommand {
	if o == nil {
		var ret []StagingCommand
		return ret
	}

	return o.Staging
}

// GetStagingOk returns a tuple with the Staging field value
// and a boolean to check if the value has been set.
func (o *ApiGetExectableJobPost200ResponseInner) GetStagingOk() ([]StagingCommand, bool) {
	if o == nil {
		return nil, false
	}
	return o.Staging, true
}

// SetStaging sets field value
func (o *ApiGetExectableJobPost200ResponseInner) SetStaging(v []StagingCommand) {
	o.Staging = v
}

// GetCommands returns the Commands field value
func (o *ApiGetExectableJobPost200ResponseInner) GetCommands() []string {
	if o == nil {
		var ret []string
		return ret
	}

	return o.Commands
}

// GetCommandsOk returns a tuple with the Commands field value
// and a boolean to check if the value has been set.
func (o *ApiGetExectableJobPost200ResponseInner) GetCommandsOk() ([]string, bool) {
	if o == nil {
		return nil, false
	}
	return o.Commands, true
}

// SetCommands sets field value
func (o *ApiGetExectableJobPost200ResponseInner) SetCommands(v []string) {
	o.Commands = v
}

// GetStdinPath returns the StdinPath field value if set, zero value otherwise.
func (o *ApiGetExectableJobPost200ResponseInner) GetStdinPath() string {
	if o == nil || IsNil(o.StdinPath) {
		var ret string
		return ret
	}
	return *o.StdinPath
}

// GetStdinPathOk returns a tuple with the StdinPath field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ApiGetExectableJobPost200ResponseInner) GetStdinPathOk() (*string, bool) {
	if o == nil || IsNil(o.StdinPath) {
		return nil, false
	}
	return o.StdinPath, true
}

// HasStdinPath returns a boolean if a field has been set.
func (o *ApiGetExectableJobPost200ResponseInner) HasStdinPath() bool {
	if o != nil && !IsNil(o.StdinPath) {
		return true
	}

	return false
}

// SetStdinPath gets a reference to the given string and assigns it to the StdinPath field.
func (o *ApiGetExectableJobPost200ResponseInner) SetStdinPath(v string) {
	o.StdinPath = &v
}

// GetStdoutPath returns the StdoutPath field value if set, zero value otherwise.
func (o *ApiGetExectableJobPost200ResponseInner) GetStdoutPath() string {
	if o == nil || IsNil(o.StdoutPath) {
		var ret string
		return ret
	}
	return *o.StdoutPath
}

// GetStdoutPathOk returns a tuple with the StdoutPath field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ApiGetExectableJobPost200ResponseInner) GetStdoutPathOk() (*string, bool) {
	if o == nil || IsNil(o.StdoutPath) {
		return nil, false
	}
	return o.StdoutPath, true
}

// HasStdoutPath returns a boolean if a field has been set.
func (o *ApiGetExectableJobPost200ResponseInner) HasStdoutPath() bool {
	if o != nil && !IsNil(o.StdoutPath) {
		return true
	}

	return false
}

// SetStdoutPath gets a reference to the given string and assigns it to the StdoutPath field.
func (o *ApiGetExectableJobPost200ResponseInner) SetStdoutPath(v string) {
	o.StdoutPath = &v
}

// GetStderrPath returns the StderrPath field value if set, zero value otherwise.
func (o *ApiGetExectableJobPost200ResponseInner) GetStderrPath() string {
	if o == nil || IsNil(o.StderrPath) {
		var ret string
		return ret
	}
	return *o.StderrPath
}

// GetStderrPathOk returns a tuple with the StderrPath field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ApiGetExectableJobPost200ResponseInner) GetStderrPathOk() (*string, bool) {
	if o == nil || IsNil(o.StderrPath) {
		return nil, false
	}
	return o.StderrPath, true
}

// HasStderrPath returns a boolean if a field has been set.
func (o *ApiGetExectableJobPost200ResponseInner) HasStderrPath() bool {
	if o != nil && !IsNil(o.StderrPath) {
		return true
	}

	return false
}

// SetStderrPath gets a reference to the given string and assigns it to the StderrPath field.
func (o *ApiGetExectableJobPost200ResponseInner) SetStderrPath(v string) {
	o.StderrPath = &v
}

// GetEnv returns the Env field value
func (o *ApiGetExectableJobPost200ResponseInner) GetEnv() map[string]string {
	if o == nil {
		var ret map[string]string
		return ret
	}

	return o.Env
}

// GetEnvOk returns a tuple with the Env field value
// and a boolean to check if the value has been set.
func (o *ApiGetExectableJobPost200ResponseInner) GetEnvOk() (*map[string]string, bool) {
	if o == nil {
		return nil, false
	}
	return &o.Env, true
}

// SetEnv sets field value
func (o *ApiGetExectableJobPost200ResponseInner) SetEnv(v map[string]string) {
	o.Env = v
}

// GetCwd returns the Cwd field value
func (o *ApiGetExectableJobPost200ResponseInner) GetCwd() string {
	if o == nil {
		var ret string
		return ret
	}

	return o.Cwd
}

// GetCwdOk returns a tuple with the Cwd field value
// and a boolean to check if the value has been set.
func (o *ApiGetExectableJobPost200ResponseInner) GetCwdOk() (*string, bool) {
	if o == nil {
		return nil, false
	}
	return &o.Cwd, true
}

// SetCwd sets field value
func (o *ApiGetExectableJobPost200ResponseInner) SetCwd(v string) {
	o.Cwd = v
}

// GetBuilderOutdir returns the BuilderOutdir field value
func (o *ApiGetExectableJobPost200ResponseInner) GetBuilderOutdir() string {
	if o == nil {
		var ret string
		return ret
	}

	return o.BuilderOutdir
}

// GetBuilderOutdirOk returns a tuple with the BuilderOutdir field value
// and a boolean to check if the value has been set.
func (o *ApiGetExectableJobPost200ResponseInner) GetBuilderOutdirOk() (*string, bool) {
	if o == nil {
		return nil, false
	}
	return &o.BuilderOutdir, true
}

// SetBuilderOutdir sets field value
func (o *ApiGetExectableJobPost200ResponseInner) SetBuilderOutdir(v string) {
	o.BuilderOutdir = v
}

// GetTimelimit returns the Timelimit field value if set, zero value otherwise.
func (o *ApiGetExectableJobPost200ResponseInner) GetTimelimit() int32 {
	if o == nil || IsNil(o.Timelimit) {
		var ret int32
		return ret
	}
	return *o.Timelimit
}

// GetTimelimitOk returns a tuple with the Timelimit field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ApiGetExectableJobPost200ResponseInner) GetTimelimitOk() (*int32, bool) {
	if o == nil || IsNil(o.Timelimit) {
		return nil, false
	}
	return o.Timelimit, true
}

// HasTimelimit returns a boolean if a field has been set.
func (o *ApiGetExectableJobPost200ResponseInner) HasTimelimit() bool {
	if o != nil && !IsNil(o.Timelimit) {
		return true
	}

	return false
}

// SetTimelimit gets a reference to the given int32 and assigns it to the Timelimit field.
func (o *ApiGetExectableJobPost200ResponseInner) SetTimelimit(v int32) {
	o.Timelimit = &v
}

// GetOutputBindings returns the OutputBindings field value
func (o *ApiGetExectableJobPost200ResponseInner) GetOutputBindings() []OutputBinding {
	if o == nil {
		var ret []OutputBinding
		return ret
	}

	return o.OutputBindings
}

// GetOutputBindingsOk returns a tuple with the OutputBindings field value
// and a boolean to check if the value has been set.
func (o *ApiGetExectableJobPost200ResponseInner) GetOutputBindingsOk() ([]OutputBinding, bool) {
	if o == nil {
		return nil, false
	}
	return o.OutputBindings, true
}

// SetOutputBindings sets field value
func (o *ApiGetExectableJobPost200ResponseInner) SetOutputBindings(v []OutputBinding) {
	o.OutputBindings = v
}

// GetFileitems returns the Fileitems field value
func (o *ApiGetExectableJobPost200ResponseInner) GetFileitems() []MapperEnt {
	if o == nil {
		var ret []MapperEnt
		return ret
	}

	return o.Fileitems
}

// GetFileitemsOk returns a tuple with the Fileitems field value
// and a boolean to check if the value has been set.
func (o *ApiGetExectableJobPost200ResponseInner) GetFileitemsOk() ([]MapperEnt, bool) {
	if o == nil {
		return nil, false
	}
	return o.Fileitems, true
}

// SetFileitems sets field value
func (o *ApiGetExectableJobPost200ResponseInner) SetFileitems(v []MapperEnt) {
	o.Fileitems = v
}

// GetGeneratedlist returns the Generatedlist field value
func (o *ApiGetExectableJobPost200ResponseInner) GetGeneratedlist() []MapperEnt {
	if o == nil {
		var ret []MapperEnt
		return ret
	}

	return o.Generatedlist
}

// GetGeneratedlistOk returns a tuple with the Generatedlist field value
// and a boolean to check if the value has been set.
func (o *ApiGetExectableJobPost200ResponseInner) GetGeneratedlistOk() ([]MapperEnt, bool) {
	if o == nil {
		return nil, false
	}
	return o.Generatedlist, true
}

// SetGeneratedlist sets field value
func (o *ApiGetExectableJobPost200ResponseInner) SetGeneratedlist(v []MapperEnt) {
	o.Generatedlist = v
}

// GetInplaceUpdate returns the InplaceUpdate field value
func (o *ApiGetExectableJobPost200ResponseInner) GetInplaceUpdate() bool {
	if o == nil {
		var ret bool
		return ret
	}

	return o.InplaceUpdate
}

// GetInplaceUpdateOk returns a tuple with the InplaceUpdate field value
// and a boolean to check if the value has been set.
func (o *ApiGetExectableJobPost200ResponseInner) GetInplaceUpdateOk() (*bool, bool) {
	if o == nil {
		return nil, false
	}
	return &o.InplaceUpdate, true
}

// SetInplaceUpdate sets field value
func (o *ApiGetExectableJobPost200ResponseInner) SetInplaceUpdate(v bool) {
	o.InplaceUpdate = v
}

// GetOutputBaseDir returns the OutputBaseDir field value if set, zero value otherwise.
func (o *ApiGetExectableJobPost200ResponseInner) GetOutputBaseDir() string {
	if o == nil || IsNil(o.OutputBaseDir) {
		var ret string
		return ret
	}
	return *o.OutputBaseDir
}

// GetOutputBaseDirOk returns a tuple with the OutputBaseDir field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ApiGetExectableJobPost200ResponseInner) GetOutputBaseDirOk() (*string, bool) {
	if o == nil || IsNil(o.OutputBaseDir) {
		return nil, false
	}
	return o.OutputBaseDir, true
}

// HasOutputBaseDir returns a boolean if a field has been set.
func (o *ApiGetExectableJobPost200ResponseInner) HasOutputBaseDir() bool {
	if o != nil && !IsNil(o.OutputBaseDir) {
		return true
	}

	return false
}

// SetOutputBaseDir gets a reference to the given string and assigns it to the OutputBaseDir field.
func (o *ApiGetExectableJobPost200ResponseInner) SetOutputBaseDir(v string) {
	o.OutputBaseDir = &v
}

func (o ApiGetExectableJobPost200ResponseInner) MarshalJSON() ([]byte, error) {
	toSerialize,err := o.ToMap()
	if err != nil {
		return []byte{}, err
	}
	return json.Marshal(toSerialize)
}

func (o ApiGetExectableJobPost200ResponseInner) ToMap() (map[string]interface{}, error) {
	toSerialize := map[string]interface{}{}
	toSerialize["id"] = o.Id
	toSerialize["staging"] = o.Staging
	toSerialize["commands"] = o.Commands
	if !IsNil(o.StdinPath) {
		toSerialize["stdin_path"] = o.StdinPath
	}
	if !IsNil(o.StdoutPath) {
		toSerialize["stdout_path"] = o.StdoutPath
	}
	if !IsNil(o.StderrPath) {
		toSerialize["stderr_path"] = o.StderrPath
	}
	toSerialize["env"] = o.Env
	toSerialize["cwd"] = o.Cwd
	toSerialize["builderOutdir"] = o.BuilderOutdir
	if !IsNil(o.Timelimit) {
		toSerialize["timelimit"] = o.Timelimit
	}
	toSerialize["outputBindings"] = o.OutputBindings
	toSerialize["fileitems"] = o.Fileitems
	toSerialize["generatedlist"] = o.Generatedlist
	toSerialize["inplace_update"] = o.InplaceUpdate
	if !IsNil(o.OutputBaseDir) {
		toSerialize["outputBaseDir"] = o.OutputBaseDir
	}
	return toSerialize, nil
}

func (o *ApiGetExectableJobPost200ResponseInner) UnmarshalJSON(data []byte) (err error) {
	// This validates that all required properties are included in the JSON object
	// by unmarshalling the object into a generic map with string keys and checking
	// that every required field exists as a key in the generic map.
	requiredProperties := []string{
		"id",
		"staging",
		"commands",
		"env",
		"cwd",
		"builderOutdir",
		"outputBindings",
		"fileitems",
		"generatedlist",
		"inplace_update",
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

	varApiGetExectableJobPost200ResponseInner := _ApiGetExectableJobPost200ResponseInner{}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&varApiGetExectableJobPost200ResponseInner)

	if err != nil {
		return err
	}

	*o = ApiGetExectableJobPost200ResponseInner(varApiGetExectableJobPost200ResponseInner)

	return err
}

type NullableApiGetExectableJobPost200ResponseInner struct {
	value *ApiGetExectableJobPost200ResponseInner
	isSet bool
}

func (v NullableApiGetExectableJobPost200ResponseInner) Get() *ApiGetExectableJobPost200ResponseInner {
	return v.value
}

func (v *NullableApiGetExectableJobPost200ResponseInner) Set(val *ApiGetExectableJobPost200ResponseInner) {
	v.value = val
	v.isSet = true
}

func (v NullableApiGetExectableJobPost200ResponseInner) IsSet() bool {
	return v.isSet
}

func (v *NullableApiGetExectableJobPost200ResponseInner) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableApiGetExectableJobPost200ResponseInner(val *ApiGetExectableJobPost200ResponseInner) *NullableApiGetExectableJobPost200ResponseInner {
	return &NullableApiGetExectableJobPost200ResponseInner{value: val, isSet: true}
}

func (v NullableApiGetExectableJobPost200ResponseInner) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableApiGetExectableJobPost200ResponseInner) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}


