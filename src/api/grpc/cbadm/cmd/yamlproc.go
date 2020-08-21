package cmd

import (
	"github.com/spf13/cobra"
)

// ===== [ Constants and Variables ] =====

// ===== [ Types ] =====

// ===== [ Implementations ] =====

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====

// NewYamlApplyCmd - YAML 파일에 있는 항목을 생성하는 기능을 수행하는 Cobra Command 생성
func NewYamlApplyCmd() *cobra.Command {

	yamlApplyCmd := &cobra.Command{
		Use:   "apply",
		Short: "This is a apply command for yaml",
		Long:  "This is a apply command for yaml",
		Run: func(cmd *cobra.Command, args []string) {
			SetupAndRun(cmd, args)
		},
	}

	return yamlApplyCmd
}

// NewYamlGetCmd - YAML 파일에 있는 항목을 조회하는 기능을 수행하는 Cobra Command 생성
func NewYamlGetCmd() *cobra.Command {

	yamlGetCmd := &cobra.Command{
		Use:   "get",
		Short: "This is a get command for yaml",
		Long:  "This is a get command for yaml",
		Run: func(cmd *cobra.Command, args []string) {
			SetupAndRun(cmd, args)
		},
	}

	return yamlGetCmd
}

// NewYamlListCmd - YAML 파일에 있는 항목의 목록 조회하는 기능을 수행하는 Cobra Command 생성
func NewYamlListCmd() *cobra.Command {

	yamlListCmd := &cobra.Command{
		Use:   "list",
		Short: "This is a list command for yaml",
		Long:  "This is a list command for yaml",
		Run: func(cmd *cobra.Command, args []string) {
			SetupAndRun(cmd, args)
		},
	}

	return yamlListCmd
}

// NewYamlRemoveCmd - YAML 파일에 있는 항목을 삭제하는 기능을 수행하는 Cobra Command 생성
func NewYamlRemoveCmd() *cobra.Command {

	yamlRemoveCmd := &cobra.Command{
		Use:   "remove",
		Short: "This is a remove command for yaml",
		Long:  "This is a remove command for yaml",
		Run: func(cmd *cobra.Command, args []string) {
			SetupAndRun(cmd, args)
		},
	}

	return yamlRemoveCmd
}
