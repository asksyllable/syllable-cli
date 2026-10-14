package cmd

import (
	"math"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func TestSDKRequestMediaAndDeletePolicy(t *testing.T) {
	for _, tc := range []struct{ method, path, accept string }{
		{"GET", "/api/v1/agents/17", "application/json"},
		{"GET", "/api/v1/pronunciations/csv", "text/csv"},
		{"DELETE", "/api/v1/pronunciations/csv", "*/*"},
		{"DELETE", "/api/v1/organizations/sip_ip_ranges/17", "application/json"},
		{"GET", "/api/v1/sessions/recording/stream?token=test", "application/octet-stream"},
		{"POST", "/api/v1/voice_groups/voices/sample", "application/octet-stream"},
		{"POST", "/api/v1/language_groups/voices/sample", "application/octet-stream"},
		{"GET", "/api/v1/takeouts/get/job/file/test.zip", "application/zip"},
	} {
		if got := sdkRequestAccept(tc.method, tc.path); got != tc.accept {
			t.Errorf("%s %s Accept=%q, want %q", tc.method, tc.path, got, tc.accept)
		}
	}
	for _, tc := range []struct {
		path string
		want bool
	}{
		{"/api/v1/agents/17", true},
		{"/api/v1/bridge_phrases/17", true},
		{"/api/v1/experiments/17", false},
		{"/api/v1/users/account", false},
		{"/api/v1/organizations/sip_ip_ranges/17", false},
		{"/api/v1/directory_members/17", false}, // uses comment, not reason
	} {
		if got := sdkDeleteReasonRequired(tc.path); got != tc.want {
			t.Errorf("DELETE %s requires reason=%v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestTransportOptionsValidation(t *testing.T) {
	previousTimeout, previousRetries, previousDelay := requestTimeout, retries, retryDelay
	previousMaxDelay, previousExponent, previousElapsed := retryMaxDelay, retryExponent, retryMaxElapsedTime
	previousStatuses, previousContentType := retryStatuses, uploadContentType
	t.Cleanup(func() {
		requestTimeout, retries, retryDelay = previousTimeout, previousRetries, previousDelay
		retryMaxDelay, retryExponent, retryMaxElapsedTime = previousMaxDelay, previousExponent, previousElapsed
		retryStatuses, uploadContentType = previousStatuses, previousContentType
	})
	reset := func() {
		requestTimeout, retries, retryDelay = 0, 0, 500*time.Millisecond
		retryMaxDelay, retryExponent, retryMaxElapsedTime = time.Minute, 1.5, time.Hour
		retryStatuses, uploadContentType = []string{"429", "5xx"}, "audio/wav"
	}
	for _, tc := range []struct {
		name  string
		alter func()
	}{
		{"negative-timeout", func() { requestTimeout = -1 }},
		{"negative-count", func() { retries = -1 }},
		{"negative-delay", func() { retryDelay = -1 }},
		{"negative-max-delay", func() { retryMaxDelay = -1 }},
		{"negative-elapsed", func() { retryMaxElapsedTime = -1 }},
		{"negative-exponent", func() { retryExponent = -1 }},
		{"nan-exponent", func() { retryExponent = math.NaN() }},
		{"infinite-exponent", func() { retryExponent = math.Inf(1) }},
		{"bad-status", func() { retryStatuses = []string{"oops"} }},
		{"bad-status-range", func() { retryStatuses = []string{"9xx"} }},
		{"bad-content-type", func() { uploadContentType = "bad MIME" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reset()
			tc.alter()
			if err := validateTransportOptions(); err == nil {
				t.Fatal("invalid transport options were accepted")
			}
		})
	}
	reset()
	if err := validateTransportOptions(); err != nil {
		t.Fatalf("valid options rejected: %v", err)
	}
}

func TestInitClientDistinguishesUnsetAndZeroTimeout(t *testing.T) {
	t.Setenv("SYLLABLE_API_KEY", "unit-test")
	previousClient, previousTimeout := apiClient, requestTimeout
	previousEnv, previousDefaultEnv := viper.GetString("env"), viper.GetString("default_env")
	t.Cleanup(func() {
		apiClient, requestTimeout = previousClient, previousTimeout
		viper.Set("env", previousEnv)
		viper.Set("default_env", previousDefaultEnv)
	})
	viper.Set("env", "prod")
	viper.Set("default_env", "")
	for _, explicit := range []bool{false, true} {
		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().DurationVar(&requestTimeout, "request-timeout", 0, "timeout")
		if explicit {
			if err := cmd.ParseFlags([]string{"--request-timeout=0"}); err != nil {
				t.Fatal(err)
			}
		}
		if err := initClient(cmd); err != nil {
			t.Fatal(err)
		}
		if explicit != (apiClient.RequestTimeout != nil) {
			t.Fatalf("explicit=%v request timeout=%v", explicit, apiClient.RequestTimeout)
		}
		if apiClient.RequestTimeout != nil && *apiClient.RequestTimeout != 0 {
			t.Fatalf("explicit zero was replaced: %s", *apiClient.RequestTimeout)
		}
	}
}
