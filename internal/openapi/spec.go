package openapi

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/getkin/kin-openapi/openapi3"
	"gopkg.in/yaml.v3"
)

func LoadProject(project string) (*Project, error) {
	cfg, configPath, err := ReadConfig(project)
	if err != nil {
		return nil, err
	}
	specPath, err := FindSpecPath(project)
	if err != nil {
		return nil, err
	}
	rawData, err := osReadFile(specPath)
	if err != nil {
		return nil, fmt.Errorf("read api spec: %w", err)
	}
	specType, specVersion, rawMap, doc3, err := parseSpec(rawData, cfg.SpecType)
	if err != nil {
		return nil, err
	}
	operations, categoryInfo, err := buildOperations(project, specType, rawMap, doc3)
	if err != nil {
		return nil, err
	}
	projectDir, err := ProjectDir(project)
	if err != nil {
		return nil, err
	}
	return &Project{
		Name:         project,
		Dir:          projectDir,
		ConfigPath:   configPath,
		SpecPath:     specPath,
		Config:       cfg,
		SpecType:     specType,
		SpecVersion:  specVersion,
		Operations:   operations,
		CategoryInfo: categoryInfo,
		RawSpec:      rawMap,
	}, nil
}

func parseSpec(data []byte, override string) (string, string, map[string]any, *openapi3.T, error) {
	var rawMap map[string]any
	if err := yaml.Unmarshal(data, &rawMap); err != nil {
		return "", "", nil, nil, fmt.Errorf("parse api spec: %w", err)
	}
	specType := detectSpecType(rawMap, override)
	switch specType {
	case "openapi":
		loader := openapi3.NewLoader()
		loader.IsExternalRefsAllowed = false
		doc, err := loader.LoadFromData(data)
		if err != nil {
			return "", "", nil, nil, fmt.Errorf("load openapi spec: %w", err)
		}
		_ = validateSpecBestEffort(doc)
		return "openapi", doc.OpenAPI, rawMap, doc, nil
	case "swagger":
		var doc2 openapi2.T
		jsonData, err := json.Marshal(rawMap)
		if err != nil {
			return "", "", nil, nil, fmt.Errorf("encode swagger spec: %w", err)
		}
		if err := json.Unmarshal(jsonData, &doc2); err != nil {
			return "", "", nil, nil, fmt.Errorf("parse swagger spec: %w", err)
		}
		doc3, err := openapi2conv.ToV3(&doc2)
		if err != nil {
			return "", "", nil, nil, fmt.Errorf("convert swagger spec: %w", err)
		}
		_ = validateSpecBestEffort(doc3)
		return "swagger", doc2.Swagger, rawMap, doc3, nil
	default:
		return "", "", nil, nil, fmt.Errorf("unsupported api spec format: expected Swagger 2.0 or OpenAPI 3.x")
	}
}

func validateSpecBestEffort(doc *openapi3.T) error {
	if doc == nil {
		return nil
	}
	return doc.Validate(context.Background())
}

func detectSpecType(rawMap map[string]any, override string) string {
	if value, ok := rawMap["openapi"].(string); ok && value != "" {
		return "openapi"
	}
	if value, ok := rawMap["swagger"].(string); ok && value == "2.0" {
		return "swagger"
	}
	switch strings.ToLower(strings.TrimSpace(override)) {
	case "swagger", "swagger2", "2.0":
		return "swagger"
	case "openapi", "openapi3", "3.0", "3.1":
		return "openapi"
	default:
		return ""
	}
}

func buildOperations(projectName, specType string, rawMap map[string]any, doc3 *openapi3.T) ([]Operation, map[string]int, error) {
	operations := make([]Operation, 0)
	categoryInfo := make(map[string]int)
	for path, pathItem := range doc3.Paths.Map() {
		for _, method := range []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"} {
			op := pathItem.GetOperation(strings.ToUpper(method))
			if op == nil {
				continue
			}
			operation := Operation{
				Project:     projectName,
				Method:      method,
				Path:        path,
				Tags:        copyOrDefault(op.Tags, DefaultCategory),
				Summary:     op.Summary,
				Description: op.Description,
				OperationID: op.OperationID,
				Security:    op.Security,
				SourceType:  specType,
				SourceDoc:   extractSourceDoc(rawMap, path, method, specType),
			}
			parameters := mergeParameters(pathItem.Parameters, op.Parameters)
			for _, paramRef := range parameters {
				if paramRef == nil || paramRef.Value == nil {
					continue
				}
				normalized := normalizeParameter(paramRef.Value)
				switch strings.ToLower(paramRef.Value.In) {
				case "query":
					operation.QueryParams = append(operation.QueryParams, normalized)
				case "path":
					operation.PathParams = append(operation.PathParams, normalized)
				case "header":
					operation.HeaderParams = append(operation.HeaderParams, normalized)
				}
			}
			if op.RequestBody != nil && op.RequestBody.Value != nil {
				operation.RequestBody = normalizeRequestBody(op.RequestBody.Value)
			}
			operation.Responses = normalizeResponses(op.Responses)
			for _, category := range operation.Categories() {
				categoryInfo[category]++
			}
			operations = append(operations, operation)
		}
	}
	sort.Slice(operations, func(i, j int) bool {
		if operations[i].Project != operations[j].Project {
			return operations[i].Project < operations[j].Project
		}
		if operations[i].Path != operations[j].Path {
			return operations[i].Path < operations[j].Path
		}
		return operations[i].Method < operations[j].Method
	})
	return operations, categoryInfo, nil
}

func copyOrDefault(values []string, fallback string) []string {
	if len(values) == 0 {
		return []string{fallback}
	}
	out := make([]string, len(values))
	copy(out, values)
	return out
}

