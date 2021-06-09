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

// NewNetworkCmd : "cbadm network *" (for CB-Tumblebug)
func NewNetworkCmd() *cobra.Command {

	networkCmd := &cobra.Command{
		Use:     "network",
		Short:   "This is a manageable command for network",
		Long:    "This is a manageable command for network",
		Aliases: []string{"vnet", "net", "vpc"},
	}

	//  Adds the commands for application.
	networkCmd.AddCommand(NewNetworkCreateCmd())
	networkCmd.AddCommand(NewNetworkListCmd())
	networkCmd.AddCommand(NewNetworkGetCmd())
	networkCmd.AddCommand(NewNetworkDeleteCmd())
	networkCmd.AddCommand(NewNetworkDeleteAllCmd())

	return networkCmd
}

// NewNetworkCreateCmd : "cbadm network create"
func NewNetworkCreateCmd() *cobra.Command {

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "This is create command for network",
		Long:  "This is create command for network",
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

// NewNetworkListCmd : "cbadm network list"
func NewNetworkListCmd() *cobra.Command {

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "This is list command for network",
		Long:  "This is list command for network",
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

	listCmd.PersistentFlags().StringVarP(&nameSpaceID, "ns", "", "", "namespace id")

	return listCmd
}

// NewNetworkGetCmd : "cbadm network get"
func NewNetworkGetCmd() *cobra.Command {

	getCmd := &cobra.Command{
		Use:   "get",
		Short: "This is get command for network",
		Long:  "This is get command for network",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()
			if nameSpaceID == "" {
				logger.Error("failed to validate --ns parameter")
				return
			}
			if resourceID == "" {
				logger.Error("failed to validate --id parameter")
				return
			}
			logger.Debug("--ns parameter value : ", nameSpaceID)
			logger.Debug("--id parameter value : ", resourceID)

			SetupAndRun(cmd, args)
		},
	}

	getCmd.PersistentFlags().StringVarP(&nameSpaceID, "ns", "", "", "namespace id")
	getCmd.PersistentFlags().StringVarP(&resourceID, "id", "", "", "network id")

	return getCmd
}

// NewNetworkDeleteCmd : "cbadm network delete"
func NewNetworkDeleteCmd() *cobra.Command {

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "This is delete command for network",
		Long:  "This is delete command for network",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()
			if nameSpaceID == "" {
				logger.Error("failed to validate --ns parameter")
				return
			}
			if resourceID == "" {
				logger.Error("failed to validate --id parameter")
				return
			}
			if force == "" {
				logger.Error("failed to validate --force parameter")
				return
			}
			logger.Debug("--ns parameter value : ", nameSpaceID)
			logger.Debug("--id parameter value : ", resourceID)
			logger.Debug("--force parameter value : ", force)

			SetupAndRun(cmd, args)
		},
	}

	deleteCmd.PersistentFlags().StringVarP(&nameSpaceID, "ns", "", "", "namespace id")
	deleteCmd.PersistentFlags().StringVarP(&resourceID, "id", "", "", "network id")
	deleteCmd.PersistentFlags().StringVarP(&force, "force", "", "false", "force flag")

	return deleteCmd
}

// NewNetworkDeleteAllCmd : "cbadm network delete-all"
func NewNetworkDeleteAllCmd() *cobra.Command {

	deleteAllCmd := &cobra.Command{
		Use:   "delete-all",
		Short: "This is delete-all command for network",
		Long:  "This is delete-all command for network",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()
			if nameSpaceID == "" {
				logger.Error("failed to validate --ns parameter")
				return
			}
			if force == "" {
				logger.Error("failed to validate --force parameter")
				return
			}
			logger.Debug("--ns parameter value : ", nameSpaceID)
			logger.Debug("--force parameter value : ", force)

			SetupAndRun(cmd, args)
		},
	}

	deleteAllCmd.PersistentFlags().StringVarP(&nameSpaceID, "ns", "", "", "namespace id")
	deleteAllCmd.PersistentFlags().StringVarP(&force, "force", "", "false", "force flag")

	return deleteAllCmd
}
