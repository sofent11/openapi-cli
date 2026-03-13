package openapi

import "strings"

const (
	DefaultConfigDirName = ".openapi-cli"
	DefaultCategory      = "default"
)

type Config struct {
	BaseURL    string `json:"baseUrl"`
	AuthHeader string `json:"authHeader"`
	AuthToken  string `json:"authToken"`
	UpdateURL  string `json:"updateUrl,omitempty"`
	SpecType   string `json:"specType,omitempty"`
}

type Project struct {
	Name         string
	Dir          string
	ConfigPath   string
	SpecPath     string
	Config       Config
	SpecType     string
	SpecVersion  string
	Operations   []Operation
	CategoryInfo map[string]int
	RawSpec      map[string]any
}

type Parameter struct {
	Name        string   `json:"name" yaml:"name"`
	In          string   `json:"in" yaml:"in"`
	Required    bool     `json:"required,omitempty" yaml:"required,omitempty"`
	Type        string   `json:"type,omitempty" yaml:"type,omitempty"`
	Description string   `json:"description,omitempty" yaml:"description,omitempty"`
	Enum        []string `json:"enum,omitempty" yaml:"enum,omitempty"`
}

type RequestBody struct {
	Required    bool           `json:"required,omitempty" yaml:"required,omitempty"`
	Description string         `json:"description,omitempty" yaml:"description,omitempty"`
	Content     map[string]any `json:"content,omitempty" yaml:"content,omitempty"`
}

type Response struct {
	Status      string         `json:"status" yaml:"status"`
	Description string         `json:"description,omitempty" yaml:"description,omitempty"`
	Content     map[string]any `json:"content,omitempty" yaml:"content,omitempty"`
}

type Operation struct {
	Project      string
	Method       string
	Path         string
	Tags         []string
	Summary      string
	Description  string
	OperationID  string
	QueryParams  []Parameter
	PathParams   []Parameter
	HeaderParams []Parameter
	RequestBody  *RequestBody
	Responses    []Response
	Security     any
	SourceType   string
	SourceDoc    map[string]any
}

func (o Operation) Categories() []string {
	if len(o.Tags) == 0 {
		return []string{DefaultCategory}
	}
	return o.Tags
}

func (o Operation) MatchesCategory(category string) bool {
	if category == "" {
		return true
	}
	for _, item := range o.Categories() {
		if strings.EqualFold(item, category) {
			return true
		}
	}
	return false
}
