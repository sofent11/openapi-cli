package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	api "github.com/sofent/openapi-cli/internal/openapi"
)

func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "openapi",
		Short: "Inspect and call local Swagger/OpenAPI projects",
		Long:  rootHelpText(),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		return fmt.Errorf("%s", strings.TrimSpace(err.Error()))
	})

	root.AddCommand(
		newHelpCommand(),
		newListCommand(),
		newSearchCommand(),
		newDocCommand(),
		newCallCommand(),
		newConfigCommand(),
	)
	applyHelpTemplate(root)
	return root
}

func newHelpCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "help",
		Short: "Show help and storage conventions",
		Long:  "Show the root help text or help for a specific subcommand.",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				fmt.Fprint(cmd.OutOrStdout(), rootHelpText())
				return nil
			}
			target, _, err := cmd.Root().Find(args)
			if err != nil || target == nil {
				return fmt.Errorf("unknown help topic: %s", strings.Join(args, " "))
			}
			return target.Help()
		},
	}
}

func newListCommand() *cobra.Command {
	var project string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List projects or categories within a project",
		Long:  "List all projects under ~/.openapi-cli/apis, or list category/tag counts inside one project.",
		Example: strings.TrimSpace(`
openapi list
openapi list -p erp
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			if project == "" {
				projects, err := api.ListProjects()
				if err != nil {
					return err
				}
				if len(projects) == 0 {
					fmt.Fprintln(out, "No projects found.")
					return nil
				}
				for _, projectName := range projects {
					fmt.Fprintln(out, projectName)
				}
				return nil
			}
			loaded, err := api.LoadProject(project)
			if err != nil {
				return err
			}
			categories := make([]string, 0, len(loaded.CategoryInfo))
			for category := range loaded.CategoryInfo {
				categories = append(categories, category)
			}
			sort.Strings(categories)
			for _, category := range categories {
				fmt.Fprintf(out, "%s (%d)\n", category, loaded.CategoryInfo[category])
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", "Project name")
	return cmd
}

func newSearchCommand() *cobra.Command {
	var project string
	var category string
	var query string
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search interfaces by name, description, path, operationId, or tag",
		Long:  "Search one project or all projects. Matching is case-insensitive and checks summary, description, path, operationId, and tags.",
		Example: strings.TrimSpace(`
openapi search -i login
openapi search -p erp -i lookup
openapi search -p erp -c users -i /users
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			results, err := api.SearchProjects(project, category, query)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if len(results) == 0 {
				fmt.Fprintln(out, "No interfaces matched.")
				return nil
			}
			for _, op := range results {
				fmt.Fprintf(out, "%s / %s / %s %s / %s\n",
					op.Project, strings.Join(op.Categories(), ","), strings.ToUpper(op.Method), op.Path, blankFallback(op.Summary))
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", "Project name")
	cmd.Flags().StringVarP(&category, "category", "c", "", "Category/tag filter")
	cmd.Flags().StringVarP(&query, "interface", "i", "", "Query string")
	mustMarkRequired(cmd, "interface")
	return cmd
}

func newDocCommand() *cobra.Command {
	var project string
	var path string
	var method string
	var format string
	cmd := &cobra.Command{
		Use:   "doc",
		Short: "Render interface documentation",
		Long:  "Render a single interface as markdown, normalized OpenAPI output, or Swagger output when available.",
		Example: strings.TrimSpace(`
openapi doc -p erp -i /users/{id} -m GET
openapi doc -p erp -i /users -m POST -t openapi
openapi doc -p erp -i /users -m POST -t swagger
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			loaded, err := api.LoadProject(project)
			if err != nil {
				return err
			}
			op, err := api.FindOperation(loaded, path, method)
			if err != nil {
				return err
			}
			rendered, err := api.RenderOperationDoc(op, format)
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), rendered)
			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", "Project name")
	cmd.Flags().StringVarP(&path, "interface", "i", "", "Interface path, e.g. /users/{id}")
	cmd.Flags().StringVarP(&method, "method", "m", "", "HTTP method, e.g. GET")
	cmd.Flags().StringVarP(&format, "type", "t", "markdown", "Doc format: swagger|openapi|markdown")
	mustMarkRequired(cmd, "project")
	mustMarkRequired(cmd, "interface")
	return cmd
}

func newCallCommand() *cobra.Command {
	var project string
	var path string
	var method string
	var query string
	var body string
	var verbose bool
	cmd := &cobra.Command{
		Use:   "call",
		Short: "Call an interface and print the response",
		Long:  "Call one interface using the project's baseUrl and optional auth header/token from setting.json.",
		Example: strings.TrimSpace(`
openapi call -p erp -i /users/{id} -m GET -q '{"id":"1"}'
openapi call -p erp -i /users -m POST -d '{"name":"alice"}'
openapi call -p erp -i /users -m POST -d '{"name":"alice"}' -v
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			loaded, err := api.LoadProject(project)
			if err != nil {
				return err
			}
			op, err := api.FindOperation(loaded, path, method)
			if err != nil {
				return err
			}
			opts := api.CallOptions{
				QueryJSON: query,
				BodyJSON:  body,
			}
			if verbose {
				req, err := api.BuildRequest(loaded, op, opts)
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), "Request:")
				fmt.Fprintln(cmd.OutOrStdout(), api.RenderCurlCommand(req))
				fmt.Fprintln(cmd.OutOrStdout())
			}
			result, err := api.CallOperation(loaded, op, opts)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Status: %s\n", result.Status)
			if hint := api.ResponseHint(loaded, op, result); hint != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Hint: %s\n", hint)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Headers:")
			headerKeys := make([]string, 0, len(result.Headers))
			for key := range result.Headers {
				headerKeys = append(headerKeys, key)
			}
			sort.Strings(headerKeys)
			for _, key := range headerKeys {
				fmt.Fprintf(cmd.OutOrStdout(), "- %s: %s\n", key, strings.Join(result.Headers[key], ", "))
			}
			fmt.Fprintln(cmd.OutOrStdout(), "\nBody:")
			fmt.Fprintln(cmd.OutOrStdout(), api.FormatResponseBody(result.Body))
			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", "Project name")
	cmd.Flags().StringVarP(&path, "interface", "i", "", "Interface path, e.g. /users/{id}")
	cmd.Flags().StringVarP(&method, "method", "m", "", "HTTP method, e.g. GET")
	cmd.Flags().StringVarP(&query, "query", "q", "", "Query JSON object")
	cmd.Flags().StringVarP(&body, "data", "d", "", "Request body JSON object")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Print the request as a curl command before calling")
	mustMarkRequired(cmd, "project")
	mustMarkRequired(cmd, "interface")
	return cmd
}

func newConfigCommand() *cobra.Command {
	var project string
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Read or update project configuration",
		Long:  "Show project configuration, create/import a project, update single config items, or sync the spec from updateUrl.",
		Example: strings.TrimSpace(`
openapi config -p erp
openapi config get -p erp baseUrl
openapi config set -p erp updateUrl https://example.com/swagger.json
openapi config sync -p erp
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := api.ReadConfig(project)
			if err != nil {
				return err
			}
			data, err := json.MarshalIndent(cfg, "", "  ")
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		},
	}
	cmd.PersistentFlags().StringVarP(&project, "project", "p", "", "Project name")
	mustMarkPersistentRequired(cmd, "project")

	cmd.AddCommand(newConfigCreateCommand())
	cmd.AddCommand(newConfigGetCommand(&project))
	cmd.AddCommand(newConfigSetCommand(&project))
	cmd.AddCommand(newConfigSyncCommand(&project))
	return cmd
}

func newConfigGetCommand(project *string) *cobra.Command {
	return &cobra.Command{
		Use:   "get <baseUrl|authHeader|authToken|updateUrl>",
		Short: "Get a single config value",
		Long:  "Read one field from setting.json for a project.",
		Example: strings.TrimSpace(`
openapi config get -p erp baseUrl
openapi config get -p erp updateUrl
`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := api.ReadConfig(*project)
			if err != nil {
				return err
			}
			value, err := readConfigValue(cfg, args[0])
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), value)
			return nil
		},
	}
}

func newConfigSetCommand(project *string) *cobra.Command {
	return &cobra.Command{
		Use:   "set <baseUrl|authHeader|authToken|updateUrl> <value>",
		Short: "Set a project config value",
		Long:  "Update one supported field in setting.json for a project.",
		Example: strings.TrimSpace(`
openapi config set -p erp baseUrl https://api.example.com
openapi config set -p erp authToken 'Bearer xxx'
openapi config set -p erp updateUrl https://example.com/swagger.json
`),
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := api.ReadConfig(*project)
			if err != nil {
				return err
			}
			if err := writeConfigValue(&cfg, args[0], args[1]); err != nil {
				return err
			}
			configPath, err := api.WriteConfig(*project, cfg)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated %s\n", configPath)
			return nil
		},
	}
}

func newConfigCreateCommand() *cobra.Command {
	var project string
	var baseURL string
	var authHeader string
	var authToken string
	var updateURL string
	var specType string
	cmd := &cobra.Command{
		Use:   "create <local-file-or-url>",
		Short: "Create a project and import its API spec",
		Long:  "Create a project directory under ~/.openapi-cli/apis/<project>, write api.json, and optionally seed initial config values.",
		Example: strings.TrimSpace(`
openapi config create -p erp ./swagger.json
openapi config create -p erp https://example.com/swagger.json
openapi config create -p erp ./openapi.yaml --base-url https://api.example.com --auth-header Authorization --auth-token 'Bearer xxx'
`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath, specPath, err := api.CreateProject(project, api.CreateProjectOptions{
				Source:     args[0],
				BaseURL:    baseURL,
				AuthHeader: authHeader,
				AuthToken:  authToken,
				UpdateURL:  updateURL,
				SpecType:   specType,
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created project %s\nSpec: %s\nConfig: %s\n", project, specPath, configPath)
			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", "Project name")
	cmd.Flags().StringVar(&baseURL, "base-url", "", "Initial baseUrl")
	cmd.Flags().StringVar(&authHeader, "auth-header", "", "Initial authHeader")
	cmd.Flags().StringVar(&authToken, "auth-token", "", "Initial authToken")
	cmd.Flags().StringVar(&updateURL, "update-url", "", "Initial updateUrl; remote source still takes precedence")
	cmd.Flags().StringVar(&specType, "spec-type", "", "Optional specType override")
	mustMarkRequired(cmd, "project")
	return cmd
}

func newConfigSyncCommand(project *string) *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Download the latest api.json from updateUrl",
		Long:  "Fetch the latest spec from setting.json.updateUrl and overwrite the local api.json file.",
		Example: strings.TrimSpace(`
openapi config sync -p erp
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			specPath, err := api.SyncProject(*project)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated %s\n", specPath)
			return nil
		},
	}
}

