// Package authn provides mTLS-based authentication for forwarding Azure credentials
// into local containers. It enables DefaultAzureCredential to work inside Docker
// containers by providing a host-side token server that the in-container azd shim
// communicates with over mTLS.
package authn
