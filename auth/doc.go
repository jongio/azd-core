// Package auth acquires Azure OAuth tokens for REST calls and maps request
// URLs to the scopes their services expect.
//
// # Scope detection
//
// DetectScope answers "which audience does this URL need a token for". It
// returns an empty scope, and no error, for a host it does not recognize, which
// callers treat as "send this request unauthenticated".
//
// The static host to scope mapping is delegated to azdext.ScopeDetector,
// extended with custom rules for the services the SDK does not cover and one
// deliberate override: azdext maps a container registry to the Resource Manager
// scope, used to exchange for an ACR refresh token, while this package issues
// the registry data plane scope so a direct /v2/ call works.
//
// Two services stay local because a static map cannot express them. Azure Data
// Explorer issues a token for the cluster itself, so the scope is derived from
// the host. A Service Bus and an Event Hubs namespace share the
// servicebus.windows.net suffix and are told apart by the request path; azdext
// resolves that suffix to Event Hubs unconditionally, which would hand a queue
// operation a token for the wrong audience.
//
// IsAzureHost answers a different and broader question: "should we try to
// authenticate at all". A host can be recognizably Azure without this package
// knowing its scope.
//
// # Token acquisition
//
// AzureTokenProvider caches tokens per scope, applies a request timeout, and
// classifies failures into AuthPermissionError, AuthCredentialUnavailableError,
// or AuthError so callers can tell "you lack permission" from "you are not
// logged in".
//
// NewAzureTokenProvider builds a resilient credential chain that tries the azd
// CLI, the Azure CLI, environment variables, workload identity, and managed
// identity in that order. Unlike azidentity.DefaultAzureCredential it continues
// past a hard failure instead of stopping at the first one, which is what makes
// it work on Azure Arc where the managed identity probe fails outright.
//
// NewAzureTokenProviderForHost prefers azdext.TokenProvider when an azd host
// client is available. That is what makes acquisition tenant correct: the SDK
// provider reads the tenant from the deployment context, while the credential
// chain has no way to know it. It is not a replacement for this package though,
// since it has no cache, uses only the azd CLI credential, and returns raw
// azidentity errors. Wrapping it keeps the cache and the classification.
//
// NewAzureTokenProviderWithCredential wraps any azcore.TokenCredential the same
// way, which is also the seam tests use.
package auth
