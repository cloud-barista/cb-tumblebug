package cmd

import (
	"github.com/spf13/cobra"

	"github.com/cloud-barista/cb-tumblebug/src/api/grpc/logger"
)

// ===== [ Constants and Variables ] =====

// ===== [ Types ] =====

// ===== [ Implementations ] =====

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====

// NewNameSpaceCmd - Namespace 관리 기능을 수행하는 Cobra Command 생성
func NewNameSpaceCmd() *cobra.Command {

	nameSpaceCmd := &cobra.Command{
		Use:   "namespaces",
		Short: "This is a manageable command for namespace",
		Long:  "This is a manageable command for namespace",
	}

	//  Adds the commands for application.
	nameSpaceCmd.AddCommand(NewNameSpaceCreateCmd())
	nameSpaceCmd.AddCommand(NewNameSpaceListCmd())
	nameSpaceCmd.AddCommand(NewNameSpaceGetCmd())
	nameSpaceCmd.AddCommand(NewNameSpaceDeleteCmd())

	return nameSpaceCmd
}

// NewNameSpaceCreateCmd - Namespace 생성 기능을 수행하는 Cobra Command 생성
func NewNameSpaceCreateCmd() *cobra.Command {

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "This is create command for namespace",
		Long:  "This is create command for namespace",
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

// NewNameSpaceListCmd - Namespace 목록 기능을 수행하는 Cobra Command 생성
func NewNameSpaceListCmd() *cobra.Command {

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "This is list command for namespace",
		Long:  "This is list command for namespace",
		Run: func(cmd *cobra.Command, args []string) {
			SetupAndRun(cmd, args)
		},
	}

	return listCmd
}

// NewNameSpaceGetCmd - Namespace 조회 기능을 수행하는 Cobra Command 생성
func NewNameSpaceGetCmd() *cobra.Command {

	getCmd := &cobra.Command{
		Use:   "get",
		Short: "This is get command for namespace",
		Long:  "This is get command for namespace",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()
			if nameSpaceID == "" {
				logger.Error("failed to validate --ns parameter")
				return
			}
			logger.Debug("--ns parameter value : ", nameSpaceID)

			SetupAndRun(cmd, args)
		},
	}

	getCmd.PersistentFlags().StringVarP(&nameSpaceID, "ns", "", "", "namespace id")

	return getCmd
}

// NewNameSpaceDeleteCmd - Namespace 삭제 기능을 수행하는 Cobra Command 생성
func NewNameSpaceDeleteCmd() *cobra.Command {

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "This is delete command for namespace",
		Long:  "This is delete command for namespace",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()
			if nameSpaceID == "" {
				logger.Error("failed to validate --ns parameter")
				return
			}
			logger.Debug("--ns parameter value : ", nameSpaceID)

			SetupAndRun(cmd, args)
		},
	}

	deleteCmd.PersistentFlags().StringVarP(&nameSpaceID, "ns", "", "", "namespace id")

	return deleteCmd
}
