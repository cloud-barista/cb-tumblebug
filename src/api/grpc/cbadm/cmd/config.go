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

// NewConfigCmd : "cbadm config *" (for CB-Tumblebug)
func NewConfigCmd() *cobra.Command {

	configCmd := &cobra.Command{
		Use:   "config",
		Short: "This is a manageable command for config",
		Long:  "This is a manageable command for config",
	}

	//  Adds the commands for application.
	configCmd.AddCommand(NewConfigCreateCmd())
	configCmd.AddCommand(NewConfigListCmd())
	configCmd.AddCommand(NewConfigGetCmd())
	configCmd.AddCommand(NewConfigDeleteAllCmd())

	return configCmd
}

// NewConfigCreateCmd : "cbadm config create"
func NewConfigCreateCmd() *cobra.Command {

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "This is create command for config",
		Long:  "This is create command for config",
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

// NewConfigListCmd : "cbadm config list"
func NewConfigListCmd() *cobra.Command {

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "This is list command for config",
		Long:  "This is list command for config",
		Run: func(cmd *cobra.Command, args []string) {
			SetupAndRun(cmd, args)
		},
	}

	return listCmd
}

// NewConfigGetCmd : "cbadm config get"
func NewConfigGetCmd() *cobra.Command {

	getCmd := &cobra.Command{
		Use:   "get",
		Short: "This is get command for config",
		Long:  "This is get command for config",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()
			if configId == "" {
				logger.Error("failed to validate --id parameter")
				return
			}
			logger.Debug("--id parameter value : ", configId)

			SetupAndRun(cmd, args)
		},
	}

	getCmd.PersistentFlags().StringVarP(&configId, "id", "", "", "config id")

	return getCmd
}

// NewConfigDeleteAllCmd : "cbadm config delete-all"
func NewConfigDeleteAllCmd() *cobra.Command {

	deleteAllCmd := &cobra.Command{
		Use:   "delete-all",
		Short: "This is delete all command for config",
		Long:  "This is delete all command for config",
		Run: func(cmd *cobra.Command, args []string) {

			SetupAndRun(cmd, args)
		},
	}

	return deleteAllCmd
}
