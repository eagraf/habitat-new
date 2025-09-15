package test_helpers

import (
	"context"
	"net/http"
)

// TestAuthMiddleWare is used by handlers to set up the context for the request
type TestAuthMiddleware struct {
	UserID string
}

func (tmw *TestAuthMiddleware) Middleware(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		newCtx := context.WithValue(ctx, "user-id", tmw.UserID)

		// Pass down the request to the next middleware (or final handler)
		next.ServeHTTP(w, r.WithContext(newCtx))
	})
}
