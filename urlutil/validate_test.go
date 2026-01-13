package urlutil

import (
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
		errMsg  string
	}{
		// Valid HTTP URLs
		{
			name:    "valid http localhost",
			url:     "http://localhost:3000",
			wantErr: false,
		},
		{
			name:    "valid http with domain",
			url:     "http://example.com",
			wantErr: false,
		},
		{
			name:    "valid http with path",
			url:     "http://example.com/api/v1",
			wantErr: false,
		},
		{
			name:    "valid http with query",
			url:     "http://example.com?key=value",
			wantErr: false,
		},
		{
			name:    "valid http with fragment",
			url:     "http://example.com#section",
			wantErr: false,
		},
		{
			name:    "valid http with port",
			url:     "http://example.com:8080",
			wantErr: false,
		},
		{
			name:    "valid http localhost IP",
			url:     "http://127.0.0.1:3000",
			wantErr: false,
		},
		
		// Valid HTTPS URLs
		{
			name:    "valid https",
			url:     "https://example.com",
			wantErr: false,
		},
		{
			name:    "valid https with subdomain",
			url:     "https://api.example.com",
			wantErr: false,
		},
		{
			name:    "valid https with path and query",
			url:     "https://example.com/path?key=value&foo=bar",
			wantErr: false,
		},
		{
			name:    "valid https with port",
			url:     "https://example.com:443",
			wantErr: false,
		},
		
		// URLs with whitespace (should be trimmed)
		{
			name:    "url with leading whitespace",
			url:     "  http://example.com",
			wantErr: false,
		},
		{
			name:    "url with trailing whitespace",
			url:     "http://example.com  ",
			wantErr: false,
		},
		{
			name:    "url with surrounding whitespace",
			url:     "  http://example.com  ",
			wantErr: false,
		},
		
		// Empty/whitespace URLs
		{
			name:    "empty url",
			url:     "",
			wantErr: true,
			errMsg:  "url cannot be empty",
		},
		{
			name:    "whitespace only url",
			url:     "   ",
			wantErr: true,
			errMsg:  "url cannot be empty",
		},
		
		// Invalid protocols
		{
			name:    "ftp protocol",
			url:     "ftp://example.com",
			wantErr: true,
			errMsg:  "url must use http:// or https://, got: ftp",
		},
		{
			name:    "file protocol",
			url:     "file:///etc/passwd",
			wantErr: true,
			errMsg:  "url must use http:// or https://, got: file",
		},
		{
			name:    "javascript protocol",
			url:     "javascript:alert(1)",
			wantErr: true,
			errMsg:  "url must use http:// or https://, got: javascript",
		},
		{
			name:    "data protocol",
			url:     "data:text/html,<script>alert(1)</script>",
			wantErr: true,
			errMsg:  "url must use http:// or https://, got: data",
		},
		{
			name:    "missing protocol",
			url:     "example.com",
			wantErr: true,
			errMsg:  "url must use http:// or https://",
		},
		
		// Missing host
		{
			name:    "http with no host",
			url:     "http://",
			wantErr: true,
			errMsg:  "url missing host/domain",
		},
		{
			name:    "https with no host",
			url:     "https://",
			wantErr: true,
			errMsg:  "url missing host/domain",
		},
		
		// Malformed URLs
		{
			name:    "not a url",
			url:     "not-a-url",
			wantErr: true,
			errMsg:  "url must use http:// or https://",
		},
		{
			name:    "malformed with spaces in host",
			url:     "http://example .com",
			wantErr: true,
			errMsg:  "invalid URL format",
		},
		
		// Length limits
		{
			name:    "url at max length",
			url:     "http://example.com/" + strings.Repeat("a", MaxURLLength-20),
			wantErr: false,
		},
		{
			name:    "url exceeds max length",
			url:     "http://example.com/" + strings.Repeat("a", MaxURLLength),
			wantErr: true,
			errMsg:  "url exceeds maximum length",
		},
		
		// Edge cases
		{
			name:    "url with unicode domain",
			url:     "http://例え.jp",
			wantErr: false,
		},
		{
			name:    "url with encoded characters",
			url:     "http://example.com/path%20with%20spaces",
			wantErr: false,
		},
		{
			name:    "url with user info",
			url:     "http://user:pass@example.com",
			wantErr: false,
		},
		{
			name:    "url with ipv6",
			url:     "http://[2001:db8::1]:8080",
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.url)
			
			if tt.wantErr {
				if err == nil {
					t.Errorf("Validate() expected error but got nil")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error = %v, want error containing %q", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestValidateHTTPSOnly(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
		errMsg  string
	}{
		// Valid HTTPS URLs
		{
			name:    "valid https",
			url:     "https://example.com",
			wantErr: false,
		},
		{
			name:    "valid https with path",
			url:     "https://api.example.com/v1",
			wantErr: false,
		},
		
		// Localhost HTTP (allowed)
		{
			name:    "http localhost",
			url:     "http://localhost:3000",
			wantErr: false,
		},
		{
			name:    "http 127.0.0.1",
			url:     "http://127.0.0.1:8080",
			wantErr: false,
		},
		{
			name:    "http ::1 (ipv6 localhost)",
			url:     "http://[::1]:3000",
			wantErr: false,
		},
		
		// Non-localhost HTTP (rejected)
		{
			name:    "http remote host",
			url:     "http://example.com",
			wantErr: true,
			errMsg:  "url must use https://",
		},
		{
			name:    "http remote IP",
			url:     "http://192.168.1.1",
			wantErr: true,
			errMsg:  "url must use https://",
		},
		
		// Invalid URLs (should fail basic validation)
		{
			name:    "empty url",
			url:     "",
			wantErr: true,
			errMsg:  "url cannot be empty",
		},
		{
			name:    "ftp protocol",
			url:     "ftp://example.com",
			wantErr: true,
			errMsg:  "url must use http:// or https://, got: ftp",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateHTTPSOnly(tt.url)
			
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateHTTPSOnly() expected error but got nil")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateHTTPSOnly() error = %v, want error containing %q", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateHTTPSOnly() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		wantErr    bool
		wantScheme string
		wantHost   string
		wantPath   string
	}{
		{
			name:       "valid http url",
			url:        "http://example.com/path",
			wantErr:    false,
			wantScheme: "http",
			wantHost:   "example.com",
			wantPath:   "/path",
		},
		{
			name:       "valid https url with port",
			url:        "https://example.com:443/api",
			wantErr:    false,
			wantScheme: "https",
			wantHost:   "example.com:443",
			wantPath:   "/api",
		},
		{
			name:       "url with whitespace",
			url:        "  http://example.com  ",
			wantErr:    false,
			wantScheme: "http",
			wantHost:   "example.com",
		},
		{
			name:    "invalid url",
			url:     "not-a-url",
			wantErr: true,
		},
		{
			name:    "empty url",
			url:     "",
			wantErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := Parse(tt.url)
			
			if tt.wantErr {
				if err == nil {
					t.Errorf("Parse() expected error but got nil")
				}
				return
			}
			
			if err != nil {
				t.Errorf("Parse() unexpected error = %v", err)
				return
			}
			
			if parsed.Scheme != tt.wantScheme {
				t.Errorf("Parse() scheme = %v, want %v", parsed.Scheme, tt.wantScheme)
			}
			if parsed.Host != tt.wantHost {
				t.Errorf("Parse() host = %v, want %v", parsed.Host, tt.wantHost)
			}
			if tt.wantPath != "" && parsed.Path != tt.wantPath {
				t.Errorf("Parse() path = %v, want %v", parsed.Path, tt.wantPath)
			}
		})
	}
}

func TestNormalizeScheme(t *testing.T) {
	tests := []struct {
		name          string
		url           string
		defaultScheme string
		want          string
	}{
		{
			name:          "url with http scheme",
			url:           "http://example.com",
			defaultScheme: "https",
			want:          "http://example.com",
		},
		{
			name:          "url with https scheme",
			url:           "https://example.com",
			defaultScheme: "http",
			want:          "https://example.com",
		},
		{
			name:          "url without scheme",
			url:           "example.com",
			defaultScheme: "https",
			want:          "https://example.com",
		},
		{
			name:          "url without scheme default http",
			url:           "example.com",
			defaultScheme: "http",
			want:          "http://example.com",
		},
		{
			name:          "url with ftp scheme",
			url:           "ftp://example.com",
			defaultScheme: "https",
			want:          "https://ftp://example.com",
		},
		{
			name:          "url with whitespace",
			url:           "  example.com  ",
			defaultScheme: "https",
			want:          "https://example.com",
		},
		{
			name:          "url with path no scheme",
			url:           "example.com/path",
			defaultScheme: "https",
			want:          "https://example.com/path",
		},
		{
			name:          "url with port no scheme",
			url:           "example.com:8080",
			defaultScheme: "http",
			want:          "http://example.com:8080",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeScheme(tt.url, tt.defaultScheme)
			if got != tt.want {
				t.Errorf("NormalizeScheme() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsLocalhost(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		want     bool
	}{
		{
			name:     "localhost",
			hostname: "localhost",
			want:     true,
		},
		{
			name:     "LOCALHOST uppercase",
			hostname: "LOCALHOST",
			want:     true,
		},
		{
			name:     "127.0.0.1",
			hostname: "127.0.0.1",
			want:     true,
		},
		{
			name:     "::1",
			hostname: "::1",
			want:     true,
		},
		{
			name:     "[::1] with brackets",
			hostname: "[::1]",
			want:     true,
		},
		{
			name:     "example.com",
			hostname: "example.com",
			want:     false,
		},
		{
			name:     "192.168.1.1",
			hostname: "192.168.1.1",
			want:     false,
		},
		{
			name:     "empty",
			hostname: "",
			want:     false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isLocalhost(tt.hostname)
			if got != tt.want {
				t.Errorf("isLocalhost() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateDomain(t *testing.T) {
	tests := []struct {
		name    string
		domain  string
		wantErr bool
		errMsg  string
	}{
		// Valid domains
		{
			name:    "valid simple domain",
			domain:  "example.com",
			wantErr: false,
		},
		{
			name:    "valid subdomain",
			domain:  "www.example.com",
			wantErr: false,
		},
		{
			name:    "valid api subdomain",
			domain:  "api.example.com",
			wantErr: false,
		},
		{
			name:    "valid multi-level subdomain",
			domain:  "api.staging.example.com",
			wantErr: false,
		},
		{
			name:    "valid domain with hyphen",
			domain:  "my-app.example.com",
			wantErr: false,
		},
		{
			name:    "valid azure domain",
			domain:  "myapp.azurewebsites.net",
			wantErr: false,
		},
		{
			name:    "localhost allowed",
			domain:  "localhost",
			wantErr: false,
		},
		
		// Domain with whitespace (trimmed)
		{
			name:    "domain with leading whitespace",
			domain:  "  example.com",
			wantErr: false,
		},
		{
			name:    "domain with trailing whitespace",
			domain:  "example.com  ",
			wantErr: false,
		},
		
		// Empty/whitespace
		{
			name:    "empty domain",
			domain:  "",
			wantErr: true,
			errMsg:  "domain cannot be empty",
		},
		{
			name:    "whitespace only",
			domain:  "   ",
			wantErr: true,
			errMsg:  "domain cannot be empty",
		},
		
		// Protocol included (should fail)
		{
			name:    "http protocol",
			domain:  "http://example.com",
			wantErr: true,
			errMsg:  "domain should not include protocol",
		},
		{
			name:    "https protocol",
			domain:  "https://example.com",
			wantErr: true,
			errMsg:  "domain should not include protocol",
		},
		{
			name:    "https with subdomain",
			domain:  "https://www.example.com",
			wantErr: true,
			errMsg:  "domain should not include protocol",
		},
		{
			name:    "ftp protocol",
			domain:  "ftp://example.com",
			wantErr: true,
			errMsg:  "domain should not include protocol",
		},
		
		// Port included (should fail)
		{
			name:    "domain with port",
			domain:  "example.com:8080",
			wantErr: true,
			errMsg:  "domain should not include port",
		},
		{
			name:    "subdomain with port",
			domain:  "api.example.com:443",
			wantErr: true,
			errMsg:  "domain should not include port",
		},
		
		// Invalid characters
		{
			name:    "domain with @ symbol",
			domain:  "hello@world.com",
			wantErr: true,
			errMsg:  "domain label contains invalid character",
		},
		{
			name:    "domain with underscore",
			domain:  "hello_world.com",
			wantErr: true,
			errMsg:  "domain label contains invalid character",
		},
		{
			name:    "domain with space",
			domain:  "hello world.com",
			wantErr: true,
			errMsg:  "domain label contains invalid character",
		},
		{
			name:    "domain with exclamation",
			domain:  "hello!.com",
			wantErr: true,
			errMsg:  "domain label contains invalid character",
		},
		{
			name:    "domain with slash",
			domain:  "hello/world.com",
			wantErr: true,
			errMsg:  "domain label contains invalid character",
		},
		
		// Missing dot (except localhost)
		{
			name:    "no dot in domain",
			domain:  "example",
			wantErr: true,
			errMsg:  "domain must have at least one dot",
		},
		
		// Invalid formats
		{
			name:    "starts with hyphen",
			domain:  "-example.com",
			wantErr: true,
			errMsg:  "domain label cannot start or end with hyphen",
		},
		{
			name:    "ends with hyphen",
			domain:  "example-.com",
			wantErr: true,
			errMsg:  "domain label cannot start or end with hyphen",
		},
		{
			name:    "consecutive dots",
			domain:  "example..com",
			wantErr: true,
			errMsg:  "domain has empty label",
		},
		{
			name:    "ends with dot",
			domain:  "example.com.",
			wantErr: true,
			errMsg:  "domain has empty label",
		},
		{
			name:    "starts with dot",
			domain:  ".example.com",
			wantErr: true,
			errMsg:  "domain has empty label",
		},
		
		// Length limits
		{
			name:    "label exceeds 63 chars",
			domain:  strings.Repeat("a", 64) + ".example.com",
			wantErr: true,
			errMsg:  "domain label exceeds 63 characters",
		},
		{
			name:    "domain exceeds 253 chars",
			domain:  strings.Repeat("a", 250) + ".com",
			wantErr: true,
			errMsg:  "domain exceeds maximum length",
		},
		{
			name:    "valid long domain",
			domain:  "a." + strings.Repeat("b", 60) + ".example.com",
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDomain(tt.domain)
			
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateDomain() expected error but got nil")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateDomain() error = %v, want error containing %q", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateDomain() unexpected error = %v", err)
				}
			}
		})
	}
}
