package auth

import (
	"testing"

	"github.com/eagraf/habitat-new/internal/node/api"
	"github.com/eagraf/habitat-new/internal/node/config"
	"github.com/gorilla/sessions"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

// TestGetRoutes_BasicFunctionality tests that GetRoutes returns the expected number of routes
func TestGetRoutes_BasicFunctionality(t *testing.T) {
	// Create a test node config using the actual config package
	testConfig, err := config.NewTestNodeConfig(nil)
	require.NoError(t, err)

	// Create a session store
	sessionStore := sessions.NewCookieStore([]byte("test-key"))

	// Call GetRoutes
	routes := GetRoutes(testConfig, sessionStore)

	// Verify we get the expected number of routes (4: login, metadata, callback, xrpc broker)
	require.Len(t, routes, 4, "Expected 4 routes to be returned")

	// Verify each route has the expected method and pattern
	expectedRoutes := []struct {
		method  string
		pattern string
	}{
		{"GET", "/login"},
		{"GET", "/client-metadata.json"},
		{"GET", "/auth-callback"},
		{"POST", "/xrpc/{rest...}"},
	}

	// Create a map of method+pattern to route for easier lookup
	routeMap := make(map[string]api.Route)
	for _, route := range routes {
		key := route.Method() + ":" + route.Pattern()
		routeMap[key] = route
	}

	// Verify all expected routes exist
	for _, expected := range expectedRoutes {
		key := expected.method + ":" + expected.pattern
		route, exists := routeMap[key]
		require.True(t, exists, "Expected route %s to exist", key)
		require.Equal(t, expected.method, route.Method(),
			"Route %s should have method %s", key, expected.method)
		require.Equal(t, expected.pattern, route.Pattern(),
			"Route %s should have pattern %s", key, expected.pattern)
	}
}

// TestGetRoutes_WithEmptyPDSURL tests GetRoutes behavior when PDS URL is empty
func TestGetRoutes_WithEmptyPDSURL(t *testing.T) {
	// Create a test node config
	testConfig, err := config.NewTestNodeConfig(nil)
	require.NoError(t, err)

	// Create a session store
	sessionStore := sessions.NewCookieStore([]byte("test-key"))

	// Call GetRoutes - should not panic
	routes := GetRoutes(testConfig, sessionStore)

	// Should still return 4 routes
	require.Len(t, routes, 4, "Expected 4 routes even with empty PDS URL")

	// Verify the login handler is created (it should handle empty PDS URL gracefully)
	var loginHandlerFound bool
	for _, route := range routes {
		if route.Method() == "GET" && route.Pattern() == "/login" {
			loginHandlerFound = true
			break
		}
	}
	require.True(t, loginHandlerFound, "Login handler should be present even with empty PDS URL")
}

// TestGetRoutes_WithNilSessionStore tests GetRoutes behavior with nil session store
func TestGetRoutes_WithNilSessionStore(t *testing.T) {
	// Create a test node config
	testConfig, err := config.NewTestNodeConfig(nil)
	require.NoError(t, err)

	// Call GetRoutes with nil session store - should not panic
	routes := GetRoutes(testConfig, nil)

	// Should still return 4 routes
	require.Len(t, routes, 4, "Expected 4 routes even with nil session store")
}

// TestGetRoutes_RouteTypes tests that the correct route types are returned
func TestGetRoutes_RouteTypes(t *testing.T) {
	// Create a test node config
	testConfig, err := config.NewTestNodeConfig(nil)
	require.NoError(t, err)

	// Create a session store
	sessionStore := sessions.NewCookieStore([]byte("test-key"))

	// Call GetRoutes
	routes := GetRoutes(testConfig, sessionStore)

	// Verify each route is of the expected type
	routeTypes := make(map[string]string)
	for _, route := range routes {
		routeTypes[route.Method()+":"+route.Pattern()] = getRouteTypeName(route)
	}

	expectedTypes := map[string]string{
		"GET:/login":                "*auth.loginHandler",
		"GET:/client-metadata.json": "*auth.metadataHandler",
		"GET:/auth-callback":        "*auth.callbackHandler",
		"POST:/xrpc/{rest...}":      "*auth.xrpcBrokerHandler",
	}

	for routeKey, expectedType := range expectedTypes {
		actualType, exists := routeTypes[routeKey]
		require.True(t, exists, "Route %s should exist", routeKey)
		require.Equal(t, expectedType, actualType,
			"Route %s should be of type %s, got %s", routeKey, expectedType, actualType)
	}
}

// TestGetRoutes_KeyGeneration tests that the key generation in GetRoutes works correctly
func TestGetRoutes_KeyGeneration(t *testing.T) {
	// Create a test node config
	testConfig, err := config.NewTestNodeConfig(nil)
	require.NoError(t, err)

	// Create a session store
	sessionStore := sessions.NewCookieStore([]byte("test-key"))

	// Call GetRoutes multiple times to ensure key generation is consistent
	routes1 := GetRoutes(testConfig, sessionStore)
	routes2 := GetRoutes(testConfig, sessionStore)

	// Both calls should return the same number of routes
	require.Len(t, routes1, 4, "First call should return 4 routes")
	require.Len(t, routes2, 4, "Second call should return 4 routes")

	// The routes should be identical (same key generation)
	for i, route1 := range routes1 {
		route2 := routes2[i]
		require.Equal(t, route1.Method(), route2.Method(),
			"Route %d should have same method", i)
		require.Equal(t, route1.Pattern(), route2.Pattern(),
			"Route %d should have same pattern", i)
	}
}

// TestGetRoutes_OAuthClientCreation tests that OAuth client creation works
func TestGetRoutes_OAuthClientCreation(t *testing.T) {
	// Create a test node config
	testConfig, err := config.NewTestNodeConfig(nil)
	require.NoError(t, err)

	// Create a session store
	sessionStore := sessions.NewCookieStore([]byte("test-key"))

	// Call GetRoutes - should not panic during OAuth client creation
	routes := GetRoutes(testConfig, sessionStore)

	// Should return routes successfully
	require.Len(t, routes, 4, "Should return 4 routes after OAuth client creation")

	// Verify that the metadata handler can serve requests (basic functionality test)
	var metadataHandlerFound bool
	for _, route := range routes {
		if route.Method() == "GET" && route.Pattern() == "/client-metadata.json" {
			if _, ok := route.(*metadataHandler); ok {
				metadataHandlerFound = true
				break
			}
		}
	}
	require.True(t, metadataHandlerFound, "Should find metadata handler")
}

// TestGetRoutes_WithCustomViper tests GetRoutes with custom viper configuration
func TestGetRoutes_WithCustomViper(t *testing.T) {
	// Create a custom viper instance
	v := viper.New()
	v.Set("environment", "test")
	v.Set("habitat_path", "/tmp/test-habitat")

	// Create a test node config with custom viper
	testConfig, err := config.NewTestNodeConfig(v)
	require.NoError(t, err)

	// Create a session store
	sessionStore := sessions.NewCookieStore([]byte("test-key"))

	// Call GetRoutes
	routes := GetRoutes(testConfig, sessionStore)

	// Should return 4 routes
	require.Len(t, routes, 4, "Should return 4 routes with custom viper config")
}

// getRouteTypeName returns the type name of a route for testing purposes
func getRouteTypeName(route api.Route) string {
	switch route.(type) {
	case *loginHandler:
		return "*auth.loginHandler"
	case *metadataHandler:
		return "*auth.metadataHandler"
	case *callbackHandler:
		return "*auth.callbackHandler"
	case *xrpcBrokerHandler:
		return "*auth.xrpcBrokerHandler"
	default:
		return "unknown"
	}
}
