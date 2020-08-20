package cmd

import (
	"github.com/spf13/cobra"

	"github.com/cloud-barista/cb-spider/api-runtime/grpc-runtime/logger"
)

// ===== [ Constants and Variables ] =====

// ===== [ Types ] =====

// ===== [ Implementations ] =====

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====

// NewRegionCmd - Region 관리 기능을 수행하는 Cobra Command 생성
func NewRegionCmd() *cobra.Command {

	regionCmd := &cobra.Command{
		Use:   "region",
		Short: "This is a manageable command for region",
		Long:  "This is a manageable command for region",
	}

	//  Adds the commands for application.
	regionCmd.AddCommand(NewRegionCreateCmd())
	regionCmd.AddCommand(NewRegionListCmd())
	regionCmd.AddCommand(NewRegionGetCmd())
	regionCmd.AddCommand(NewRegionDeleteCmd())

	return regionCmd
}

// NewRegionCreateCmd - Region 생성 기능을 수행하는 Cobra Command 생성
func NewRegionCreateCmd() *cobra.Command {

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "This is create command for region",
		Long:  "This is create command for region",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()
			readInDataFromFile()
			if inData == "" {
				logger.Error("failed to validate --indata parameter")
				return
			}
			logger.Debug("--indata parameter value : \n", inData)
			logger.Debug("--infile parameter value : ", inFile)

			SetupAndRun(cmd, args)
		},
	}

	createCmd.PersistentFlags().StringVarP(&inData, "indata", "d", "", "input string data")
	createCmd.PersistentFlags().StringVarP(&inFile, "infile", "f", "", "input file path")

	return createCmd
}

// NewRegionListCmd - Region 목록 기능을 수행하는 Cobra Command 생성
func NewRegionListCmd() *cobra.Command {

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "This is list command for region",
		Long:  "This is list command for region",
		Run: func(cmd *cobra.Command, args []string) {
			SetupAndRun(cmd, args)
		},
	}

	return listCmd
}

// NewRegionGetCmd - Region 조회 기능을 수행하는 Cobra Command 생성
func NewRegionGetCmd() *cobra.Command {

	getCmd := &cobra.Command{
		Use:   "get",
		Short: "This is get command for region",
		Long:  "This is get command for region",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()
			if regionName == "" {
				logger.Error("failed to validate --name parameter")
				return
			}
			logger.Debug("--name parameter value : ", regionName)

			SetupAndRun(cmd, args)
		},
	}

	getCmd.PersistentFlags().StringVarP(&regionName, "name", "n", "", "region name")

	return getCmd
}

// NewRegionDeleteCmd - Region 삭제 기능을 수행하는 Cobra Command 생성
func NewRegionDeleteCmd() *cobra.Command {

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "This is delete command for region",
		Long:  "This is delete command for region",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()
			if regionName == "" {
				logger.Error("failed to validate --name parameter")
				return
			}
			logger.Debug("--name parameter value : ", regionName)

			SetupAndRun(cmd, args)
		},
	}

	deleteCmd.PersistentFlags().StringVarP(&regionName, "name", "n", "", "region name")

	return deleteCmd
}
