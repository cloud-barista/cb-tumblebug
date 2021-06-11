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

// NewSpecCmd : "cbadm spec *" (for CB-Tumblebug)
func NewSpecCmd() *cobra.Command {

	specCmd := &cobra.Command{
		Use:   "spec",
		Short: "This is a manageable command for spec",
		Long:  "This is a manageable command for spec",
	}

	//  Adds the commands for application.
	specCmd.AddCommand(NewSpecWithInfoCreateCmd())
	specCmd.AddCommand(NewSpecWithIdCreateCmd())
	specCmd.AddCommand(NewSpecListCmd())
	specCmd.AddCommand(NewSpecListIdCmd())
	specCmd.AddCommand(NewSpecListCspCmd())
	specCmd.AddCommand(NewSpecGetCmd())
	specCmd.AddCommand(NewSpecGetCspCmd())
	specCmd.AddCommand(NewSpecDeleteCmd())
	specCmd.AddCommand(NewSpecDeleteAllCmd())
	specCmd.AddCommand(NewSpecFetchCmd())
	specCmd.AddCommand(NewSpecFilterCmd())
	specCmd.AddCommand(NewSpecFilterByRangeCmd())
	specCmd.AddCommand(NewSpecSortCmd())
	specCmd.AddCommand(NewSpecUpdateCmd())

	return specCmd
}

