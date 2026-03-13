package openapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type CallOptions struct {
	QueryJSON string
	BodyJSON  string
	Client    *http.Client
}

type CallResult struct {
	Status     string
	StatusCode int
	Headers    http.Header
	Body       []byte
}

func CallOperation(project *Project, op *Operation, opts CallOptions) (*CallResult, error) {
	req, err := BuildRequest(project, op, opts)
	if err != nil {
		return nil, err
	}
	client := opts.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call interface: %s; check baseUrl, network reachability, and auth settings", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	return &CallResult{
		Status:     resp.Status,
		StatusCode: resp.StatusCode,
		Headers:    resp.Header.Clone(),
		Body:       body,
	}, nil
}

func BuildRequest(project *Project, op *Operation, opts CallOptions) (*http.Request, error) {
	if strings.TrimSpace(project.Config.BaseURL) == "" {
		return nil, fmt.Errorf("project baseUrl is empty; set it with `openapi config set -p %s baseUrl <url>`", project.Name)
	}
	queryInput, err := parseJSONObject(opts.QueryJSON)
	if err != nil {
		return nil, fmt.Errorf("parse query json: %s; pass a JSON object such as -q '{\"pageNum\":\"1\",\"pageSize\":\"20\"}'", err)
	}
	path, queryInput, err := resolvePathParams(op.Path, queryInput)
	if err != nil {
		return nil, err
	}
	queryValues := objectToValues(queryInput)
	baseURL := strings.TrimRight(project.Config.BaseURL, "/")
	reqURL, err := url.Parse(baseURL + path)
	if err != nil {
		return nil, fmt.Errorf("build request url: %w", err)
	}
	reqURL.RawQuery = queryValues.Encode()

	bodyReader := io.Reader(nil)
	if strings.TrimSpace(opts.BodyJSON) != "" {
		var bodyMap any
		if err := json.Unmarshal([]byte(opts.BodyJSON), &bodyMap); err != nil {
			return nil, fmt.Errorf("parse body json: %s; pass a JSON object such as -d '{\"name\":\"alice\"}'", err)
		}
		bodyReader = bytes.NewBufferString(opts.BodyJSON)
	}

	req, err := http.NewRequest(strings.ToUpper(op.Method), reqURL.String(), bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if project.Config.AuthHeader != "" && project.Config.AuthToken != "" {
		req.Header.Set(project.Config.AuthHeader, project.Config.AuthToken)
	}
	return req, nil
}

func parseJSONObject(raw string) (map[string]any, error) {
	if strings.TrimSpace(raw) == "" {
		return map[string]any{}, nil
	}
	var input map[string]any
	if err := json.Unmarshal([]byte(raw), &input); err != nil {
		return nil, err
	}
	return input, nil
}

func objectToValues(input map[string]any) url.Values {
	values := url.Values{}
	for key, value := range input {
		switch typed := value.(type) {
		case []any:
			for _, item := range typed {
				values.Add(key, fmt.Sprint(item))
			}
		default:
			values.Set(key, fmt.Sprint(value))
		}
	}
	return values
}

func resolvePathParams(path string, input map[string]any) (string, map[string]any, error) {
	resolved := path
	remaining := make(map[string]any, len(input))
	for key, value := range input {
		remaining[key] = value
	}

	paramNames := extractPathParamNames(path)
	for _, name := range paramNames {
		value, ok := remaining[name]
		if !ok {
			return "", nil, fmt.Errorf("missing path parameter %q; pass it in -q, for example: -q '{\"%s\":\"value\"}'", name, name)
		}
		text := url.PathEscape(fmt.Sprint(value))
		resolved = strings.ReplaceAll(resolved, "{"+name+"}", text)
		resolved = strings.ReplaceAll(resolved, ":"+name, text)
		delete(remaining, name)
	}
	return resolved, remaining, nil
}

func extractPathParamNames(path string) []string {
	seen := map[string]struct{}{}
	var names []string

	for i := 0; i < len(path); i++ {
		if path[i] == '{' {
			end := strings.IndexByte(path[i+1:], '}')
			if end >= 0 {
				name := path[i+1 : i+1+end]
				if name != "" {
					if _, ok := seen[name]; !ok {
						seen[name] = struct{}{}
						names = append(names, name)
					}
				}
				i = i + end + 1
			}
		}
		if path[i] == ':' {
			start := i + 1
			end := start
			for end < len(path) {
				r := rune(path[end])
				if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-') {
					break
				}
				end++
			}
			if end > start {
				name := path[start:end]
				if _, ok := seen[name]; !ok {
					seen[name] = struct{}{}
					names = append(names, name)
				}
				i = end - 1
			}
		}
	}
	return names
}

