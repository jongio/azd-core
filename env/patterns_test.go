package env

import (
	"reflect"
	"strings"
	"testing"
)

func TestFilterByPrefix(t *testing.T) {
	tests := []struct {
		name    string
		envVars map[string]string
		prefix  string
		want    map[string]string
	}{
		{
			name: "single match",
			envVars: map[string]string{
				"AZURE_TENANT_ID": "xyz",
				"DATABASE_URL":    "postgres://...",
			},
			prefix: "AZURE_",
			want: map[string]string{
				"AZURE_TENANT_ID": "xyz",
			},
		},
		{
			name: "multiple matches",
			envVars: map[string]string{
				"AZURE_TENANT_ID": "xyz",
				"AZURE_CLIENT_ID": "abc",
				"DATABASE_URL":    "postgres://...",
			},
			prefix: "AZURE_",
			want: map[string]string{
				"AZURE_TENANT_ID": "xyz",
				"AZURE_CLIENT_ID": "abc",
			},
		},
		{
			name: "no matches",
			envVars: map[string]string{
				"DATABASE_URL": "postgres://...",
				"API_KEY":      "secret",
			},
			prefix: "AZURE_",
			want:   map[string]string{},
		},
		{
			name:    "empty map",
			envVars: map[string]string{},
			prefix:  "AZURE_",
			want:    map[string]string{},
		},
		{
			name:    "nil map",
			envVars: nil,
			prefix:  "AZURE_",
			want:    map[string]string{},
		},
		{
			name: "case insensitive prefix",
			envVars: map[string]string{
				"azure_tenant_id": "xyz",
				"AZURE_CLIENT_ID": "abc",
				"Azure_Region":    "westus",
			},
			prefix: "AZURE_",
			want: map[string]string{
				"azure_tenant_id": "xyz",
				"AZURE_CLIENT_ID": "abc",
				"Azure_Region":    "westus",
			},
		},
		{
			name: "empty prefix matches all",
			envVars: map[string]string{
				"KEY1": "value1",
				"KEY2": "value2",
			},
			prefix: "",
			want: map[string]string{
				"KEY1": "value1",
				"KEY2": "value2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterByPrefix(tt.envVars, tt.prefix)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FilterByPrefix() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterByPrefixSlice(t *testing.T) {
	tests := []struct {
		name     string
		envSlice []string
		prefix   string
		want     []string
	}{
		{
			name: "single match",
			envSlice: []string{
				"AZURE_TENANT_ID=xyz",
				"DATABASE_URL=postgres://...",
			},
			prefix: "AZURE_",
			want:   []string{"AZURE_TENANT_ID=xyz"},
		},
		{
			name: "multiple matches",
			envSlice: []string{
				"AZURE_TENANT_ID=xyz",
				"AZURE_CLIENT_ID=abc",
				"DATABASE_URL=postgres://...",
			},
			prefix: "AZURE_",
			want:   []string{"AZURE_TENANT_ID=xyz", "AZURE_CLIENT_ID=abc"},
		},
		{
			name: "no matches",
			envSlice: []string{
				"DATABASE_URL=postgres://...",
				"API_KEY=secret",
			},
			prefix: "AZURE_",
			want:   []string{},
		},
		{
			name:     "empty slice",
			envSlice: []string{},
			prefix:   "AZURE_",
			want:     []string{},
		},
		{
			name:     "nil slice",
			envSlice: nil,
			prefix:   "AZURE_",
			want:     []string{},
		},
		{
			name: "malformed entries skipped",
			envSlice: []string{
				"AZURE_TENANT_ID=xyz",
				"MALFORMED",
				"AZURE_CLIENT_ID=abc",
			},
			prefix: "AZURE_",
			want:   []string{"AZURE_TENANT_ID=xyz", "AZURE_CLIENT_ID=abc"},
		},
		{
			name: "case insensitive",
			envSlice: []string{
				"azure_tenant_id=xyz",
				"AZURE_CLIENT_ID=abc",
			},
			prefix: "AZURE_",
			want:   []string{"azure_tenant_id=xyz", "AZURE_CLIENT_ID=abc"},
		},
		{
			name: "values with equals signs",
			envSlice: []string{
				"AZURE_CONN=Server=localhost;User=sa",
				"DATABASE_URL=postgres://...",
			},
			prefix: "AZURE_",
			want:   []string{"AZURE_CONN=Server=localhost;User=sa"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterByPrefixSlice(tt.envSlice, tt.prefix)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FilterByPrefixSlice() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractPattern(t *testing.T) {
	tests := []struct {
		name    string
		envVars map[string]string
		opts    PatternOptions
		want    map[string]string
	}{
		{
			name: "prefix only",
			envVars: map[string]string{
				"SERVICE_API_URL": "https://api.example.com",
				"SERVICE_WEB_URL": "https://web.example.com",
				"DATABASE_URL":    "postgres://...",
			},
			opts: PatternOptions{
				Prefix: "SERVICE_",
			},
			want: map[string]string{
				"SERVICE_API_URL": "https://api.example.com",
				"SERVICE_WEB_URL": "https://web.example.com",
			},
		},
		{
			name: "prefix and suffix",
			envVars: map[string]string{
				"SERVICE_API_URL": "https://api.example.com",
				"SERVICE_WEB_URL": "https://web.example.com",
				"SERVICE_DB_HOST": "db.example.com",
				"DATABASE_URL":    "postgres://...",
			},
			opts: PatternOptions{
				Prefix: "SERVICE_",
				Suffix: "_URL",
			},
			want: map[string]string{
				"SERVICE_API_URL": "https://api.example.com",
				"SERVICE_WEB_URL": "https://web.example.com",
			},
		},
		{
			name: "trim prefix",
			envVars: map[string]string{
				"SERVICE_API_URL": "https://api.example.com",
				"SERVICE_WEB_URL": "https://web.example.com",
			},
			opts: PatternOptions{
				Prefix:     "SERVICE_",
				TrimPrefix: true,
			},
			want: map[string]string{
				"API_URL": "https://api.example.com",
				"WEB_URL": "https://web.example.com",
			},
		},
		{
			name: "trim suffix",
			envVars: map[string]string{
				"SERVICE_API_URL": "https://api.example.com",
				"SERVICE_WEB_URL": "https://web.example.com",
			},
			opts: PatternOptions{
				Suffix:     "_URL",
				TrimSuffix: true,
			},
			want: map[string]string{
				"SERVICE_API": "https://api.example.com",
				"SERVICE_WEB": "https://web.example.com",
			},
		},
		{
			name: "trim prefix and suffix",
			envVars: map[string]string{
				"SERVICE_API_URL": "https://api.example.com",
				"SERVICE_WEB_URL": "https://web.example.com",
			},
			opts: PatternOptions{
				Prefix:     "SERVICE_",
				Suffix:     "_URL",
				TrimPrefix: true,
				TrimSuffix: true,
			},
			want: map[string]string{
				"API": "https://api.example.com",
				"WEB": "https://web.example.com",
			},
		},
		{
			name: "with transform function",
			envVars: map[string]string{
				"SERVICE_MY_API_URL":  "https://api.example.com",
				"SERVICE_WEB_APP_URL": "https://web.example.com",
			},
			opts: PatternOptions{
				Prefix:     "SERVICE_",
				Suffix:     "_URL",
				TrimPrefix: true,
				TrimSuffix: true,
				Transform:  func(s string) string { return strings.ToLower(strings.ReplaceAll(s, "_", "-")) },
			},
			want: map[string]string{
				"my-api":  "https://api.example.com",
				"web-app": "https://web.example.com",
			},
		},
		{
			name: "with validator function",
			envVars: map[string]string{
				"SERVICE_API_URL": "https://api.example.com",
				"SERVICE_WEB_URL": "",
				"SERVICE_DB_URL":  "postgres://...",
			},
			opts: PatternOptions{
				Prefix:    "SERVICE_",
				Validator: func(v string) bool { return v != "" },
			},
			want: map[string]string{
				"SERVICE_API_URL": "https://api.example.com",
				"SERVICE_DB_URL":  "postgres://...",
			},
		},
		{
			name: "case insensitive matching",
			envVars: map[string]string{
				"service_api_url": "https://api.example.com",
				"SERVICE_WEB_URL": "https://web.example.com",
				"Service_Db_Url":  "postgres://...",
			},
			opts: PatternOptions{
				Prefix: "SERVICE_",
				Suffix: "_URL",
			},
			want: map[string]string{
				"service_api_url": "https://api.example.com",
				"SERVICE_WEB_URL": "https://web.example.com",
				"Service_Db_Url":  "postgres://...",
			},
		},
		{
			name: "empty prefix and suffix",
			envVars: map[string]string{
				"KEY1": "value1",
				"KEY2": "value2",
			},
			opts: PatternOptions{},
			want: map[string]string{
				"KEY1": "value1",
				"KEY2": "value2",
			},
		},
		{
			name:    "nil map",
			envVars: nil,
			opts: PatternOptions{
				Prefix: "SERVICE_",
			},
			want: map[string]string{},
		},
		{
			name: "validator rejects all",
			envVars: map[string]string{
				"SERVICE_API_URL": "https://api.example.com",
				"SERVICE_WEB_URL": "https://web.example.com",
			},
			opts: PatternOptions{
				Prefix:    "SERVICE_",
				Validator: func(v string) bool { return false },
			},
			want: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractPattern(tt.envVars, tt.opts)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ExtractPattern() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizeServiceName(t *testing.T) {
	tests := []struct {
		name       string
		envVarName string
		want       string
	}{
		{
			name:       "simple conversion",
			envVarName: "MY_API_SERVICE",
			want:       "my-api-service",
		},
		{
			name:       "single word",
			envVarName: "WEB",
			want:       "web",
		},
		{
			name:       "multiple underscores",
			envVarName: "MY_CUSTOM_API_SERVICE_V2",
			want:       "my-custom-api-service-v2",
		},
		{
			name:       "already lowercase",
			envVarName: "api_service",
			want:       "api-service",
		},
		{
			name:       "mixed case",
			envVarName: "My_Custom_Service",
			want:       "my-custom-service",
		},
		{
			name:       "no underscores",
			envVarName: "APISERVICE",
			want:       "apiservice",
		},
		{
			name:       "trailing underscore",
			envVarName: "API_SERVICE_",
			want:       "api-service-",
		},
		{
			name:       "leading underscore",
			envVarName: "_API_SERVICE",
			want:       "-api-service",
		},
		{
			name:       "empty string",
			envVarName: "",
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeServiceName(tt.envVarName)
			if got != tt.want {
				t.Errorf("NormalizeServiceName() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Edge case tests
func TestExtractPatternEdgeCases(t *testing.T) {
	t.Run("unicode in keys", func(t *testing.T) {
		envVars := map[string]string{
			"SERVICE_例え_URL": "https://example.com",
		}
		opts := PatternOptions{
			Prefix:     "SERVICE_",
			Suffix:     "_URL",
			TrimPrefix: true,
			TrimSuffix: true,
		}
		got := ExtractPattern(envVars, opts)
		want := map[string]string{
			"例え": "https://example.com",
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("ExtractPattern() with unicode = %v, want %v", got, want)
		}
	})

	t.Run("unicode in values", func(t *testing.T) {
		envVars := map[string]string{
			"SERVICE_API_URL": "https://例え.jp",
		}
		opts := PatternOptions{
			Prefix: "SERVICE_",
		}
		got := ExtractPattern(envVars, opts)
		want := map[string]string{
			"SERVICE_API_URL": "https://例え.jp",
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("ExtractPattern() with unicode value = %v, want %v", got, want)
		}
	})
}
