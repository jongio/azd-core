// Package httpclient provides an HTTP client with authentication, retry with
// exponential backoff, response pagination, and configurable redirect handling.
//
// The client is built for a caller that drives every knob from the outside,
// which is why it is not a wrapper over azdext.ResilientClient: that type fixes
// its options at construction and cannot carry per-request headers, a TLS opt
// out, a redirect policy, a response size limit, or an explicit auth scope.
//
// Pagination is likewise its own implementation rather than azdext.Pager,
// because Pager reads only nextLink. This package also follows Microsoft
// Graph's @odata.nextLink and the RFC 5988 Link header, and it merges the
// sibling fields of the first page into the combined result.
//
// Requests are retried on network errors and on 408, 429, 500, 502, 503, and
// 504. A Retry-After header, in either its second or millisecond form,
// overrides the computed backoff, and the backoff itself is jittered so that
// clients throttled by one service do not retry in lockstep.
//
// Redirect targets are chosen by the server rather than the caller, so they
// are validated with azdext.SSRFSafeRedirect on top of the caller's own
// follow and hop-count policy.
package httpclient
