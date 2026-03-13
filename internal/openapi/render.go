package openapi

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

func RenderOperationDoc(op *Operation, format string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "markdown":
		return renderMarkdown(op), nil
	case "openapi":
		return renderOpenAPI(op)
	case "swagger":
		return renderSwagger(op)
	default:
		return "", fmt.Errorf("unsupported doc format: %s", format)
	}
}

func renderMarkdown(op *Operation) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s %s\n\n", strings.ToUpper(op.Method), op.Path)
	if op.Summary != "" {
		fmt.Fprintf(&b, "## Summary\n%s\n\n", op.Summary)
	}
	if op.Description != "" {
		fmt.Fprintf(&b, "## Description\n%s\n\n", op.Description)
	}
	fmt.Fprintf(&b, "## Metadata\n- Project: %s\n- Tags: %s\n- OperationID: %s\n\n",
		op.Project, strings.Join(op.Categories(), ", "), blankAsNA(op.OperationID))
	renderParametersSection(&b, "Path Parameters", op.PathParams)
	renderParametersSection(&b, "Query Parameters", op.QueryParams)
	renderParametersSection(&b, "Header Parameters", op.HeaderParams)
	if op.RequestBody != nil {
		fmt.Fprintf(&b, "## Request Body\n- Required: %t\n- Description: %s\n\n```yaml\n%s```\n\n",
			op.RequestBody.Required, blankAsNA(op.RequestBody.Description), mustYAML(op.RequestBody.Content))
	}
	if len(op.Responses) > 0 {
		b.WriteString("## Responses\n")
		for _, response := range op.Responses {
			fmt.Fprintf(&b, "### %s\n- Description: %s\n\n", response.Status, blankAsNA(response.Description))
			if len(response.Content) > 0 {
				fmt.Fprintf(&b, "```yaml\n%s```\n\n", mustYAML(response.Content))
			}
		}
	}
	return strings.TrimSpace(b.String()) + "\n"
}

func renderParametersSection(b *strings.Builder, title string, params []Parameter) {
	if len(params) == 0 {
		return
	}
	fmt.Fprintf(b, "## %s\n", title)
	for _, param := range params {
		fmt.Fprintf(b, "- `%s` (%s, required=%t): %s", param.Name, blankAsNA(param.Type), param.Required, blankAsNA(param.Description))
		if len(param.Enum) > 0 {
			fmt.Fprintf(b, " enum=%s", strings.Join(param.Enum, ","))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
}

func renderOpenAPI(op *Operation) (string, error) {
	payload := map[string]any{
		"openapi": "3.0.0",
		"path":    op.Path,
		"method":  strings.ToLower(op.Method),
		"doc": map[string]any{
			"tags":        op.Tags,
			"summary":     op.Summary,
			"description": op.Description,
			"operationId": op.OperationID,
			"parameters":  combineParameters(op),
			"requestBody": op.RequestBody,
			"responses":   op.Responses,
			"security":    op.Security,
		},
	}
	out, err := yaml.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("render openapi doc: %w", err)
	}
	return string(out), nil
}

func renderSwagger(op *Operation) (string, error) {
	if op.SourceType == "swagger" && op.SourceDoc != nil {
		out, err := yaml.Marshal(op.SourceDoc)
		if err != nil {
			return "", fmt.Errorf("render swagger doc: %w", err)
		}
		return string(out), nil
	}
	payload := map[string]any{
		"message": "source spec is OpenAPI 3.x; falling back to normalized operation output",
		"doc": map[string]any{
			"path":        op.Path,
			"method":      strings.ToLower(op.Method),
			"summary":     op.Summary,
			"description": op.Description,
			"parameters":  combineParameters(op),
			"requestBody": op.RequestBody,
			"responses":   op.Responses,
		},
	}
	out, err := yaml.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("render swagger fallback doc: %w", err)
	}
	return string(out), nil
}

func combineParameters(op *Operation) []Parameter {
	combined := make([]Parameter, 0, len(op.PathParams)+len(op.QueryParams)+len(op.HeaderParams))
	combined = append(combined, op.PathParams...)
	combined = append(combined, op.QueryParams...)
	combined = append(combined, op.HeaderParams...)
	return combined
}

func mustYAML(v any) string {
	if v == nil {
		return "{}\n"
	}
	out, err := yaml.Marshal(v)
	if err != nil {
		return fmt.Sprintf("error: %v\n", err)
	}
	return string(out)
}

func blankAsNA(value string) string {
	if strings.TrimSpace(value) == "" {
		return "N/A"
	}
	return value
}
