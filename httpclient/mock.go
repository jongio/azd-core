package httpclient

import "context"

// MockTokenProvider is a mock implementation of TokenProvider for testing.
type MockTokenProvider struct {
	Token string
	Error error
}

// GetToken returns the configured token or error.
func (m *MockTokenProvider) GetToken(ctx context.Context, scope string) (string, error) {
	if m.Error != nil {
		return "", m.Error
	}
	return m.Token, nil
}
