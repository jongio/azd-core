package auth_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/azure/azure-dev/cli/azd/pkg/azdext"
	"github.com/jongio/azd-core/auth"
)

// TestDetectScope_GainedFromSDK covers the hosts that azd-core could not
// resolve before delegating to azdext.ScopeDetector.
func TestDetectScope_GainedFromSDK(t *testing.T) {
	cases := []struct {
		name string
		url  string
		want string
	}{
		{"azure openai", "https://contoso.openai.azure.com/openai/deployments", "https://cognitiveservices.azure.com/.default"},
		{"cognitive services", "https://contoso.cognitiveservices.azure.com/vision/v3.2/analyze", "https://cognitiveservices.azure.com/.default"},
		{"ai services", "https://contoso.services.ai.azure.com/models", "https://cognitiveservices.azure.com/.default"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := auth.DetectScope(tc.url)
			if err != nil {
				t.Fatalf("DetectScope(%q) returned error: %v", tc.url, err)
			}

			if got != tc.want {
				t.Errorf("DetectScope(%q) = %q, want %q", tc.url, got, tc.want)
			}
		})
	}
}

// TestDetectScope_PreservedLocalRules covers every mapping azdext does not
// carry. A regression here means a custom rule was dropped and the affected
// service would silently start receiving an unauthenticated request.
func TestDetectScope_PreservedLocalRules(t *testing.T) {
	cases := []struct {
		name string
		url  string
		want string
	}{
		{"log analytics", "https://api.loganalytics.io/v1/workspaces/w/query", "https://api.loganalytics.io/.default"},
		{"batch", "https://acct.batch.azure.com/jobs", "https://batch.core.windows.net/.default"},
		{"mariadb", "https://s.mariadb.database.azure.com/", "https://ossrdbms-aad.database.windows.net/.default"},
		{"sql", "https://s.database.windows.net/", "https://database.windows.net/.default"},
		{"synapse", "https://w.dev.azuresynapse.net/databases", "https://dev.azuresynapse.net/.default"},
		{"data lake", "https://a.azuredatalakestore.net/webhdfs/v1/", "https://datalake.azure.net/.default"},
		{"media services", "https://m.media.azure.net/api/Assets", "https://rest.media.azure.net/.default"},
		{"table storage", "https://acct.table.core.windows.net/Tables", "https://storage.azure.com/.default"},
		{"file storage", "https://acct.file.core.windows.net/share", "https://storage.azure.com/.default"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := auth.DetectScope(tc.url)
			if err != nil {
				t.Fatalf("DetectScope(%q) returned error: %v", tc.url, err)
			}

			if got != tc.want {
				t.Errorf("DetectScope(%q) = %q, want %q", tc.url, got, tc.want)
			}
		})
	}
}

// TestDetectScope_ContainerRegistryOverridesSDK pins the one rule where
// azd-core deliberately disagrees with azdext. azdext returns the Resource
// Manager scope, which is used to exchange for an ACR refresh token; azd-core
// issues the registry data plane scope so that a direct /v2/ call works.
func TestDetectScope_ContainerRegistryOverridesSDK(t *testing.T) {
	got, err := auth.DetectScope("https://myregistry.azurecr.io/v2/_catalog")
	if err != nil {
		t.Fatalf("DetectScope returned error: %v", err)
	}

	const want = "https://containerregistry.azure.net/.default"
	if got != want {
		t.Errorf("DetectScope = %q, want %q (custom rules must take precedence over azdext defaults)", got, want)
	}
}

