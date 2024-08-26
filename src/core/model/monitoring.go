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

// Package model is to handle object of CB-Tumblebug
package model

const (
	MonMetricAll     string = "all"
	MonMetricCpu     string = "cpu"
	MonMetricCpufreq string = "cpufreq"
	MonMetricMem     string = "mem"
	MonMetricNet     string = "net"
	MonMetricSwap    string = "swap"
	MonMetricDisk    string = "disk"
	MonMetricDiskio  string = "diskio"
)

// MonAgentInstallReq struct
type MonAgentInstallReq struct {
	NsId     string `json:"nsId,omitempty"`
	MciId    string `json:"mciId,omitempty"`
	VmId     string `json:"vmId,omitempty"`
	PublicIp string `json:"publicIp,omitempty"`
	Port     string `json:"port,omitempty"`
	UserName string `json:"userName,omitempty"`
	SshKey   string `json:"sshKey,omitempty"`
	CspType  string `json:"cspType,omitempty"`
}

// MonResultSimple struct is for containing vm monitoring results
type MonResultSimple struct {
	Metric string `json:"metric"`
	VmId   string `json:"vmId"`
	Value  string `json:"value"`
	Err    string `json:"err"`
}

// MonResultSimpleResponse struct is for containing Mci monitoring results
type MonResultSimpleResponse struct {
	NsId          string            `json:"nsId"`
	MciId         string            `json:"mciId"`
	MciMonitoring []MonResultSimple `json:"mciMonitoring"`
}

// DfAgentInstallReq is struct for CB-Dragonfly monitoring agent installation request
type DfAgentInstallReq struct {
	NsId        string `json:"ns_id"`
	MciId       string `json:"mci_id"`
	VmId        string `json:"vm_id"`
	PublicIp    string `json:"public_ip"`
	UserName    string `json:"user_name"`
	SshKey      string `json:"ssh_key"`
	CspType     string `json:"cspType"`
	ServiceType string `json:"service_type"`
	Port        string `json:"port"`
}
