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

// NewUtilCmd : "cbadm util *" (for CB-Tumblebug)
func NewUtilCmd() *cobra.Command {

	utilCmd := &cobra.Command{
		Use:   "util",
		Short: "This is a manageable command for tumblebug utility",
		Long:  "This is a manageable command for tumblebug utility",
	}

	//  Adds the commands for application.
	utilCmd.AddCommand(NewConnConfigListCmd())
	utilCmd.AddCommand(NewConnConfigGetCmd())

	utilCmd.AddCommand(NewRegionSpiderListCmd())
	utilCmd.AddCommand(NewRegionSpiderGetCmd())

	utilCmd.AddCommand(NewMcirResourcesInspectCmd())
	utilCmd.AddCommand(NewVmResourcesInspectCmd())

	utilCmd.AddCommand(NewObjectListCmd())
	utilCmd.AddCommand(NewObjectGetCmd())
	utilCmd.AddCommand(NewObjectDeleteCmd())
	utilCmd.AddCommand(NewObjectDeleteAllCmd())

	return utilCmd
}

// NewConnConfigListCmd : "cbadm util list-cc"
func NewConnConfigListCmd() *cobra.Command {

	listCCCmd := &cobra.Command{
		Use:   "list-cc",
		Short: "This is list-cc command for tumblebug utility",
		Long:  "This is list-cc command for tumblebug utility",
		Run: func(cmd *cobra.Command, args []string) {
			SetupAndRun(cmd, args)
		},
	}

	return listCCCmd
}

// NewConnConfigGetCmd : "cbadm util get-cc"
func NewConnConfigGetCmd() *cobra.Command {

	getCCCmd := &cobra.Command{
		Use:   "get-cc",
		Short: "This is get-cc command for tumblebug utility",
		Long:  "This is get-cc command for tumblebug utility",
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

	getCCCmd.PersistentFlags().StringVarP(&connConfigName, "cc", "", "", "connection config name")

	return getCCCmd
}

// NewRegionSpiderListCmd : "cbadm util list-region"
func NewRegionSpiderListCmd() *cobra.Command {

	listRegionCmd := &cobra.Command{
		Use:   "list-region",
		Short: "This is list-region command for tumblebug utility",
		Long:  "This is list-region command for tumblebug utility",
		Run: func(cmd *cobra.Command, args []string) {
			SetupAndRun(cmd, args)
		},
	}

	return listRegionCmd
}

// NewRegionSpiderGetCmd : "cbadm util get-region"
func NewRegionSpiderGetCmd() *cobra.Command {

	getRegionCmd := &cobra.Command{
		Use:   "get-region",
		Short: "This is get-region command for tumblebug utility",
		Long:  "This is get-region command for tumblebug utility",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()
			if regionName == "" {
				logger.Error("failed to validate --region parameter")
				return
			}
			logger.Debug("--region parameter value : ", regionName)

			SetupAndRun(cmd, args)
		},
	}

	getRegionCmd.PersistentFlags().StringVarP(&regionName, "region", "", "", "region name")

	return getRegionCmd
}

// NewMcirResourcesInspectCmd : "cbadm util inspect-mcir"
func NewMcirResourcesInspectCmd() *cobra.Command {

	inspectMcirCmd := &cobra.Command{
		Use:   "inspect-mcir",
		Short: "This is inspect-mcir command for tumblebug utility",
		Long:  "This is inspect-mcir command for tumblebug utility",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()
			if connConfigName == "" {
				logger.Error("failed to validate --cc parameter")
				return
			}
			if resourceType == "" {
				logger.Error("failed to validate --type parameter")
				return
			}
			logger.Debug("--cc parameter value : ", connConfigName)
			logger.Debug("--type parameter value : ", resourceType)

			SetupAndRun(cmd, args)
		},
	}

	inspectMcirCmd.PersistentFlags().StringVarP(&connConfigName, "cc", "", "", "connection name")
	inspectMcirCmd.PersistentFlags().StringVarP(&resourceType, "type", "", "", "resource type")

	return inspectMcirCmd
}

// NewVmResourcesInspectCmd : "cbadm util inspect-vm"
func NewVmResourcesInspectCmd() *cobra.Command {

	inspectVmCmd := &cobra.Command{
		Use:   "inspect-vm",
		Short: "This is inspect-vm command for tumblebug utility",
		Long:  "This is inspect-vm command for tumblebug utility",
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

	inspectVmCmd.PersistentFlags().StringVarP(&connConfigName, "cc", "", "", "connection name")

	return inspectVmCmd
}

// NewObjectListCmd : "cbadm util list-obj"
func NewObjectListCmd() *cobra.Command {

	listObjCmd := &cobra.Command{
		Use:   "list-obj",
		Short: "This is list-obj command for tumblebug utility",
		Long:  "This is list-obj command for tumblebug utility",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()

			logger.Debug("--key parameter value : ", objKey)

			SetupAndRun(cmd, args)
		},
	}

	listObjCmd.PersistentFlags().StringVarP(&objKey, "key", "", "", "object key")

	return listObjCmd
}

// NewObjectGetCmd : "cbadm util get-obj"
func NewObjectGetCmd() *cobra.Command {

	getObjCmd := &cobra.Command{
		Use:   "get-obj",
		Short: "This is get-obj command for tumblebug utility",
		Long:  "This is get-obj command for tumblebug utility",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()
			if objKey == "" {
				logger.Error("failed to validate --key parameter")
				return
			}
			logger.Debug("--key parameter value : ", objKey)

			SetupAndRun(cmd, args)
		},
	}

	getObjCmd.PersistentFlags().StringVarP(&objKey, "key", "", "", "object key")

	return getObjCmd
}

// NewObjectDeleteCmd : "cbadm util delete-obj"
func NewObjectDeleteCmd() *cobra.Command {

	deleteObjCmd := &cobra.Command{
		Use:   "delete-obj",
		Short: "This is delete-obj command for tumblebug utility",
		Long:  "This is delete-obj command for tumblebug utility",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()
			if objKey == "" {
				logger.Error("failed to validate --key parameter")
				return
			}
			logger.Debug("--key parameter value : ", objKey)

			SetupAndRun(cmd, args)
		},
	}

	deleteObjCmd.PersistentFlags().StringVarP(&objKey, "key", "", "", "object key")

	return deleteObjCmd
}

// NewObjectDeleteAllCmd : "cbadm util delete-all-obj"
func NewObjectDeleteAllCmd() *cobra.Command {

	deleteAllObjCmd := &cobra.Command{
		Use:   "delete-all-obj",
		Short: "This is delete-all-obj command for tumblebug utility",
		Long:  "This is delete-all-obj command for tumblebug utility",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()
			if objKey == "" {
				logger.Error("failed to validate --key parameter")
				return
			}
			logger.Debug("--key parameter value : ", objKey)

			SetupAndRun(cmd, args)
		},
	}

	deleteAllObjCmd.PersistentFlags().StringVarP(&objKey, "key", "", "", "object key")

	return deleteAllObjCmd
}