func FormatResponseBody(body []byte) string {
	var payload any
	if err := json.Unmarshal(body, &payload); err == nil {
		if out, err := json.MarshalIndent(payload, "", "  "); err == nil {
			return string(out)
		}
	}
	return string(body)
}

func ResponseHint(project *Project, op *Operation, result *CallResult) string {
	if project == nil || op == nil || result == nil || result.StatusCode < 400 {
		return ""
	}
	switch result.StatusCode {
	case http.StatusBadRequest:
		return fmt.Sprintf("请求参数不符合接口要求。请先运行 `openapi doc -p %s -i %s` 检查必填 query/path/body 参数，再核对 -q 和 -d 的 JSON 结构。", project.Name, op.Path)
	case http.StatusUnauthorized:
		return fmt.Sprintf("认证失败。请检查项目配置里的 authHeader/authToken，或先运行 `openapi config -p %s` 查看当前配置；如果需要，可先用 `openapi call -p %s -i %s -v` 查看实际发送的请求头。", project.Name, project.Name, op.Path)
	case http.StatusForbidden:
		return fmt.Sprintf("当前凭证没有权限访问这个接口。请检查 authToken 是否正确、账号是否具备权限，或联系接口提供方确认 `%s %s` 的访问权限。", strings.ToUpper(op.Method), op.Path)
	case http.StatusNotFound:
		return fmt.Sprintf("服务端返回 404。请检查 baseUrl 和接口路径是否正确，必要时运行 `openapi doc -p %s -i %s` 确认本地文档路径。", project.Name, op.Path)
	case http.StatusMethodNotAllowed:
		return fmt.Sprintf("HTTP 方法不被接受。请确认接口文档要求的方法是否正确；可以运行 `openapi doc -p %s -i %s` 检查当前路径对应的方法。", project.Name, op.Path)
	case http.StatusUnsupportedMediaType:
		return "请求体类型不被接受。当前 CLI 在传 -d 时会发送 application/json；请确认该接口是否要求 JSON 请求体。"
	case http.StatusUnprocessableEntity:
		return fmt.Sprintf("请求格式能被解析，但业务字段校验失败。请检查 -q / -d 中字段名、字段类型和必填项，并参考 `openapi doc -p %s -i %s`。", project.Name, op.Path)
	case http.StatusTooManyRequests:
		return "请求过于频繁，已被限流。请稍后重试，或降低调用频率。"
	default:
		if result.StatusCode >= 500 {
			return "服务端发生异常。请先确认请求参数和鉴权无误，再结合响应体中的 message/traceId 联系接口维护方排查。"
		}
		return "请求失败。请结合状态码、响应体和 `-v` 输出的 curl 命令继续排查。"
	}
}

func RenderCurlCommand(req *http.Request) string {
	if req == nil {
		return ""
	}
	parts := []string{"curl"}
	parts = append(parts, "-X", shellQuote(req.Method))

	headerKeys := make([]string, 0, len(req.Header))
	for key := range req.Header {
		headerKeys = append(headerKeys, key)
	}
	sort.Strings(headerKeys)
	for _, key := range headerKeys {
		for _, value := range req.Header.Values(key) {
			parts = append(parts, "-H", shellQuote(fmt.Sprintf("%s: %s", key, value)))
		}
	}

	if req.GetBody != nil {
		body, err := req.GetBody()
		if err == nil {
			data, readErr := io.ReadAll(body)
			_ = body.Close()
			if readErr == nil && len(data) > 0 {
				parts = append(parts, "--data-raw", shellQuote(string(data)))
			}
		}
	}

	parts = append(parts, shellQuote(req.URL.String()))
	return strings.Join(parts, " ")
}

func shellQuote(value string) string {
	return strconv.Quote(value)
}
