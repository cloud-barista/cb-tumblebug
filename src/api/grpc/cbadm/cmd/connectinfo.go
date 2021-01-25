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

// NewConnectInfosCmd - 연결설정 관리 기능을 수행하는 Cobra Command 생성
func NewConnectInfosCmd() *cobra.Command {

	connectionCmd := &cobra.Command{
		Use:   "connect-info",
		Short: "This is a manageable command for connection config",
		Long:  "This is a manageable command for connection config",
	}

	//  Adds the commands for application.
	connectionCmd.AddCommand(NewConnectInfosCreateCmd())
	connectionCmd.AddCommand(NewConnectInfosListCmd())
	connectionCmd.AddCommand(NewCConnectInfosGetCmd())
	connectionCmd.AddCommand(NewConnectInfosDeleteCmd())

	return connectionCmd
}

// NewConnectInfosCreateCmd - 연결설정 생성 기능을 수행하는 Cobra Command 생성
func NewConnectInfosCreateCmd() *cobra.Command {

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "This is create command for connection config",
		Long:  "This is create command for connection config",
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

// NewConnectInfosListCmd - 연결설정 목록 기능을 수행하는 Cobra Command 생성
func NewConnectInfosListCmd() *cobra.Command {

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "This is list command for connection config",
		Long:  "This is list command for connection config",
		Run: func(cmd *cobra.Command, args []string) {
			SetupAndRun(cmd, args)
		},
	}

	return listCmd
}

// NewCConnectInfosGetCmd - 연결설정 조회 기능을 수행하는 Cobra Command 생성
func NewCConnectInfosGetCmd() *cobra.Command {

	getCmd := &cobra.Command{
		Use:   "get",
		Short: "This is get command for connection config",
		Long:  "This is get command for connection config",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()
			if configName == "" {
				logger.Error("failed to validate --name parameter")
				return
			}
			logger.Debug("--name parameter value : ", configName)

			SetupAndRun(cmd, args)
		},
	}

	getCmd.PersistentFlags().StringVarP(&configName, "name", "n", "", "config name")

	return getCmd
}

// NewConnectInfosDeleteCmd - 연결설정 삭제 기능을 수행하는 Cobra Command 생성
func NewConnectInfosDeleteCmd() *cobra.Command {

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "This is delete command for connection config",
		Long:  "This is delete command for connection config",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()
			if configName == "" {
				logger.Error("failed to validate --name parameter")
				return
			}
			logger.Debug("--name parameter value : ", configName)

			SetupAndRun(cmd, args)
		},
	}

	deleteCmd.PersistentFlags().StringVarP(&configName, "name", "n", "", "config name")

	return deleteCmd
}
