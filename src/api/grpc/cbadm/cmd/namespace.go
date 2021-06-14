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

// NewNameSpaceCmd : "cbadm namespace *" (for CB-Tumblebug)
func NewNameSpaceCmd() *cobra.Command {

	nameSpaceCmd := &cobra.Command{
		Use:     "namespace",
		Aliases: []string{"ns"},
		Short:   "This is a manageable command for namespace",
		Long:    "This is a manageable command for namespace",
	}

	//  Adds the commands for application.
	nameSpaceCmd.AddCommand(NewNameSpaceCreateCmd())
	nameSpaceCmd.AddCommand(NewNameSpaceListCmd())
	nameSpaceCmd.AddCommand(NewNameSpaceListIdCmd())
	nameSpaceCmd.AddCommand(NewNameSpaceGetCmd())
	nameSpaceCmd.AddCommand(NewNameSpaceDeleteCmd())
	nameSpaceCmd.AddCommand(NewNameSpaceDeleteAllCmd())
	nameSpaceCmd.AddCommand(NewNameSpaceCheckCmd())

	return nameSpaceCmd
}

// NewNameSpaceCreateCmd : "cbadm namespace create"
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

// NewNameSpaceListCmd : "cbadm namespace list"
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

// NewNameSpaceListIdCmd : "cbadm namespace list-id"
func NewNameSpaceListIdCmd() *cobra.Command {

	listIdCmd := &cobra.Command{
		Use:   "list-id",
		Short: "This is list-id command for namespace",
		Long:  "This is list-id command for namespace",
		Run: func(cmd *cobra.Command, args []string) {
			SetupAndRun(cmd, args)
		},
	}

	return listIdCmd
}

// NewNameSpaceGetCmd : "cbadm namespace get"
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

// NewNameSpaceDeleteCmd : "cbadm namespace delete"
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

// NewNameSpaceDeleteAllCmd : "cbadm namespace delete-all"
func NewNameSpaceDeleteAllCmd() *cobra.Command {

	deleteAllCmd := &cobra.Command{
		Use:   "delete-all",
		Short: "This is delete-all command for namespace",
		Long:  "This is delete-all command for namespace",
		Run: func(cmd *cobra.Command, args []string) {
			SetupAndRun(cmd, args)
		},
	}

	return deleteAllCmd
}

// NewNameSpaceCheckCmd : "cbadm namespace check"
func NewNameSpaceCheckCmd() *cobra.Command {

	checkCmd := &cobra.Command{
		Use:   "check",
		Short: "This is check command for namespace",
		Long:  "This is check command for namespace",
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

	checkCmd.PersistentFlags().StringVarP(&nameSpaceID, "ns", "", "", "namespace id")

	return checkCmd
}
