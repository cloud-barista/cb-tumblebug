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

// NewDriverCmd - Cloud Driver 관리 기능을 수행하는 Cobra Command 생성
func NewDriverCmd() *cobra.Command {

	driverCmd := &cobra.Command{
		Use:   "driver",
		Short: "This is a manageable command for cloud driver",
		Long:  "This is a manageable command for cloud driver",
	}

	//  Adds the commands for application.
	driverCmd.AddCommand(NewDriverCreateCmd())
	driverCmd.AddCommand(NewDriverListCmd())
	driverCmd.AddCommand(NewDriverGetCmd())
	driverCmd.AddCommand(NewDriverDeleteCmd())

	return driverCmd
}

// NewDriverCreateCmd - Cloud Driver 생성 기능을 수행하는 Cobra Command 생성
func NewDriverCreateCmd() *cobra.Command {

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "This is create command for cloud driver",
		Long:  "This is create command for cloud driver",
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

// NewDriverListCmd - Cloud Driver 목록 기능을 수행하는 Cobra Command 생성
func NewDriverListCmd() *cobra.Command {

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "This is list command for cloud driver",
		Long:  "This is list command for cloud driver",
		Run: func(cmd *cobra.Command, args []string) {
			SetupAndRun(cmd, args)
		},
	}

	return listCmd
}

// NewDriverGetCmd - Cloud Driver 조회 기능을 수행하는 Cobra Command 생성
func NewDriverGetCmd() *cobra.Command {

	getCmd := &cobra.Command{
		Use:   "get",
		Short: "This is get command for cloud driver",
		Long:  "This is get command for cloud driver",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()
			if driverName == "" {
				logger.Error("failed to validate --name parameter")
				return
			}
			logger.Debug("--name parameter value : ", driverName)

			SetupAndRun(cmd, args)
		},
	}

	getCmd.PersistentFlags().StringVarP(&driverName, "name", "n", "", "driver name")

	return getCmd
}

// NewDriverDeleteCmd - Cloud Driver 삭제 기능을 수행하는 Cobra Command 생성
func NewDriverDeleteCmd() *cobra.Command {

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "This is delete command for cloud driver",
		Long:  "This is delete command for cloud driver",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()
			if driverName == "" {
				logger.Error("failed to validate --name parameter")
				return
			}
			logger.Debug("--name parameter value : ", driverName)

			SetupAndRun(cmd, args)
		},
	}

	deleteCmd.PersistentFlags().StringVarP(&driverName, "name", "n", "", "driver name")

	return deleteCmd
}
