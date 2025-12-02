package template

import (
	"html/template"
	"time"

	"blueprint-go/internal/system/i18n"
)

// DefaultFuncs returns the default template functions
func DefaultFuncs() template.FuncMap {
	return template.FuncMap{
		// Translation function - will be overridden per-request
		"T": func(text string) string {
			return text
		},

		// Date formatting
		"formatDate": func(t time.Time, format string) string {
			switch format {
			case "short":
				return t.Format("01/02/2006")
			case "medium":
				return t.Format("Jan 2, 2006")
			case "long":
				return t.Format("January 2, 2006 3:04 PM")
			case "iso":
				return t.Format("2006-01-02")
			default:
				return t.Format(format)
			}
		},

		// Safe HTML (use with caution)
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s)
		},

		// Safe URL
		"safeURL": func(s string) template.URL {
			return template.URL(s)
		},

		// Check if value is in slice
		"contains": func(slice []string, val string) bool {
			for _, s := range slice {
				if s == val {
					return true
				}
			}
			return false
		},

		// Ternary operator equivalent
		"ifThen": func(condition bool, trueVal, falseVal interface{}) interface{} {
			if condition {
				return trueVal
			}
			return falseVal
		},

		// Default value if empty
		"default": func(val, defaultVal interface{}) interface{} {
			if val == nil || val == "" {
				return defaultVal
			}
			return val
		},

		// Add integers
		"add": func(a, b int) int {
			return a + b
		},

		// Subtract integers
		"sub": func(a, b int) int {
			return a - b
		},

		// Iterate over a range
		"seq": func(start, end int) []int {
			var result []int
			for i := start; i <= end; i++ {
				result = append(result, i)
			}
			return result
		},
	}
}

// RequestFuncs returns template functions customized for a specific request
// lang: current language code
// module: current module name
func RequestFuncs(lang, module string) template.FuncMap {
	return template.FuncMap{
		"T": func(text string) string {
			return i18n.Translate(lang, module, text)
		},
	}
}