// NewSpecWithInfoCreateCmd : "cbadm spec create"
func NewSpecWithInfoCreateCmd() *cobra.Command {

	createWithInfoCmd := &cobra.Command{
		Use:   "create",
		Short: "This is create command for spec",
		Long:  "This is create command for spec",
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

// NewSpecWithIdCreateCmd : "cbadm spec create-id"
func NewSpecWithIdCreateCmd() *cobra.Command {

	createWithIdCmd := &cobra.Command{
		Use:   "create-id",
		Short: "This is create-id command for spec",
		Long:  "This is create-id command for spec",
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

// NewSpecListCmd : "cbadm spec list"
func NewSpecListCmd() *cobra.Command {

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "This is list command for spec",
		Long:  "This is list command for spec",
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

// NewSpecListIdCmd : "cbadm spec list-id"
func NewSpecListIdCmd() *cobra.Command {

	listIdCmd := &cobra.Command{
		Use:   "list-id",
		Short: "This is list-id command for spec",
		Long:  "This is list-id command for spec",
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

	listIdCmd.PersistentFlags().StringVarP(&nameSpaceID, "ns", "", "", "namespace id")

	return listIdCmd
}

// NewSpecListCspCmd : "cbadm spec list-csp"
func NewSpecListCspCmd() *cobra.Command {

	listCspCmd := &cobra.Command{
		Use:   "list-csp",
		Short: "This is list-csp command for spec",
		Long:  "This is list-csp command for spec",
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

// NewSpecGetCmd : "cbadm spec get"
func NewSpecGetCmd() *cobra.Command {

	getCmd := &cobra.Command{
		Use:   "get",
		Short: "This is get command for spec",
		Long:  "This is get command for spec",
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
	getCmd.PersistentFlags().StringVarP(&resourceID, "id", "", "", "spec id")

	return getCmd
}

// NewSpecGetCspCmd : "cbadm spec get-csp"
func NewSpecGetCspCmd() *cobra.Command {

	getCspCmd := &cobra.Command{
		Use:   "get-csp",
		Short: "This is get-csp command for spec",
		Long:  "This is get-csp command for spec",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()
			if connConfigName == "" {
				logger.Error("failed to validate --cc parameter")
				return
			}
			if cspSpecName == "" {
				logger.Error("failed to validate --spec parameter")
				return
			}
			logger.Debug("--cc parameter value : ", connConfigName)
			logger.Debug("--spec parameter value : ", cspSpecName)

			SetupAndRun(cmd, args)
		},
	}

	getCspCmd.PersistentFlags().StringVarP(&connConfigName, "cc", "", "", "connection name")
	getCspCmd.PersistentFlags().StringVarP(&cspSpecName, "spec", "", "", "csp spec name")

	return getCspCmd
}

// NewSpecDeleteCmd : "cbadm spec delete"
func NewSpecDeleteCmd() *cobra.Command {

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "This is delete command for spec",
		Long:  "This is delete command for spec",
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
	deleteCmd.PersistentFlags().StringVarP(&resourceID, "id", "", "", "spec id")
	deleteCmd.PersistentFlags().StringVarP(&force, "force", "", "false", "force flag")

	return deleteCmd
}

// NewSpecDeleteAllCmd : "cbadm spec delete-all"
func NewSpecDeleteAllCmd() *cobra.Command {

	deleteAllCmd := &cobra.Command{
		Use:   "delete-all",
		Short: "This is delete-all command for spec",
		Long:  "This is delete-all command for spec",
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

// NewSpecFetchCmd : "cbadm spec fetch"
func NewSpecFetchCmd() *cobra.Command {

	fetchCmd := &cobra.Command{
		Use:   "fetch",
		Short: "This is fetch command for spec",
		Long:  "This is fetch command for spec",
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

// NewSpecFilterCmd : "cbadm spec filter"
func NewSpecFilterCmd() *cobra.Command {

	filterCmd := &cobra.Command{
		Use:   "filter",
		Short: "This is filter command for spec",
		Long:  "This is filter command for spec",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()
			/*
				if nameSpaceID == "" {
					logger.Error("failed to validate --ns parameter")
					return
				}
				logger.Debug("--ns parameter value : ", nameSpaceID)
			*/
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

	//filterCmd.PersistentFlags().StringVarP(&nameSpaceID, "ns", "", "", "namespace id")
	filterCmd.PersistentFlags().StringVarP(&inData, "indata", "d", "", "input string data")
	filterCmd.PersistentFlags().StringVarP(&inFile, "infile", "f", "", "input file path")

	return filterCmd
}

// NewSpecFilterByRangeCmd : "cbadm spec filter-by-range"
func NewSpecFilterByRangeCmd() *cobra.Command {

	filterByRangeCmd := &cobra.Command{
		Use:   "filter-by-range",
		Short: "This is filter-by-range command for spec",
		Long:  "This is filter-by-range command for spec",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()
			/*
				if nameSpaceID == "" {
					logger.Error("failed to validate --ns parameter")
					return
				}
				logger.Debug("--ns parameter value : ", nameSpaceID)
			*/
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

	//filterByRangeCmd.PersistentFlags().StringVarP(&nameSpaceID, "ns", "", "", "namespace id")
	filterByRangeCmd.PersistentFlags().StringVarP(&inData, "indata", "d", "", "input string data")
	filterByRangeCmd.PersistentFlags().StringVarP(&inFile, "infile", "f", "", "input file path")

	return filterByRangeCmd
}

// NewSpecSortCmd : "cbadm spec sort"
func NewSpecSortCmd() *cobra.Command {

	sortCmd := &cobra.Command{
		Use:   "sort",
		Short: "This is sort command for spec",
		Long:  "This is sort command for spec",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()
			/*
				if nameSpaceID == "" {
					logger.Error("failed to validate --ns parameter")
					return
				}
				logger.Debug("--ns parameter value : ", nameSpaceID)
			*/
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

	//sortCmd.PersistentFlags().StringVarP(&nameSpaceID, "ns", "", "", "namespace id")
	sortCmd.PersistentFlags().StringVarP(&inData, "indata", "d", "", "input string data")
	sortCmd.PersistentFlags().StringVarP(&inFile, "infile", "f", "", "input file path")

	return sortCmd
}

// NewSpecUpdateCmd : "cbadm spec update"
func NewSpecUpdateCmd() *cobra.Command {

	updateCmd := &cobra.Command{
		Use:   "update",
		Short: "This is update command for spec",
		Long:  "This is update command for spec",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()
			/*
				if nameSpaceID == "" {
					logger.Error("failed to validate --ns parameter")
					return
				}
				logger.Debug("--ns parameter value : ", nameSpaceID)
			*/
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

	//updateCmd.PersistentFlags().StringVarP(&nameSpaceID, "ns", "", "", "namespace id")
	updateCmd.PersistentFlags().StringVarP(&inData, "indata", "d", "", "input string data")
	updateCmd.PersistentFlags().StringVarP(&inFile, "infile", "f", "", "input file path")

	return updateCmd
}