func readConfigValue(cfg api.Config, key string) (string, error) {
	switch key {
	case "baseUrl":
		return cfg.BaseURL, nil
	case "authHeader":
		return cfg.AuthHeader, nil
	case "authToken":
		return cfg.AuthToken, nil
	case "updateUrl":
		return cfg.UpdateURL, nil
	default:
		return "", fmt.Errorf("unsupported config key: %s; supported keys: baseUrl, authHeader, authToken, updateUrl", key)
	}
}

func writeConfigValue(cfg *api.Config, key, value string) error {
	switch key {
	case "baseUrl":
		cfg.BaseURL = value
	case "authHeader":
		cfg.AuthHeader = value
	case "authToken":
		cfg.AuthToken = value
	case "updateUrl":
		cfg.UpdateURL = value
	default:
		return fmt.Errorf("unsupported config key: %s; supported keys: baseUrl, authHeader, authToken, updateUrl", key)
	}
	return nil
}

func mustMarkRequired(cmd *cobra.Command, name string) {
	if err := cmd.MarkFlagRequired(name); err != nil {
		fmt.Fprintln(os.Stderr, "failed to mark flag required:", err)
	}
}

func mustMarkPersistentRequired(cmd *cobra.Command, name string) {
	if err := cmd.MarkPersistentFlagRequired(name); err != nil {
		fmt.Fprintln(os.Stderr, "failed to mark persistent flag required:", err)
	}
}

