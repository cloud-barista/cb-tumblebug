package csp

// Supported Cloud Service Providers
const (
	Alibaba   = "alibaba"
	AWS       = "aws"
	Azure     = "azure"
	GCP       = "gcp"
	IBM       = "ibm"
	Tencent   = "tencent"
	NCP       = "ncp"
	NHN       = "nhn"
	KT        = "kt"
	OpenStack = "openstack"
)

// AllCSPs is the list of all supported Cloud Service Providers
var AllCSPs = []string{
	AWS, Azure, GCP, Alibaba, Tencent, IBM, OpenStack, NCP, NHN, KT,
}
