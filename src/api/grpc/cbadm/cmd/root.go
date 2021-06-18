package cmd

import (
	"github.com/cloud-barista/cb-tumblebug/src/api/grpc/config"
	"github.com/spf13/cobra"
)

// ===== [ Constants and Variables ] =====

const (
	// CLIVersion : version of cbadm cli
	CLIVersion = "1.0"
)

var (
	configFile string
	inData     string
	inFile     string
	inType     string
	outType    string

	driverName     string
	credentialName string
	regionName     string
	configName     string

	nameSpaceID     string
	resourceID      string
	force           string
	sshSaveFileName string

	option string
	mcisID string
	vmID   string

	connConfigName string

	resourceType string
	cspSpecName  string
	cspImageId   string
	host         string
	action       string
	metric       string

	configId string
	objKey   string

	parser config.Parser
)

// ===== [ Types ] =====

// ===== [ Implementations ] =====

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====

// NewRootCmd : Create Root Cobra Command
func NewRootCmd() *cobra.Command {

	rootCmd := &cobra.Command{
		Use:   "cbadm",
		Short: "cbadm is a lightweight grpc cli tool",
		Long:  "This is a lightweight grpc cli tool for Cloud-Barista",
	}

	// Option flags
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "./grpc_conf.yaml", "config file")
	rootCmd.PersistentFlags().StringVarP(&inType, "input", "i", "yaml", "input format (json/yaml)")
	rootCmd.PersistentFlags().StringVarP(&outType, "output", "o", "yaml", "output format (json/yaml)")

	// Make new config parser (which uses Viper)
	parser = config.MakeParser()

	//  Add subcommands for application.
	rootCmd.AddCommand(NewVersionCmd())

	rootCmd.AddCommand(NewDriverCmd())
	rootCmd.AddCommand(NewCredentialCmd())
	rootCmd.AddCommand(NewRegionCmd())
	rootCmd.AddCommand(NewConnectInfosCmd())

	rootCmd.AddCommand(NewNameSpaceCmd())
	rootCmd.AddCommand(NewImageCmd())
	rootCmd.AddCommand(NewNetworkCmd())
	rootCmd.AddCommand(NewSecurityCmd())
	rootCmd.AddCommand(NewKeypairCmd())
	rootCmd.AddCommand(NewSpecCmd())
	rootCmd.AddCommand(NewMcisCmd())

	rootCmd.AddCommand(NewYamlApplyCmd())
	rootCmd.AddCommand(NewYamlGetCmd())
	rootCmd.AddCommand(NewYamlListCmd())
	rootCmd.AddCommand(NewYamlRemoveCmd())

	rootCmd.AddCommand(NewUtilCmd())
	rootCmd.AddCommand(NewConfigCmd())

	return rootCmd
}