func applyHelpTemplate(cmd *cobra.Command) {
	cmd.SetHelpTemplate(commandHelpTemplate())
	for _, child := range cmd.Commands() {
		applyHelpTemplate(child)
	}
}

func commandHelpTemplate() string {
	return strings.TrimSpace(`
{{- if .Long}}{{.Long}}

{{end -}}
Usage:
  {{.UseLine}}
{{- if .HasAvailableSubCommands }}

Commands:
{{- range .Commands}}
  {{rpad .Name .NamePadding }} {{.Short}}
{{- end}}
{{- end}}
{{- if .HasAvailableLocalFlags }}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}
{{- end}}
{{- if .HasAvailableInheritedFlags }}

Inherited Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}
{{- end}}
{{- if .Example }}

Examples:
{{.Example}}
{{- end}}
`) + "\n"
}

func blankFallback(value string) string {
	if strings.TrimSpace(value) == "" {
		return "(no summary)"
	}
	return value
}

func rootHelpText() string {
	return strings.TrimSpace(`
OpenAPI CLI

Read local Swagger 2.0 and OpenAPI 3.x project specs from:
  ~/.openapi-cli/apis/<project>/

Expected project files:
  api.json | api.yaml | api.yml
  setting.json

setting.json fields:
  baseUrl
  authHeader
  authToken
  updateUrl
  specType (optional override)

Examples:
  openapi help
  openapi list
  openapi list -p erp
  openapi search -p erp -c user -i login
  openapi doc -p erp -i /users/{id} -m GET -t markdown
  openapi call -p erp -i /users -m POST -q '{"page":1}' -d '{"name":"alice"}'
  openapi config create -p erp ./swagger.json
  openapi config create -p erp ./swagger.json --base-url https://api.example.com --auth-header Authorization --auth-token 'Bearer xxx'
  openapi config create -p erp https://example.com/swagger.json
  openapi config -p erp
  openapi config get -p erp baseUrl
  openapi config get -p erp updateUrl
  openapi config set -p erp authToken my-token
  openapi config set -p erp updateUrl https://example.com/swagger.json
  openapi config sync -p erp
`) + "\n"
}
