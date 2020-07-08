/*
Copyright © 2020 NAME HERE <EMAIL ADDRESS>

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
package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"

	//conf "github.com/cloud-barista/cb-tumblebug/src/cli/config"

	"github.com/cloud-barista/cb-tumblebug/src/common"
	"github.com/cloud-barista/cb-tumblebug/src/mcir"
)

const ActionTerminate string = "Terminate"
const ActionSuspend string = "Suspend"
const ActionResume string = "Resume"
const ActionReboot string = "Reboot"

const StatusRunning string = "Running"
const StatusSuspended string = "Suspended"
const StatusFailed string = "Failed"
const StatusTerminated string = "Terminated"
const StatusCreating string = "Creating"
const StatusSuspending string = "Suspending"
const StatusResuming string = "Resuming"
const StatusRebooting string = "Rebooting"
const StatusTerminating string = "Terminating"

type KeyValue struct {
	Key   string
	Value string
}

// Structs for REST API

/*
type TbMcisReq struct {
	//Id     string    `json:"id"`
	Name   string    `json:"name"`
	Vm_req []TbVmReq `json:"vm_req"`
	//Vm_num         string  `json:"vm_num"`
	Placement_algo string `json:"placement_algo"`
	Description    string `json:"description"`
}
*/

type TbMcisInfo struct {
	// Fields for both request and response
	Name           string     `json:"name"`
	Vm             []TbVmInfo `json:"vm"`
	Placement_algo string     `json:"placement_algo"`
	Description    string     `json:"description"`

	// Additional fields for response
	Id           string `json:"id"`
	Status       string `json:"status"`
	TargetStatus string `json:"targetStatus"`
	TargetAction string `json:"targetAction"`

	// Disabled for now
	//Vm             []vmOverview `json:"vm"`
}

/*
type TbVmReq struct {
	//Id             string `json:"id"`
	//ConnectionName string `json:"connectionName"`

	// 1. Required by CB-Spider
	//CspVmName string `json:"cspVmName"` // will be deprecated

	CspImageName        string `json:"cspImageName"`
	CspVirtualNetworkId string `json:"cspVirtualNetworkId"`
	//CspNetworkInterfaceId string   `json:"cspNetworkInterfaceId"`
	//CspPublicIPId         string   `json:"cspPublicIPId"`
	CspSecurityGroupIds []string `json:"cspSecurityGroupIds"`
	CspSpecId           string   `json:"cspSpecId"`
	CspKeyPairName      string   `json:"cspKeyPairName"`

	CbImageId          string `json:"cbImageId"`
	CbVirtualNetworkId string `json:"cbVirtualNetworkId"`
	//CbNetworkInterfaceId string   `json:"cbNetworkInterfaceId"`
	//CbPublicIPId         string   `json:"cbPublicIPId"`
	CbSecurityGroupIds []string `json:"cbSecurityGroupIds"`
	CbSpecId           string   `json:"cbSpecId"`
	CbKeyPairId        string   `json:"cbKeyPairId"`

	VMUserId     string `json:"vmUserId"`
	VMUserPasswd string `json:"vmUserPasswd"`

	Name           string `json:"name"`
	ConnectionName string `json:"connectionName"`
	SpecId         string `json:"specId"`
	ImageId        string `json:"imageId"`
	VNetId         string `json:"vNetId"`
	SubnetId       string `json:"subnetId"`
	//Vnic_id            string   `json:"vnic_id"`
	//Public_ip_id       string   `json:"public_ip_id"`
	SecurityGroupIds []string `json:"securityGroupIds"`
	SshKeyId         string   `json:"sshKeyId"`
	Description      string   `json:"description"`
	VmUserAccount    string   `json:"vmUserAccount"`
	VmUserPassword   string   `json:"vmUserPassword"`
}
*/

type TbVmInfo struct {
	Id             string `json:"id"`
	Name           string `json:"name"`
	ConnectionName string `json:"connectionName"`
	SpecId         string `json:"specId"`
	ImageId        string `json:"imageId"`
	VNetId         string `json:"vNetId"`
	SubnetId       string `json:"subnetId"`
	//Vnic_id            string   `json:"vnic_id"`
	//Public_ip_id       string   `json:"public_ip_id"`
	SecurityGroupIds []string `json:"securityGroupIds"`
	SshKeyId         string   `json:"sshKeyId"`
	Description      string   `json:"description"`
	VmUserAccount    string   `json:"vmUserAccount"`
	VmUserPassword   string   `json:"vmUserPassword"`

	Location GeoLocation `json:"location"`

	// 2. Provided by CB-Spider
	Region      RegionInfo `json:"region"` // AWS, ex) {us-east1, us-east1-c} or {ap-northeast-2}
	PublicIP    string     `json:"publicIP"`
	PublicDNS   string     `json:"publicDNS"`
	PrivateIP   string     `json:"privateIP"`
	PrivateDNS  string     `json:"privateDNS"`
	VMBootDisk  string     `json:"vmBootDisk"` // ex) /dev/sda1
	VMBlockDisk string     `json:"vmBlockDisk"`

	// 3. Required by CB-Tumblebug
	Status       string `json:"status"`
	TargetStatus string `json:"targetStatus"`
	TargetAction string `json:"targetAction"`

	CspViewVmDetail SpiderVMInfo `json:"cspViewVmDetail"`
}

