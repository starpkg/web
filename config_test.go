package web

import (
	"os"
	"testing"
	"time"

	"github.com/starpkg/base"
)

// TestConfigurationUsage tests that module configurations are properly used
func TestConfigurationUsage(t *testing.T) {
	// Test with custom configuration values
	customModule := newModuleWithOptions(
		genConfigOption(configKeyHost, "Custom host", "0.0.0.0"),
		genConfigOption(configKeyPort, "Custom port", 9090),
		genConfigOption(configKeyReadTimeout, "Custom read timeout", 60),             // 60 seconds
		genConfigOption(configKeyWriteTimeout, "Custom write timeout", 45),           // 45 seconds
		genConfigOption(configKeyMaxBodySize, "Custom max body size", int64(64<<20)), // 64MB
		genConfigOption(configKeyEnableCORS, "Custom CORS setting", true),
		genConfigOption(configKeyDebugMode, "Custom debug mode", true),
		genConfigOption(configKeyServerHeader, "Custom server header", "TestServer/1.0"),
	)

	// Create a server to test configuration usage
	server := newServer(customModule, "0.0.0.0", 9090)

	// Verify the configuration values are applied
	if server.host != "0.0.0.0" {
		t.Errorf("Expected host '0.0.0.0', got '%s'", server.host)
	}

	if server.port != 9090 {
		t.Errorf("Expected port 9090, got %d", server.port)
	}

	if server.readTimeout != 60*time.Second {
		t.Errorf("Expected read timeout 60s, got %v", server.readTimeout)
	}

	if server.writeTimeout != 45*time.Second {
		t.Errorf("Expected write timeout 45s, got %v", server.writeTimeout)
	}

	if server.maxBodySize != int64(64<<20) {
		t.Errorf("Expected max body size 64MB (%d), got %d", int64(64<<20), server.maxBodySize)
	}

	if !server.enableCORS {
		t.Error("Expected CORS to be enabled")
	}
}

// TestEnvironmentVariableConfiguration tests that environment variables are properly used
func TestEnvironmentVariableConfiguration(t *testing.T) {
	// Set environment variables
	os.Setenv("web_host", "env-host")
	os.Setenv("web_port", "8888")
	os.Setenv("web_read_timeout", "120")
	os.Setenv("web_write_timeout", "90")
	os.Setenv("web_enable_cors", "true")
	defer func() {
		os.Unsetenv("web_host")
		os.Unsetenv("web_port")
		os.Unsetenv("web_read_timeout")
		os.Unsetenv("web_write_timeout")
		os.Unsetenv("web_enable_cors")
	}()

	// Create a module that should pick up environment variables
	module := NewModule()

	// Test that environment variables are read correctly
	host := module.ext.GetString(configKeyHost)
	port := module.ext.GetInt(configKeyPort)
	readTimeout := module.ext.GetInt(configKeyReadTimeout)
	writeTimeout := module.ext.GetInt(configKeyWriteTimeout)
	enableCORS := module.ext.GetBool(configKeyEnableCORS)

	if host != "env-host" {
		t.Errorf("Expected host from env 'env-host', got '%s'", host)
	}

	if port != 8888 {
		t.Errorf("Expected port from env 8888, got %d", port)
	}

	if readTimeout != 120 {
		t.Errorf("Expected read timeout from env 120, got %d", readTimeout)
	}

	if writeTimeout != 90 {
		t.Errorf("Expected write timeout from env 90, got %d", writeTimeout)
	}

	if !enableCORS {
		t.Error("Expected CORS to be enabled from env")
	}
}

// TestDefaultConfiguration tests that default configuration values are correct
func TestDefaultConfiguration(t *testing.T) {
	module := NewModule()

	// Test default values
	host := module.ext.GetString(configKeyHost)
	port := module.ext.GetInt(configKeyPort)
	readTimeout := module.ext.GetInt(configKeyReadTimeout)
	writeTimeout := module.ext.GetInt(configKeyWriteTimeout)
	enableCORS := module.ext.GetBool(configKeyEnableCORS)
	debugMode := module.ext.GetBool(configKeyDebugMode)

	if host != "localhost" {
		t.Errorf("Expected default host 'localhost', got '%s'", host)
	}

	if port != 8080 {
		t.Errorf("Expected default port 8080, got %d", port)
	}

	if readTimeout != 30 {
		t.Errorf("Expected default read timeout 30, got %d", readTimeout)
	}

	if writeTimeout != 30 {
		t.Errorf("Expected default write timeout 30, got %d", writeTimeout)
	}

	if enableCORS {
		t.Error("Expected default CORS to be disabled")
	}

	if debugMode {
		t.Error("Expected default debug mode to be disabled")
	}

	// Test max body size with GetConfigValue
	maxBodySize, err := base.GetConfigValue[int64](module.cfgMod, configKeyMaxBodySize)
	if err != nil {
		t.Errorf("Failed to get max body size config: %v", err)
	}

	expectedMaxBodySize := int64(32 << 20) // 32MB
	if maxBodySize != expectedMaxBodySize {
		t.Errorf("Expected default max body size %d, got %d", expectedMaxBodySize, maxBodySize)
	}
}

// TestServerTimeoutConfiguration tests that server timeouts are properly applied
func TestServerTimeoutConfiguration(t *testing.T) {
	// Test with custom timeout values
	customModule := newModuleWithOptions(
		genConfigOption(configKeyHost, "Custom host", "localhost"),
		genConfigOption(configKeyPort, "Custom port", 0),                             // Use port 0 for auto-assignment
		genConfigOption(configKeyReadTimeout, "Custom read timeout", 120),            // 2 minutes
		genConfigOption(configKeyWriteTimeout, "Custom write timeout", 180),          // 3 minutes
		genConfigOption(configKeyMaxBodySize, "Custom max body size", int64(64<<20)), // 64MB
		genConfigOption(configKeyEnableCORS, "Custom CORS setting", false),
		genConfigOption(configKeyDebugMode, "Custom debug mode", false),
		genConfigOption(configKeyServerHeader, "Custom server header", "TimeoutTestServer/1.0"),
	)

	// Create a server with port 0 to avoid conflicts
	server := newServer(customModule, "localhost", 0)

	// Start the server
	err := server.Start()
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Wait a bit for the server to be fully ready
	time.Sleep(200 * time.Millisecond)

	// Verify server is running
	if !server.IsRunning() {
		t.Fatal("Server should be running")
	}

	// Now safely check the timeout configuration values
	// These are set during server creation, not during Start()
	if server.readTimeout != 120*time.Second {
		t.Errorf("Expected read timeout 120s, got %v", server.readTimeout)
	}

	if server.writeTimeout != 180*time.Second {
		t.Errorf("Expected write timeout 180s, got %v", server.writeTimeout)
	}

	// Check that the http.Server has the correct timeout values
	server.mu.RLock()
	httpServer := server.httpServer
	server.mu.RUnlock()

	if httpServer == nil {
		t.Fatal("httpServer should not be nil after starting")
	}

	if httpServer.ReadTimeout != 120*time.Second {
		t.Errorf("Expected HTTP server read timeout 120s, got %v", httpServer.ReadTimeout)
	}

	if httpServer.WriteTimeout != 180*time.Second {
		t.Errorf("Expected HTTP server write timeout 180s, got %v", httpServer.WriteTimeout)
	}

	// Stop the server
	err = server.Stop()
	if err != nil {
		t.Errorf("Failed to stop server: %v", err)
	}
}
