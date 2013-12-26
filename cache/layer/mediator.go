package layer

import (
	"gnd.la/app"
	"gnd.la/util/hashutil"
	"net/http"
)

// Mediator is the interface which indicates the Layer
// if and how a response should be cached.
type Mediator interface {
	// Skip indicates if the request in the given context should skip the cache.
	Skip(ctx *app.Context) bool
	// Key returns the cache key used for the given context.
	Key(ctx *app.Context) string
	// Cache returns wheter a response with the given code and headers should
	// be cached.
	Cache(ctx *app.Context, responseCode int, outgoingHeaders http.Header) bool
	// Expires returns the cache expiration time for given context, response code
	// and headers.
	Expires(ctx *app.Context, responseCode int, outgoingHeaders http.Header) int
}

// SimpleMediator implements a Mediator which caches GET and HEAD
// request with a 200 response code for a fixed time and skips
// the cache if any of the indicated cookies are present. Cache keys
// are generated by hashing the request method and its URL.
type SimpleMediator struct {
	// SkipCookies includes any cookie which should make the request
	// skip the cache Layer when the cookie is present.
	SkipCookies []string
	// Expiration indicates the cache expiration for cached requests.
	Expiration int
}

func (m *SimpleMediator) Skip(ctx *app.Context) bool {
	if m := ctx.R.Method; m != "GET" && m != "HEAD" {
		return true
	}
	c := ctx.Cookies()
	for _, v := range m.SkipCookies {
		if c.Has(v) {
			return true
		}
	}
	return false
}

func (m *SimpleMediator) Key(ctx *app.Context) string {
	return hashutil.Md5(ctx.R.Method + ctx.R.URL.String())
}

func (m *SimpleMediator) Cache(ctx *app.Context, responseCode int, outgoingHeaders http.Header) bool {
	return responseCode == http.StatusOK
}

func (m *SimpleMediator) Expires(ctx *app.Context, responseCode int, outgoingHeaders http.Header) int {
	return m.Expiration
}