type vmOverview struct {
	Id             string     `json:"id"`
	Name           string     `json:"name"`
	ConnectionName string     `json:"connectionName"`
	Region         RegionInfo `json:"region"` // AWS, ex) {us-east1, us-east1-c} or {ap-northeast-2}
	PublicIP       string     `json:"publicIP"`
	PublicDNS      string     `json:"publicDNS"`
	Status         string     `json:"status"`
}

type RegionInfo struct {
	Region string
	Zone   string
}

type vmCspViewInfo struct {
	Name      string    // AWS,
	Id        string    // AWS,
	StartTime time.Time // Timezone: based on cloud-barista server location.

	Region           RegionInfo // AWS, ex) {us-east1, us-east1-c} or {ap-northeast-2}
	ImageId          string
	VMSpecId         string   // AWS, instance type or flavour, etc... ex) t2.micro or f1.micro
	VirtualNetworkId string   // AWS, ex) subnet-8c4a53e4
	SecurityGroupIds []string // AWS, ex) sg-0b7452563e1121bb6

	NetworkInterfaceId string // ex) eth0
	PublicIP           string // ex) AWS, 13.125.43.21
	PublicDNS          string // ex) AWS, ec2-13-125-43-0.ap-northeast-2.compute.amazonaws.com
	PrivateIP          string // ex) AWS, ip-172-31-4-60.ap-northeast-2.compute.internal
	PrivateDNS         string // ex) AWS, 172.31.4.60

	KeyPairName  string // ex) AWS, powerkimKeyPair
	VMUserId     string // ex) user1
	VMUserPasswd string

	VMBootDisk  string // ex) /dev/sda1
	VMBlockDisk string // ex)

	KeyValueList []KeyValue
}

type McisStatusInfo struct {
	Id   string `json:"id"`
	Name string `json:"name"`
	//Vm_num string         `json:"vm_num"`
	Status string           `json:"status"`
	Vm     []TbVmStatusInfo `json:"vm"`
}

type TbVmStatusInfo struct {
	Id        string `json:"id"`
	Csp_vm_id string `json:"csp_vm_id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Public_ip string `json:"public_ip"`
}

type McisRecommendReq struct {
	Vm_req          []TbVmRecommendReq `json:"vm_req"`
	Placement_algo  string             `json:"placement_algo"`
	Placement_param []KeyValue         `json:"placement_param"`
	Max_result_num  string             `json:"max_result_num"`
}

type TbVmRecommendReq struct {
	Request_name   string `json:"request_name"`
	Max_result_num string `json:"max_result_num"`

	Vcpu_size   string `json:"vcpu_size"`
	Memory_size string `json:"memory_size"`
	Disk_size   string `json:"disk_size"`
	//Disk_type   string `json:"disk_type"`

	Placement_algo  string     `json:"placement_algo"`
	Placement_param []KeyValue `json:"placement_param"`
}

type TbVmPriority struct {
	Priority string          `json:"priority"`
	Vm_spec  mcir.TbSpecInfo `json:"vm_spec"`
}
type TbVmRecommendInfo struct {
	Vm_req          TbVmRecommendReq `json:"vm_req"`
	Vm_priority     []TbVmPriority   `json:"vm_priority"`
	Placement_algo  string           `json:"placement_algo"`
	Placement_param []KeyValue       `json:"placement_param"`
}

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "tbctl",
	Short: "A brief description of your application",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(cfgFile)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.cli.yaml)")

	fmt.Printf("Setting config file: %s", cfgFile)

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.

		var configuration TbMcisReq

		viper.SetConfigFile(cfgFile)
		if err := viper.ReadInConfig(); err != nil {
			fmt.Printf("Error reading config file, %s", err)
		}
		err := viper.Unmarshal(&configuration)
		if err != nil {
			fmt.Printf("Unable to decode into struct, %v", err)
		}

		common.PrintJsonPretty(configuration)

		/*
		   var configuration conf.Configurations
		   // Use config file from the flag.
		   viper.SetConfigFile(cfgFile)

		   if err := viper.ReadInConfig(); err != nil {
		     fmt.Printf("Error reading config file, %s", err)
		   }
		     // Set undefined variables
		   viper.SetDefault("database.dbname", "test_db")

		   err := viper.Unmarshal(&configuration)
		   if err != nil {
		     fmt.Printf("Unable to decode into struct, %v", err)
		   }

		   // Reading variables using the model
		   fmt.Println("Reading variables using the model..")
		   fmt.Println("Database is\t", configuration.Database.DBName)
		   fmt.Println("Port is\t\t", configuration.Server.Port)
		   fmt.Println("EXAMPLE_PATH is\t", configuration.EXAMPLE_PATH)
		   fmt.Println("EXAMPLE_VAR is\t", configuration.EXAMPLE_VAR)

		   // Reading variables without using the model
		   fmt.Println("\nReading variables without using the model..")
		   fmt.Println("Database is\t", viper.GetString("database.dbname"))
		   fmt.Println("Port is\t\t", viper.GetInt("server.port"))
		   fmt.Println("EXAMPLE_PATH is\t", viper.GetString("EXAMPLE_PATH"))
		   fmt.Println("EXAMPLE_VAR is\t", viper.GetString("EXAMPLE_VAR"))
		*/
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".cli" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".cli")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
