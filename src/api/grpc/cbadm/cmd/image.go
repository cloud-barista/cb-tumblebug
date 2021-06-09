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

// NewImageCmd : "cbadm image *" (for CB-Tumblebug)
func NewImageCmd() *cobra.Command {

	imageCmd := &cobra.Command{
		Use:   "image",
		Short: "This is a manageable command for image",
		Long:  "This is a manageable command for image",
	}

	//  Adds the commands for application.
	imageCmd.AddCommand(NewImageCreateWithInfoCmd())
	imageCmd.AddCommand(NewImageCreateWithIdCmd())
	imageCmd.AddCommand(NewImageListCmd())
	imageCmd.AddCommand(NewImageListCspCmd())
	imageCmd.AddCommand(NewImageGetCmd())
	imageCmd.AddCommand(NewImageGetCspCmd())
	imageCmd.AddCommand(NewImageDeleteCmd())
	imageCmd.AddCommand(NewImageDeleteAllCmd())
	imageCmd.AddCommand(NewImageFetchCmd())
	imageCmd.AddCommand(NewImageSearchCmd())

	return imageCmd
}

// NewImageCreateWithInfoCmd : "cbadm image create"
func NewImageCreateWithInfoCmd() *cobra.Command {

	createWithInfoCmd := &cobra.Command{
		Use:   "create",
		Short: "This is create command for image",
		Long:  "This is create command for image",
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

	createWithInfoCmd.PersistentFlags().StringVarP(&inData, "indata", "d", "", "input string data")
	createWithInfoCmd.PersistentFlags().StringVarP(&inFile, "infile", "f", "", "input file path")

	return createWithInfoCmd
}

// NewImageCreateWithIdCmd : "cbadm image create-id"
func NewImageCreateWithIdCmd() *cobra.Command {

	createWithIdCmd := &cobra.Command{
		Use:   "create-id",
		Short: "This is create-id command for image",
		Long:  "This is create-id command for image",
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

	createWithIdCmd.PersistentFlags().StringVarP(&inData, "indata", "d", "", "input string data")
	createWithIdCmd.PersistentFlags().StringVarP(&inFile, "infile", "f", "", "input file path")

	return createWithIdCmd
}

// NewImageListCmd : "cbadm image list"
func NewImageListCmd() *cobra.Command {

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "This is list command for image",
		Long:  "This is list command for image",
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

// NewImageListCspCmd : "cbadm image list-csp"
func NewImageListCspCmd() *cobra.Command {

	listCspCmd := &cobra.Command{
		Use:   "list-csp",
		Short: "This is list-csp command for image",
		Long:  "This is list-csp command for image",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()
			if connConfigName == "" {
				logger.Error("failed to validate --cc parameter")
				return
			}
			logger.Debug("--cc parameter value : ", connConfigName)

			SetupAndRun(cmd, args)
		},
	}

	listCspCmd.PersistentFlags().StringVarP(&connConfigName, "cc", "", "", "connection name")

	return listCspCmd
}

// NewImageGetCmd : "cbadm image get"
func NewImageGetCmd() *cobra.Command {

	getCmd := &cobra.Command{
		Use:   "get",
		Short: "This is get command for image",
		Long:  "This is get command for image",
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
	getCmd.PersistentFlags().StringVarP(&resourceID, "id", "", "", "image id")

	return getCmd
}

// NewImageGetCspCmd : "cbadm image get-csp"
func NewImageGetCspCmd() *cobra.Command {

	getCspCmd := &cobra.Command{
		Use:   "get-csp",
		Short: "This is get-csp command for image",
		Long:  "This is get-csp command for image",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()
			if connConfigName == "" {
				logger.Error("failed to validate --cc parameter")
				return
			}
			if cspImageId == "" {
				logger.Error("failed to validate --image parameter")
				return
			}
			logger.Debug("--cc parameter value : ", connConfigName)
			logger.Debug("--image parameter value : ", cspImageId)

			SetupAndRun(cmd, args)
		},
	}

	getCspCmd.PersistentFlags().StringVarP(&connConfigName, "cc", "", "", "connection name")
	getCspCmd.PersistentFlags().StringVarP(&cspImageId, "image", "", "", "csp image id")

	return getCspCmd
}

// NewImageDeleteCmd : "cbadm image delete"
func NewImageDeleteCmd() *cobra.Command {

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "This is delete command for image",
		Long:  "This is delete command for image",
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
	deleteCmd.PersistentFlags().StringVarP(&resourceID, "id", "", "", "image id")
	deleteCmd.PersistentFlags().StringVarP(&force, "force", "", "false", "force flag")

	return deleteCmd
}

// NewImageDeleteAllCmd : "cbadm image delete-all"
func NewImageDeleteAllCmd() *cobra.Command {

	deleteAllCmd := &cobra.Command{
		Use:   "delete-all",
		Short: "This is delete-all command for image",
		Long:  "This is delete-all command for image",
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

// NewImageFetchCmd : "cbadm image fetch"
func NewImageFetchCmd() *cobra.Command {

	fetchCmd := &cobra.Command{
		Use:   "fetch",
		Short: "This is fetch command for image",
		Long:  "This is fetch command for image",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()
			if connConfigName == "" {
				logger.Error("failed to validate --cc parameter")
				return
			}
			if nameSpaceID == "" {
				logger.Error("failed to validate --ns parameter")
				return
			}
			logger.Debug("--cc parameter value : ", connConfigName)
			logger.Debug("--ns parameter value : ", nameSpaceID)

			SetupAndRun(cmd, args)
		},
	}

	fetchCmd.PersistentFlags().StringVarP(&connConfigName, "cc", "", "", "connection name")
	fetchCmd.PersistentFlags().StringVarP(&nameSpaceID, "ns", "", "", "namespace id")

	return fetchCmd
}

// NewImageSearchCmd : "cbadm image search"
func NewImageSearchCmd() *cobra.Command {

	searchCmd := &cobra.Command{
		Use:   "search",
		Short: "This is search command for image",
		Long:  "This is search command for image",
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

	searchCmd.PersistentFlags().StringVarP(&inData, "indata", "d", "", "input string data")
	searchCmd.PersistentFlags().StringVarP(&inFile, "infile", "f", "", "input file path")

	return searchCmd
}
