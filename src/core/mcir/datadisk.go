/*
Copyright 2019 The Cloud-Barista Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package mcir is to manage multi-cloud infra resource
package mcir

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/go-resty/resty/v2"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"

	validator "github.com/go-playground/validator/v10"
)

type DiskStatus string

const (
	DiskCreating  DiskStatus = "Creating"
	DiskAvailable DiskStatus = "Available"
	DiskAttached  DiskStatus = "Attached"
	DiskDeleting  DiskStatus = "Deleting"
	DiskError     DiskStatus = "Error"
)

// TbAttachDetachDataDiskReq is a wrapper struct to create JSON body of 'Attach/Detach disk request'
type TbAttachDetachDataDiskReq struct {
	DataDiskId string `json:"dataDiskId" validate:"required"`
}

// SpiderDiskAttachDetachReqWrapper is a wrapper struct to create JSON body of 'Attach/Detach disk request'
type SpiderDiskAttachDetachReqWrapper struct {
	ConnectionName string
	ReqInfo        SpiderDiskAttachDetachReq
}

// SpiderDiskAttachDetachReq is a struct to create JSON body of 'Attach/Detach disk request'
type SpiderDiskAttachDetachReq struct {
	VMName string
}

// SpiderDiskUpsizeReqWrapper is a wrapper struct to create JSON body of 'Upsize disk request'
type SpiderDiskUpsizeReqWrapper struct {
	ConnectionName string
	ReqInfo        SpiderDiskUpsizeReq
}

// SpiderDiskUpsizeReq is a struct to create JSON body of 'Upsize disk request'
type SpiderDiskUpsizeReq struct {
	Size string // "", "default", "50", "1000"  # (GB)
}

// SpiderDiskReqInfoWrapper is a wrapper struct to create JSON body of 'Get disk request'
type SpiderDiskReqInfoWrapper struct {
	ConnectionName string
	ReqInfo        SpiderDiskInfo
}

// SpiderDiskInfo is a struct to create JSON body of 'Get disk request'
type SpiderDiskInfo struct {
	// Fields for request
	Name  string
	CSPid string

	// Fields for both request and response
	DiskType string // "", "SSD(gp2)", "Premium SSD", ...
	DiskSize string // "", "default", "50", "1000"  # (GB)

	// Fields for response
	IId common.IID // {NameId, SystemId}

	Status  DiskStatus // DiskCreating | DiskAvailable | DiskAttached | DiskDeleting | DiskError
	OwnerVM common.IID // When the Status is DiskAttached

	CreatedTime  time.Time
	KeyValueList []common.KeyValue
}

// TbDataDiskReq is a struct to handle 'Register dataDisk' request toward CB-Tumblebug.
type TbDataDiskReq struct {
	Name           string `json:"name" validate:"required" example:"aws-ap-southeast-1-datadisk"`
	ConnectionName string `json:"connectionName" validate:"required" example:"aws-ap-southeast-1"`
	DiskType       string `json:"diskType" example:"default"`
	DiskSize       string `json:"diskSize" validate:"required" example:"77" default:"100"`
	Description    string `json:"description,omitempty"`

	// Fields for "Register existing dataDisk" feature
	// CspDataDiskId is required to register object from CSP (option=register)
	CspDataDiskId string `json:"cspDataDiskId"`
}

// TbDataDiskReqStructLevelValidation func is for Validation
func TbDataDiskReqStructLevelValidation(sl validator.StructLevel) {

	u := sl.Current().Interface().(TbDataDiskReq)

	err := common.CheckString(u.Name)
	if err != nil {
		// ReportError(field interface{}, fieldName, structFieldName, tag, param string)
		sl.ReportError(u.Name, "name", "Name", err.Error(), "")
	}
}

// TbDataDiskInfo is a struct that represents TB dataDisk object.
type TbDataDiskInfo struct {
	Id                   string            `json:"id,omitempty" example:"aws-ap-southeast-1-datadisk"`
	Name                 string            `json:"name,omitempty" example:"aws-ap-southeast-1-datadisk"`
	ConnectionName       string            `json:"connectionName,omitempty" example:"aws-ap-southeast-1"`
	DiskType             string            `json:"diskType" example:"standard"`
	DiskSize             string            `json:"diskSize" example:"77"`
	CspDataDiskId        string            `json:"cspDataDiskId,omitempty" example:"vol-0d397c3239629bd43"`
	CspDataDiskName      string            `json:"cspDataDiskName,omitempty" example:"ns01-aws-ap-southeast-1-datadisk"`
	Status               DiskStatus        `json:"status" example:"Available"` // Available, Unavailable, Attached, ...
	AssociatedObjectList []string          `json:"associatedObjectList" example:["/ns/ns01/mcis/mcis01/vm/aws-ap-southeast-1-1"]`
	CreatedTime          time.Time         `json:"createdTime,omitempty" example:"2022-10-12T05:09:51.05Z"`
	KeyValueList         []common.KeyValue `json:"keyValueList,omitempty"`
	Description          string            `json:"description,omitempty" example:"Available"`
	IsAutoGenerated      bool              `json:"isAutoGenerated,omitempty"`

	// SystemLabel is for describing the MCIR in a keyword (any string can be used) for special System purpose
	SystemLabel string `json:"systemLabel,omitempty" example:"Managed by CB-Tumblebug" default:""`
}

// CreateDataDisk accepts DataDisk creation request, creates and returns an TB dataDisk object
func CreateDataDisk(nsId string, u *TbDataDiskReq, option string) (TbDataDiskInfo, error) {

	resourceType := common.StrDataDisk

	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return TbDataDiskInfo{}, err
	}

	if option != "register" { // fields validation
		err = validate.Struct(u)
		if err != nil {
			if _, ok := err.(*validator.InvalidValidationError); ok {
				fmt.Println(err)
				return TbDataDiskInfo{}, err
			}

			return TbDataDiskInfo{}, err
		}
	}

	check, err := CheckResource(nsId, resourceType, u.Name)

	if check {
		err := fmt.Errorf("The dataDisk %s already exists.", u.Name)
		return TbDataDiskInfo{}, err
	}

	if err != nil {
		err := fmt.Errorf("Failed to check the existence of the dataDisk %s.", u.Name)
		return TbDataDiskInfo{}, err
	}

	tempReq := SpiderDiskReqInfoWrapper{
		ConnectionName: u.ConnectionName,
		ReqInfo: SpiderDiskInfo{
			Name:     fmt.Sprintf("%s-%s", nsId, u.Name),
			CSPid:    u.CspDataDiskId, // for option=register
			DiskType: u.DiskType,
			DiskSize: u.DiskSize,
		},
	}

	var tempSpiderDiskInfo *SpiderDiskInfo

	// if os.Getenv("SPIDER_CALL_METHOD") == "REST" {

	client := resty.New().SetCloseConnection(true)
	client.SetAllowGetMethodPayload(true)

	req := client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(tempReq).
		SetResult(&SpiderDiskInfo{}) // or SetResult(AuthSuccess{}).
		//SetError(&AuthError{}).       // or SetError(AuthError{}).

	var resp *resty.Response
	// var err error

	var url string
	if option == "register" && u.CspDataDiskId == "" {
		url = fmt.Sprintf("%s/disk/%s", common.SpiderRestUrl, u.Name)
		resp, err = req.Get(url)
	} else if option == "register" && u.CspDataDiskId != "" {
		url = fmt.Sprintf("%s/regdisk", common.SpiderRestUrl)
		resp, err = req.Post(url)
	} else { // option != "register"
		url = fmt.Sprintf("%s/disk", common.SpiderRestUrl)
		resp, err = req.Post(url)
	}

	if err != nil {
		common.CBLog.Error(err)
		err := fmt.Errorf("an error occurred while requesting to CB-Spider")
		return TbDataDiskInfo{}, err
	}

	fmt.Printf("HTTP Status code: %d \n", resp.StatusCode())
	switch {
	case resp.StatusCode() >= 400 || resp.StatusCode() < 200:
		err := fmt.Errorf(string(resp.Body()))
		fmt.Println("body: ", string(resp.Body()))
		common.CBLog.Error(err)
		return TbDataDiskInfo{}, err
	}

	tempSpiderDiskInfo = resp.Result().(*SpiderDiskInfo)

	// } else { // gRPC
	// } // gRPC

	content := TbDataDiskInfo{
		Id:                   u.Name,
		Name:                 u.Name,
		ConnectionName:       u.ConnectionName,
		DiskType:             tempSpiderDiskInfo.DiskType,
		DiskSize:             tempSpiderDiskInfo.DiskSize,
		CspDataDiskId:        tempSpiderDiskInfo.IId.SystemId,
		CspDataDiskName:      tempSpiderDiskInfo.IId.NameId,
		Status:               tempSpiderDiskInfo.Status,
		AssociatedObjectList: []string{},
		CreatedTime:          tempSpiderDiskInfo.CreatedTime,
		KeyValueList:         tempSpiderDiskInfo.KeyValueList,
		Description:          u.Description,
		IsAutoGenerated:      false,
	}

	if option == "register" {
		if u.CspDataDiskId == "" {
			content.SystemLabel = "Registered from CB-Spider resource"
		} else if u.CspDataDiskId != "" {
			content.SystemLabel = "Registered from CSP resource"
		}
	}

	// cb-store
	fmt.Println("=========================== PUT CreateDataDisk")
	Key := common.GenResourceKey(nsId, resourceType, content.Id)
	Val, _ := json.Marshal(content)
	err = common.CBStore.Put(Key, string(Val))
	if err != nil {
		common.CBLog.Error(err)
		return content, err
	}
	return content, nil
}

// TbDataDiskUpsizeReq is a struct to handle 'Upsize dataDisk' request toward CB-Tumblebug.
type TbDataDiskUpsizeReq struct {
	DiskSize    string `json:"diskSize" validate:"required"`
	Description string `json:"description"`
}

// UpsizeDataDisk accepts DataDisk upsize request, creates and returns an TB dataDisk object
func UpsizeDataDisk(nsId string, resourceId string, u *TbDataDiskUpsizeReq) (TbDataDiskInfo, error) {

	resourceType := common.StrDataDisk

	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return TbDataDiskInfo{}, err
	}

	err = validate.Struct(u)
	if err != nil {
		if _, ok := err.(*validator.InvalidValidationError); ok {
			fmt.Println(err)
			return TbDataDiskInfo{}, err
		}

		return TbDataDiskInfo{}, err
	}

	check, err := CheckResource(nsId, resourceType, resourceId)

	if !check {
		err := fmt.Errorf("The dataDisk %s does not exist.", resourceId)
		return TbDataDiskInfo{}, err
	}

	if err != nil {
		err := fmt.Errorf("Failed to check the existence of the dataDisk %s.", resourceId)
		return TbDataDiskInfo{}, err
	}

	dataDiskInterface, err := GetResource(nsId, resourceType, resourceId)
	if err != nil {
		err := fmt.Errorf("Failed to get the dataDisk object %s.", resourceId)
		return TbDataDiskInfo{}, err
	}

	dataDisk := dataDiskInterface.(TbDataDiskInfo)

	diskSize_as_is, _ := strconv.Atoi(dataDisk.DiskSize)
	diskSize_to_be, err := strconv.Atoi(u.DiskSize)
	if err != nil {
		err := fmt.Errorf("Failed to convert the desired disk size (%s) into int.", u.DiskSize)
		return TbDataDiskInfo{}, err
	}

	if !(diskSize_as_is < diskSize_to_be) {
		err := fmt.Errorf("Desired disk size (%s GB) should be > %s GB.", u.DiskSize, dataDisk.DiskSize)
		return TbDataDiskInfo{}, err
	}

	tempReq := SpiderDiskUpsizeReqWrapper{
		ConnectionName: dataDisk.ConnectionName,
		ReqInfo: SpiderDiskUpsizeReq{
			Size: u.DiskSize,
		},
	}

	// if os.Getenv("SPIDER_CALL_METHOD") == "REST" {

	client := resty.New().SetCloseConnection(true)
	client.SetAllowGetMethodPayload(true)

	req := client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(tempReq)
		// SetResult(&SpiderDiskInfo{}) // or SetResult(AuthSuccess{}).
		//SetError(&AuthError{}).       // or SetError(AuthError{}).

	var resp *resty.Response
	// var err error

	url := fmt.Sprintf("%s/disk/%s/size", common.SpiderRestUrl, fmt.Sprintf("%s-%s", nsId, resourceId))
	resp, err = req.Put(url)

	if err != nil {
		common.CBLog.Error(err)
		err := fmt.Errorf("an error occurred while requesting to CB-Spider")
		return TbDataDiskInfo{}, err
	}

	fmt.Printf("HTTP Status code: %d \n", resp.StatusCode())
	switch {
	case resp.StatusCode() >= 400 || resp.StatusCode() < 200:
		err := fmt.Errorf(string(resp.Body()))
		fmt.Println("body: ", string(resp.Body()))
		common.CBLog.Error(err)
		return TbDataDiskInfo{}, err
	}

	/*
		isSuccessful := resp.Result().(bool)
		if isSuccessful == false {
			err := fmt.Errorf("Failed to upsize the dataDisk %s", resourceId)
			return TbDataDiskInfo{}, err
		}
	*/

	// } else { // gRPC
	// } // gRPC

	content := dataDisk
	content.DiskSize = u.DiskSize
	content.Description = u.Description

	// cb-store
	fmt.Println("=========================== PUT UpsizeDataDisk")
	Key := common.GenResourceKey(nsId, resourceType, content.Id)
	Val, _ := json.Marshal(content)
	err = common.CBStore.Put(Key, string(Val))
	if err != nil {
		common.CBLog.Error(err)
		return content, err
	}
	return content, nil
}
