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

// Package mci is to handle REST API for mci
package model

// SiteDetail struct represents the structure for detailed site information
type SiteDetail struct {
	CSP            string `json:"csp" example:"aws"`
	Region         string `json:"region" example:"ap-northeast-2"`
	ConnectionName string `json:"connectionName" example:"aws-ap-northeast-2"`
	// Zone              string `json:"zone,omitempty" example:"ap-northeast-2a"`
	VNetId string `json:"vnet" example:"vpc-xxxxx"`
	// SubnetId          string `json:"subnet,omitempty" example:"subnet-xxxxx"`
	GatewaySubnetCidr string `json:"gatewaySubnetCidr,omitempty" example:"xxx.xxx.xxx.xxx/xx"`
	ResourceGroup     string `json:"resourceGroup,omitempty" example:"rg-xxxxx"`
}

// Sites struct represents the overall site information
type sites struct {
	Aws     []SiteDetail `json:"aws,omitempty"`
	Azure   []SiteDetail `json:"azure,omitempty"`
	Gcp     []SiteDetail `json:"gcp,omitempty"`
	Alibaba []SiteDetail `json:"alibaba,omitempty"`
	Tencent []SiteDetail `json:"tencent,omitempty"`
	Ibm     []SiteDetail `json:"ibm,omitempty"`
}

// SitesInfo struct represents the overall site information including namespace and MCI ID
type SitesInfo struct {
	NsId  string `json:"nsId" example:"ns-01"`
	MciId string `json:"mciId" example:"mci-01"`
	Count int    `json:"count" example:"3"`
	Sites sites  `json:"sites"`
}

func NewSiteInfo(nsId, mciId string) *SitesInfo {
	siteInfo := &SitesInfo{
		NsId:  nsId,
		MciId: mciId,
		Count: 0,
		Sites: sites{
			Aws:     []SiteDetail{},
			Azure:   []SiteDetail{},
			Gcp:     []SiteDetail{},
			Alibaba: []SiteDetail{},
			Tencent: []SiteDetail{},
			Ibm:     []SiteDetail{},
		},
	}

	return siteInfo
}

/*
 *
 */

type SiteProperty struct {
	VNetId              string              `json:"vNetId" example:"vnet01"`
	CspSpecificProperty CspSpecificProperty `json:"cspSpecificProperty,omitempty"`
}

type CspSpecificProperty struct {
	Aws     *AwsSpecificProperty     `json:"aws,omitempty"`
	Azure   *AzureSpecificProperty   `json:"azure,omitempty"`
	Gcp     *GcpSpecificProperty     `json:"gcp,omitempty"`
	Alibaba *AlibabaSpecificProperty `json:"alibaba,omitempty"`
	// Tencent *TencentSpecificProperty `json:"tencent,omitempty"`
	// Ibm     *IbmSpecificProperty     `json:"ibm,omitempty"`
}

type AwsSpecificProperty struct {
	BgpAsn string `json:"bgpAsn,omitempty" default:"64512" example:"64512"`
}

type AzureSpecificProperty struct {
	GatewaySubnetCidr string `json:"gatewaySubnetCidr,omitempty" default:"" example:"xxx.xxx.xxx.xxx/xx"`
	BgpAsn            string `json:"bgpAsn,omitempty" default:"65531" example:"65531"`
	VpnSku            string `json:"vpnSku,omitempty" default:"VpnGw1AZ" example:"VpnGw1AZ"`
}

type GcpSpecificProperty struct {
	BgpAsn string `json:"bgpAsn,omitempty" default:"65530" example:"65530"`
}

type AlibabaSpecificProperty struct {
	BgpAsn string `json:"bgpAsn,omitempty" default:"65532" example:"65532"`
}

// * Note: nothing is needed for Tencent currently.
type TencentSpecificProperty struct {
}

// * Note: nothing is needed for IBM currently.
type IbmSpecificProperty struct {
}

type RestPostVpnRequest struct {
	Name  string       `json:"name" validate:"required" example:"vpn01"`
	Site1 SiteProperty `json:"site1" validate:"required"`
	Site2 SiteProperty `json:"site2" validate:"required"`
}

type Response struct {
	Success bool                   `json:"success" example:"true"`
	Status  int                    `json:"status,omitempty" example:"200"`
	Message string                 `json:"message" example:"Any message"`
	Detail  string                 `json:"details,omitempty" example:"Any details"`
	Object  map[string]interface{} `json:"object,omitempty"`
	List    []interface{}          `json:"list,omitempty"`
}

type VpnIdList struct {
	VpnIdList []string `json:"vpnIdList"`
}

type VpnInfoList struct {
	VpnInfoList []VpnInfo `json:"vpnInfoList"`
}

type VpnInfo struct {
	// ResourceType is the type of the resource
	ResourceType string `json:"resourceType"`

	// Id is unique identifier for the object
	Id string `json:"id" example:"vpn01"`
	// Uid is universally unique identifier for the object, used for labelSelector
	Uid string `json:"uid,omitempty" example:"wef12awefadf1221edcf"`

	// Name is human-readable string to represent the object
	Name        string          `json:"name" example:"vpn01"`
	Description string          `json:"description"`
	Status      string          `json:"status"`
	VpnSites    []VpnSiteDetail `json:"vpnSites"`
}

type VpnSiteDetail struct {
	ConnectionName   string     `json:"connectionName"`
	ConnectionConfig ConnConfig `json:"connectionConfig"`

	// ResourceDetails represents a CSP's multiple resources associated with the VPN from a CSP.
	ResourceDetails []ResourceDetail `json:"resourceDetails"`
}

type ResourceDetail struct {
	// CspResourceName is name assigned to the CSP resource. This name is internally used to handle the resource.
	CspResourceName string `json:"cspResourceName" default:"" example:"we12fawefadf1221edcf"`
	// CspResourceId is resource identifier managed by CSP
	CspResourceId string `json:"cspResourceId" default:"" example:"csp-06eb41e14121c550a"`
	// CspResourceDetail is the detailed information of the resource provided from the terrarium.
	CspResourceDetail any `json:"cspResourceDetail"`

	Status string `json:"status,omitempty"`
}