// TestDetectScope_KustoIsPerCluster verifies the dynamic rule that a static
// host to scope map cannot express.
func TestDetectScope_KustoIsPerCluster(t *testing.T) {
	cases := map[string]string{
		"https://help.kusto.windows.net/v1/rest/query": "https://help.kusto.windows.net/.default",
		"https://other.kusto.windows.net/v1/mgmt":      "https://other.kusto.windows.net/.default",
	}

	for in, want := range cases {
		got, err := auth.DetectScope(in)
		if err != nil {
			t.Fatalf("DetectScope(%q) returned error: %v", in, err)
		}

		if got != want {
			t.Errorf("DetectScope(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestDetectScope_ServiceBusSplitsOnPath verifies the second dynamic rule.
// azdext maps the whole servicebus.windows.net suffix to Event Hubs, which
// would hand a queue operation a token for the wrong audience.
func TestDetectScope_ServiceBusSplitsOnPath(t *testing.T) {
	cases := []struct {
		name string
		url  string
		want string
	}{
		{"queue singular", "https://ns.servicebus.windows.net/queue/messages", "https://servicebus.azure.net/.default"},
		{"queues plural", "https://ns.servicebus.windows.net/queues/orders/messages", "https://servicebus.azure.net/.default"},
		{"event hub", "https://ns.servicebus.windows.net/myhub/messages", "https://eventhubs.azure.net/.default"},
		{"namespace root", "https://ns.servicebus.windows.net/", "https://eventhubs.azure.net/.default"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := auth.DetectScope(tc.url)
			if err != nil {
				t.Fatalf("DetectScope(%q) returned error: %v", tc.url, err)
			}

			if got != tc.want {
				t.Errorf("DetectScope(%q) = %q, want %q", tc.url, got, tc.want)
			}
		})
	}
}

// TestDetectScope_UnknownHostIsNotAnError pins the contract callers rely on.
// azdext.ScopeDetector returns a *ScopeDetectorError for an unmapped host;
// azd-core translates that back to an empty scope so the request is sent
// unauthenticated instead of failing.
func TestDetectScope_UnknownHostIsNotAnError(t *testing.T) {
	for _, in := range []string{
		"https://example.com/api",
		"https://api.github.com/repos",
		"http://localhost:3000/health",
	} {
		got, err := auth.DetectScope(in)
		if err != nil {
			t.Errorf("DetectScope(%q) returned error %v, want nil", in, err)
		}

		if got != "" {
			t.Errorf("DetectScope(%q) = %q, want empty", in, got)
		}
	}
}

// TestDetectScope_EmptyHost covers a URL that parses but carries no host.
func TestDetectScope_EmptyHost(t *testing.T) {
	got, err := auth.DetectScope("/relative/path")
	if err != nil {
		t.Fatalf("DetectScope returned error: %v", err)
	}

	if got != "" {
		t.Errorf("DetectScope = %q, want empty", got)
	}
}

// TestDetectScope_HostIsCaseInsensitive verifies normalization happens before
// the lookup, in both the local rules and the delegated ones.
func TestDetectScope_HostIsCaseInsensitive(t *testing.T) {
	cases := map[string]string{
		"https://MyCluster.Kusto.Windows.Net/v1/rest/query": "https://mycluster.kusto.windows.net/.default",
		"https://MyRegistry.AzureCR.io/v2/":                 "https://containerregistry.azure.net/.default",
		"https://NS.ServiceBus.Windows.Net/queue/m":         "https://servicebus.azure.net/.default",
	}

	for in, want := range cases {
		got, err := auth.DetectScope(in)
		if err != nil {
			t.Fatalf("DetectScope(%q) returned error: %v", in, err)
		}

		if got != want {
			t.Errorf("DetectScope(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestDetectScope_InvalidURL verifies a parse failure is surfaced rather than
// swallowed into the unknown-host path.
func TestDetectScope_InvalidURL(t *testing.T) {
	_, err := auth.DetectScope("https://exa mple.com/\x7f")
	if err == nil {
		t.Fatal("DetectScope returned nil error for an unparseable URL")
	}

	if !strings.Contains(err.Error(), "failed to parse URL") {
		t.Errorf("error = %v, want it to mention the parse failure", err)
	}
}

// stubCredential is a minimal azcore.TokenCredential for exercising the
// explicit-credential constructors without any network or process access.
type stubCredential struct {
	calls int
	token string
	err   error
}

func (s *stubCredential) GetToken(_ context.Context, _ policy.TokenRequestOptions) (azcore.AccessToken, error) {
	s.calls++

	if s.err != nil {
		return azcore.AccessToken{}, s.err
	}

	return azcore.AccessToken{Token: s.token, ExpiresOn: time.Now().Add(time.Hour)}, nil
}

func TestNewAzureTokenProviderWithCredential_NilCredential(t *testing.T) {
	_, err := auth.NewAzureTokenProviderWithCredential(nil)
	if err == nil {
		t.Fatal("expected an error for a nil credential")
	}
}

// TestNewAzureTokenProviderWithCredential_CachesPerScope is the reason this
// constructor exists rather than callers using azdext.TokenProvider directly:
// the SDK provider has no cache, so a repeated request would shell out to azd
// every time.
func TestNewAzureTokenProviderWithCredential_CachesPerScope(t *testing.T) {
	cred := &stubCredential{token: "abc"}

	provider, err := auth.NewAzureTokenProviderWithCredential(cred)
	if err != nil {
		t.Fatalf("constructor returned error: %v", err)
	}

	ctx := context.Background()

	for i := 0; i < 3; i++ {
		token, err := provider.GetToken(ctx, "https://management.azure.com/.default")
		if err != nil {
			t.Fatalf("GetToken returned error: %v", err)
		}

		if token != "abc" {
			t.Errorf("GetToken = %q, want %q", token, "abc")
		}
	}

	if cred.calls != 1 {
		t.Errorf("credential called %d times, want 1 (result should be cached)", cred.calls)
	}

	if _, err := provider.GetToken(ctx, "https://graph.microsoft.com/.default"); err != nil {
		t.Fatalf("GetToken for a second scope returned error: %v", err)
	}

	if cred.calls != 2 {
		t.Errorf("credential called %d times, want 2 (a distinct scope must not hit the cache)", cred.calls)
	}
}

// TestNewAzureTokenProviderWithCredential_ClassifiesErrors is the second reason
// for the wrapper: azdext.TokenProvider returns the raw azidentity error.
func TestNewAzureTokenProviderWithCredential_ClassifiesErrors(t *testing.T) {
	cred := &stubCredential{err: errors.New("AADSTS50034: insufficient privileges to complete the operation")}

	provider, err := auth.NewAzureTokenProviderWithCredential(cred)
	if err != nil {
		t.Fatalf("constructor returned error: %v", err)
	}

	_, err = provider.GetToken(context.Background(), "https://management.azure.com/.default")
	if err == nil {
		t.Fatal("expected an error from GetToken")
	}

	if errors.Is(err, cred.err) == false && !strings.Contains(err.Error(), "insufficient privileges") {
		t.Errorf("error %v lost the underlying cause", err)
	}
}

// TestNewAzureTokenProviderForHost_NilClientFallsBack verifies the selector
// takes the credential chain path when there is no azd host to talk to.
func TestNewAzureTokenProviderForHost_NilClientFallsBack(t *testing.T) {
	provider, err := auth.NewAzureTokenProviderForHost(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("NewAzureTokenProviderForHost returned error: %v", err)
	}

	if provider == nil {
		t.Fatal("NewAzureTokenProviderForHost returned a nil provider")
	}
}

// TestAuthErrorsUnwrap verifies each typed auth error keeps the cause
// reachable through errors.Is and errors.As.
func TestAuthErrorsUnwrap(t *testing.T) {
	cause := errors.New("underlying cause")

	errs := []error{
		&auth.AuthPermissionError{Scope: "s", Err: cause},
		&auth.AuthCredentialUnavailableError{Err: cause},
		&auth.AuthError{Scope: "s", Err: cause},
	}

	for _, err := range errs {
		if !errors.Is(err, cause) {
			t.Errorf("%T does not unwrap to its cause", err)
		}

		if err.Error() == "" {
			t.Errorf("%T produced an empty message", err)
		}
	}
}

// TestGetToken_ClassifiesPermissionFromStatusCode covers the typed SDK path in
// classifyAuthError, where the status code decides rather than the message.
func TestGetToken_ClassifiesPermissionFromStatusCode(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		sdkErr := &azidentity.AuthenticationFailedError{
			RawResponse: &http.Response{StatusCode: status},
		}

		provider, err := auth.NewAzureTokenProviderWithCredential(&stubCredential{err: sdkErr})
		if err != nil {
			t.Fatalf("constructor returned error: %v", err)
		}

		_, err = provider.GetToken(context.Background(), "https://management.azure.com/.default")

		var permErr *auth.AuthPermissionError
		if !errors.As(err, &permErr) {
			t.Errorf("status %d produced %T, want *auth.AuthPermissionError", status, err)
		}
	}
}

// TestGetToken_ClassifiesCredentialUnavailable covers the second string branch.
func TestGetToken_ClassifiesCredentialUnavailable(t *testing.T) {
	provider, err := auth.NewAzureTokenProviderWithCredential(
		&stubCredential{err: errors.New("please run az login first")},
	)
	if err != nil {
		t.Fatalf("constructor returned error: %v", err)
	}

	_, err = provider.GetToken(context.Background(), "https://management.azure.com/.default")

	var unavailable *auth.AuthCredentialUnavailableError
	if !errors.As(err, &unavailable) {
		t.Errorf("got %T, want *auth.AuthCredentialUnavailableError", err)
	}
}

// TestGetToken_ClassifiesUnrecognizedErrorGenerically covers the default branch.
func TestGetToken_ClassifiesUnrecognizedErrorGenerically(t *testing.T) {
	provider, err := auth.NewAzureTokenProviderWithCredential(
		&stubCredential{err: errors.New("connection reset by peer")},
	)
	if err != nil {
		t.Fatalf("constructor returned error: %v", err)
	}

	_, err = provider.GetToken(context.Background(), "https://management.azure.com/.default")

	var generic *auth.AuthError
	if !errors.As(err, &generic) {
		t.Errorf("got %T, want *auth.AuthError", err)
	}
}

// TestGetToken_DoesNotCacheUnusableTokens verifies the guard in setCached. A
// credential that returns a blank token or no expiry must not poison the cache,
// otherwise every later call for that scope would return an unusable value.
func TestGetToken_DoesNotCacheUnusableTokens(t *testing.T) {
	cases := []struct {
		name string
		cred *unusableCredential
	}{
		{"empty token", &unusableCredential{}},
		{"zero expiry", &unusableCredential{token: "abc"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			provider, err := auth.NewAzureTokenProviderWithCredential(tc.cred)
			if err != nil {
				t.Fatalf("constructor returned error: %v", err)
			}

			ctx := context.Background()
			for i := 0; i < 2; i++ {
				if _, err := provider.GetToken(ctx, "https://management.azure.com/.default"); err != nil {
					t.Fatalf("GetToken returned error: %v", err)
				}
			}

			if tc.cred.calls != 2 {
				t.Errorf("credential called %d times, want 2 (an unusable token must not be cached)", tc.cred.calls)
			}
		})
	}
}

// TestGetToken_RefetchesExpiredToken verifies the expiry skew check in
// getCached. A token inside the skew window is treated as already expired.
func TestGetToken_RefetchesExpiredToken(t *testing.T) {
	cred := &expiringCredential{ttl: 30 * time.Second}

	provider, err := auth.NewAzureTokenProviderWithCredential(cred)
	if err != nil {
		t.Fatalf("constructor returned error: %v", err)
	}

	ctx := context.Background()
	for i := 0; i < 2; i++ {
		if _, err := provider.GetToken(ctx, "https://management.azure.com/.default"); err != nil {
			t.Fatalf("GetToken returned error: %v", err)
		}
	}

	if cred.calls != 2 {
		t.Errorf("credential called %d times, want 2 (a token expiring inside the skew window must be refetched)", cred.calls)
	}
}

// TestNewAzureTokenProviderForHost_UsesSuppliedCredential covers the host path.
// Supplying both a tenant and a credential keeps the call offline: the SDK
// provider only reaches for the deployment context when the tenant is unset.
func TestNewAzureTokenProviderForHost_UsesSuppliedCredential(t *testing.T) {
	cred := &stubCredential{token: "host-token"}

	provider, err := auth.NewAzureTokenProviderForHost(
		context.Background(),
		&azdext.AzdClient{},
		&azdext.TokenProviderOptions{TenantID: "00000000-0000-0000-0000-000000000000", Credential: cred},
	)
	if err != nil {
		t.Fatalf("NewAzureTokenProviderForHost returned error: %v", err)
	}

	token, err := provider.GetToken(context.Background(), "https://management.azure.com/.default")
	if err != nil {
		t.Fatalf("GetToken returned error: %v", err)
	}

	if token != "host-token" {
		t.Errorf("GetToken = %q, want %q", token, "host-token")
	}

	if cred.calls != 1 {
		t.Errorf("credential called %d times, want 1", cred.calls)
	}
}

func TestIsAzureHost_InvalidURL(t *testing.T) {
	if auth.IsAzureHost("https://exa mple.com/\x7f") {
		t.Error("IsAzureHost returned true for an unparseable URL")
	}
}

func TestIsAzureHost_BareHostMatch(t *testing.T) {
	if !auth.IsAzureHost("https://management.azure.com/subscriptions") {
		t.Error("IsAzureHost returned false for management.azure.com")
	}

	if auth.IsAzureHost("https://example.com/api") {
		t.Error("IsAzureHost returned true for a non-Azure host")
	}
}

// unusableCredential returns a token the cache must reject.
type unusableCredential struct {
	calls int
	token string
}

func (u *unusableCredential) GetToken(_ context.Context, _ policy.TokenRequestOptions) (azcore.AccessToken, error) {
	u.calls++

	return azcore.AccessToken{Token: u.token}, nil
}

// expiringCredential returns a token that expires inside the skew window.
type expiringCredential struct {
	calls int
	ttl   time.Duration
}

func (e *expiringCredential) GetToken(_ context.Context, _ policy.TokenRequestOptions) (azcore.AccessToken, error) {
	e.calls++

	return azcore.AccessToken{Token: "short", ExpiresOn: time.Now().Add(e.ttl)}, nil
}
