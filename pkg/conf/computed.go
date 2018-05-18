package conf

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/sourcegraph/sourcegraph/schema"
)

const defaultHTTPStrictTransportSecurity = "max-age=31536000" // 1 year

// HTTPStrictTransportSecurity returns the value of the Strict-Transport-Security HTTP header to set.
func HTTPStrictTransportSecurity() string {
	switch v := Get().HttpStrictTransportSecurity.(type) {
	case string:
		return v
	case bool:
		if !v {
			return ""
		}
		return defaultHTTPStrictTransportSecurity
	default:
		return defaultHTTPStrictTransportSecurity
	}
}

// SearchTimeoutParameterEnabled returns true if SearchTimeoutParameter experiment is enabled.
func SearchTimeoutParameterEnabled() bool {
	p := Get().ExperimentalFeatures.SearchTimeoutParameter
	// default is enabled
	return p != "disabled"
}

// JumpToDefOSSIndexEnabled returns true if JumpToDefOSSIndex experiment is enabled.
func JumpToDefOSSIndexEnabled() bool {
	p := Get().ExperimentalFeatures.JumpToDefOSSIndex
	// default is disabled
	return p == "enabled"
}

// HostSurveysLocally returns true if HostSurveysLocally experiment is enabled.
func HostSurveysLocallyEnabled() bool {
	p := Get().ExperimentalFeatures.HostSurveysLocally
	// default is disabled
	return p == "enabled"
}

// MultipleAuthProvidersEnabled reports whether the "multipleAuthProviders" experiment is enabled.
func MultipleAuthProvidersEnabled() bool { return multipleAuthProvidersEnabled(Get()) }
func multipleAuthProvidersEnabled(c *schema.SiteConfiguration) bool {
	// default is disabled
	return c.ExperimentalFeatures != nil && c.ExperimentalFeatures.MultipleAuthProviders == "enabled"
}

// AccessTokensEnabled returns whether access tokens are enabled.
func AccessTokensEnabled() bool {
	return !Get().AuthDisableAccessTokens
}

// EmailVerificationRequired returns whether users must verify an email address before they
// can perform most actions on this site.
//
// It's false for sites that do not have an email sending API key set up.
func EmailVerificationRequired() bool {
	return Get().EmailSmtp != nil
}

// CanSendEmail returns whether the site can send emails (e.g., to reset a password or
// invite a user to an org).
//
// It's false for sites that do not have an email sending API key set up.
func CanSendEmail() bool {
	return Get().EmailSmtp != nil
}

// HasGitHubDotComToken reports whether there are any personal access tokens configured for
// github.com.
func HasGitHubDotComToken() bool {
	for _, c := range Get().Github {
		u, err := url.Parse(c.Url)
		if err != nil {
			continue
		}
		hostname := strings.ToLower(u.Hostname())
		if (hostname == "github.com" || hostname == "api.github.com") && c.Token != "" {
			return true
		}
	}
	return false
}

// HasGitLabDotComToken reports whether there are any personal access tokens configured for
// github.com.
func HasGitLabDotComToken() bool {
	for _, c := range Get().Gitlab {
		u, err := url.Parse(c.Url)
		if err != nil {
			continue
		}
		hostname := strings.ToLower(u.Hostname())
		if hostname == "gitlab.com" && c.Token != "" {
			return true
		}
	}
	return false
}

// EnabledLangservers returns the langservers that are not disabled.
func EnabledLangservers() []*schema.Langservers {
	all := Get().Langservers
	results := make([]*schema.Langservers, 0, len(all))
	for _, langserver := range all {
		if langserver.Disabled {
			continue
		}
		results = append(results, langserver)
	}
	return results
}

// DeployType tells the deployment type.
func DeployType() string {
	if e := os.Getenv("DEPLOY_TYPE"); e != "" {
		return e
	}
	return "dev"
}

// IsDataCenter tells if the given deployment type is "datacenter".
func IsDataCenter(deployType string) bool {
	return deployType == "datacenter"
}

// IsServer tells if the given deployment type is "server".
func IsServer(deployType string) bool {
	return deployType == "server"
}

// IsDev tells if the given deployment type is "dev".
func IsDev(deployType string) bool {
	return deployType == "dev"
}

// DebugManageDocker tells if Docker language servers should be managed or not.
//
// This only exists for dev mode / debugging purposes, and should never be used
// in a production setting. It panics if the deploy type is not "dev".
func DebugManageDocker() bool {
	if deployType := DeployType(); !IsDev(deployType) {
		panic(fmt.Sprintf("DebugManageDocker cannot be called except when DEPLOY_TYPE=dev (found %q)", deployType))
	}
	v, err := strconv.ParseBool(os.Getenv("DEBUG_MANAGE_DOCKER"))
	if err != nil {
		// We do not use managed Docker by default in dev mode. This is because
		// there appear to be bugs in Docker For Mac that we encounter due to
		// goreman restarting our frontend process quickly. For example,
		// pkg/langservers/langservers.go tries to stop Docker containers on
		// process shutdown and tries to start them on startup -- and it also
		// tries to create the lsp network on startup -- some combination of
		// these actions occurring at the same time is believed to cause
		// issues such as https://github.com/sourcegraph/sourcegraph/issues/10587
		// where the Docker "lsp" network we create suddenly has an extreme
		// amount of packet loss, and where containers like codeintel-go get
		// stuck permanently in a "starting" state.
		return false
	}
	return v
}