func mergeParameters(base, specific openapi3.Parameters) openapi3.Parameters {
	merged := make(openapi3.Parameters, 0, len(base)+len(specific))
	seen := map[string]struct{}{}
	for _, paramRef := range specific {
		if paramRef == nil || paramRef.Value == nil {
			continue
		}
		key := strings.ToLower(paramRef.Value.In + ":" + paramRef.Value.Name)
		seen[key] = struct{}{}
	}
	for _, paramRef := range base {
		if paramRef == nil || paramRef.Value == nil {
			continue
		}
		key := strings.ToLower(paramRef.Value.In + ":" + paramRef.Value.Name)
		if _, ok := seen[key]; ok {
			continue
		}
		merged = append(merged, paramRef)
	}
	merged = append(merged, specific...)
	return merged
}

func normalizeParameter(param *openapi3.Parameter) Parameter {
	typ := ""
	enum := []string{}
	if param.Schema != nil && param.Schema.Value != nil {
		typ = strings.Join([]string(*param.Schema.Value.Type), ",")
		for _, item := range param.Schema.Value.Enum {
			enum = append(enum, fmt.Sprint(item))
		}
	}
	return Parameter{
		Name:        param.Name,
		In:          param.In,
		Required:    param.Required,
		Type:        typ,
		Description: param.Description,
		Enum:        enum,
	}
}

func normalizeRequestBody(body *openapi3.RequestBody) *RequestBody {
	content := make(map[string]any)
	for contentType, mediaType := range body.Content {
		content[contentType] = mediaType.Schema.Value
	}
	return &RequestBody{
		Required:    body.Required,
		Description: body.Description,
		Content:     content,
	}
}

func normalizeResponses(responses *openapi3.Responses) []Response {
	if responses == nil {
		return nil
	}
	keys := make([]string, 0)
	for status := range responses.Map() {
		keys = append(keys, status)
	}
	sort.Strings(keys)
	out := make([]Response, 0, len(keys))
	for _, status := range keys {
		responseRef := responses.Value(status)
		if responseRef == nil || responseRef.Value == nil {
			continue
		}
		content := make(map[string]any)
		for contentType, mediaType := range responseRef.Value.Content {
			content[contentType] = mediaType.Schema.Value
		}
		out = append(out, Response{
			Status:      status,
			Description: derefString(responseRef.Value.Description),
			Content:     content,
		})
	}
	return out
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func extractSourceDoc(rawMap map[string]any, path, method, specType string) map[string]any {
	pathsMap, ok := rawMap["paths"].(map[string]any)
	if !ok {
		return nil
	}
	pathMap, ok := pathsMap[path].(map[string]any)
	if !ok {
		return nil
	}
	methodKey := strings.ToLower(method)
	rawOp, ok := pathMap[methodKey]
	if !ok {
		return nil
	}
	rawOpMap, ok := rawOp.(map[string]any)
	if !ok {
		return nil
	}
	if specType == "swagger" {
		return map[string]any{
			"swagger": rawMap["swagger"],
			"path":    path,
			"method":  methodKey,
			"doc":     rawOpMap,
		}
	}
	return map[string]any{
		"openapi": rawMap["openapi"],
		"path":    path,
		"method":  methodKey,
		"doc":     rawOpMap,
	}
}

func FindOperation(project *Project, path, method string) (*Operation, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("interface path is required; pass -i <path>, for example: openapi doc -p %s -i /users/{id}", project.Name)
	}
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		var matches []*Operation
		for i := range project.Operations {
			op := &project.Operations[i]
			if op.Path == path {
				matches = append(matches, op)
			}
		}
		switch len(matches) {
		case 0:
			return nil, fmt.Errorf("interface not found: %s; use `openapi search -p %s -i keyword` to find the correct path", path, project.Name)
		case 1:
			return matches[0], nil
		default:
			methods := make([]string, 0, len(matches))
			for _, match := range matches {
				methods = append(methods, strings.ToUpper(match.Method))
			}
			sort.Strings(methods)
			return nil, fmt.Errorf("multiple methods found for %s; use -m to choose one of: %s", path, strings.Join(methods, ", "))
		}
	}
	for i := range project.Operations {
		op := &project.Operations[i]
		if op.Path == path && strings.EqualFold(op.Method, method) {
			return op, nil
		}
	}
	return nil, fmt.Errorf("interface not found: %s %s; check the path and method, or run `openapi search -p %s -i keyword`", method, path, project.Name)
}

func SearchProjects(projectName, category, query string) ([]Operation, error) {
	query = strings.TrimSpace(strings.ToLower(query))
	if query == "" {
		return nil, fmt.Errorf("query is required; pass -i <keyword>, for example: openapi search -i user")
	}
	var projects []string
	if projectName != "" {
		projects = []string{projectName}
	} else {
		var err error
		projects, err = ListProjects()
		if err != nil {
			return nil, err
		}
	}
	var results []Operation
	for _, name := range projects {
		project, err := LoadProject(name)
		if err != nil {
			return nil, err
		}
		for _, op := range project.Operations {
			if !op.MatchesCategory(category) {
				continue
			}
			haystack := strings.ToLower(strings.Join([]string{
				op.Summary, op.Description, op.Path, op.OperationID, strings.Join(op.Tags, " "),
			}, "\n"))
			if strings.Contains(haystack, query) {
				results = append(results, op)
			}
		}
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Project != results[j].Project {
			return results[i].Project < results[j].Project
		}
		if results[i].Path != results[j].Path {
			return results[i].Path < results[j].Path
		}
		return results[i].Method < results[j].Method
	})
	return results, nil
}

func MarshalIndentedJSON(v any) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}
