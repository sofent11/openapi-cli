package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func FormatCommandError(root *cobra.Command, err error) string {
	if err == nil {
		return ""
	}
	message := strings.TrimSpace(err.Error())

	switch {
	case strings.Contains(message, "unknown command"):
		return fmt.Sprintf(
			"命令不存在：%s\n请先运行 `%s help` 查看可用命令，或运行 `%s <command> --help` 查看子命令帮助。",
			message,
			root.CommandPath(),
			root.CommandPath(),
		)
	case strings.Contains(message, "unknown shorthand flag"),
		strings.Contains(message, "unknown flag"):
		return fmt.Sprintf(
			"参数不存在：%s\n请运行对应命令的 `--help` 查看正确参数，例如 `%s call --help`。",
			message,
			root.CommandPath(),
		)
	case strings.Contains(message, "required flag(s)"):
		return fmt.Sprintf(
			"缺少必填参数：%s\n请运行对应命令的 `--help` 查看参数说明和示例，例如 `%s config create --help`。",
			message,
			root.CommandPath(),
		)
	default:
		return "Error: " + message
	}
}
