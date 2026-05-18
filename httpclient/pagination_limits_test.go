package httpclient

import (
"context"
"encoding/json"
"errors"
"fmt"
"io"
"net/http"
"net/http/httptest"
"testing"

"github.com/stretchr/testify/assert"
"github.com/stretchr/testify/require"
)

func newPaginatedServer(t *testing.T, totalPages int, bodySize int) *httptest.Server {
t.Helper()
page := 0
return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
page++
item := map[string]any{"id": page, "data": fmt.Sprintf("%0*d", bodySize-50, 0)}
resp := map[string]any{"value": []any{item}}
if page < totalPages {
resp["nextLink"] = r.URL.String()
}
w.Header().Set("Content-Type", "application/json")
_ = json.NewEncoder(w).Encode(resp)
}))
}

func TestHandlePagination_SizeLimitExceeded(t *testing.T) {
srv := newPaginatedServer(t, 100, 200)
defer srv.Close()
opts := RequestOptions{Method: "GET", URL: srv.URL, Paginate: true, MaxPaginationSize: 300}
client := srv.Client()
ctx := context.Background()
req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL, nil)
resp, _ := client.Do(req)
body, _ := io.ReadAll(resp.Body)
_ = resp.Body.Close()
firstResp := &Response{StatusCode: resp.StatusCode, Headers: resp.Header, Body: body}
_, err := handlePagination(ctx, client, opts, firstResp)
assert.True(t, errors.Is(err, ErrPaginationSizeLimitExceeded), "expected size limit error, got: %v", err)
}

func TestHandlePagination_PageLimitExceeded(t *testing.T) {
srv := newPaginatedServer(t, 9999, 50)
defer srv.Close()
opts := RequestOptions{Method: "GET", URL: srv.URL, Paginate: true, MaxPages: 3}
client := srv.Client()
ctx := context.Background()
req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL, nil)
resp, _ := client.Do(req)
body, _ := io.ReadAll(resp.Body)
_ = resp.Body.Close()
firstResp := &Response{StatusCode: resp.StatusCode, Headers: resp.Header, Body: body}
_, err := handlePagination(ctx, client, opts, firstResp)
assert.True(t, errors.Is(err, ErrPaginationPageLimitExceeded), "expected page limit error, got: %v", err)
}

func TestHandlePagination_DefaultLimits(t *testing.T) {
assert.Equal(t, int64(1024*1024*1024), int64(DefaultMaxPaginationSize))
assert.Equal(t, 1000, DefaultMaxPages)
}

func TestHandlePagination_WithinLimits(t *testing.T) {
srv := newPaginatedServer(t, 5, 50)
defer srv.Close()
opts := RequestOptions{Method: "GET", URL: srv.URL, Paginate: true, MaxPaginationSize: 10 * 1024 * 1024, MaxPages: 100}
client := srv.Client()
ctx := context.Background()
req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL, nil)
resp, _ := client.Do(req)
body, _ := io.ReadAll(resp.Body)
_ = resp.Body.Close()
firstResp := &Response{StatusCode: resp.StatusCode, Headers: resp.Header, Body: body}
result, err := handlePagination(ctx, client, opts, firstResp)
require.NoError(t, err)
assert.NotNil(t, result)
var data map[string]any
require.NoError(t, json.Unmarshal(result, &data))
values, ok := data["value"].([]any)
require.True(t, ok)
assert.Equal(t, 5, len(values))
}