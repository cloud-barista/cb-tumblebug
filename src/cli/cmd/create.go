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

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/cloud-barista/cb-tumblebug/src/common"
)

var fileStr string
var typeStr string

// createCmd represents the create command
var createCmd = &cobra.Command{
	Use:   "create",
	Short: "create based on file",
	Long: `create based on file. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("create called")
		//fmt.Println(typeStr)
		// get the flag value, its default value is false

		if fileStr == "" {
			fmt.Println("file is required")
		} else {
			
			var configuration TbMcisReq

    		viper.SetConfigFile(fileStr)
			if err := viper.ReadInConfig(); err != nil {
			fmt.Printf("Error reading config file, %s", err)
			}
			err := viper.Unmarshal(&configuration)
			if err != nil {
			fmt.Printf("Unable to decode into struct, %v", err)
			}

			common.PrintJsonPretty(configuration)

		}

		if typeStr == "" {
			fmt.Println("typeStr is empty")
		} else if typeStr == "ns" {
			fmt.Println("ns")
		} else if typeStr == "mcir" {
			fmt.Println("mcir")
		} else if typeStr == "mcis" {
			fmt.Println("mcis")
		} else {
			fmt.Println("wrong")
		}

	},
}

func init() {
	rootCmd.AddCommand(createCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// createCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	//createCmd.Flags().BoolP("type", "t", false, "Type: NS, MCIR, MCIS")
	createCmd.PersistentFlags().StringVarP(&fileStr, "file", "f", "*.json", "Location of config file")
	createCmd.PersistentFlags().StringVarP(&typeStr, "type", "t", "default", "Type: ns, mcir, mcis")
}
