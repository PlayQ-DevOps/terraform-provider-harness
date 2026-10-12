package apierr

import "strings"

const (
	unauthorizedHint = "\nHint:\n" +
		"1) Please check if the token has expired or is wrong.\n" +
		"2) Harness Provider is misconfigured. For FirstGen resources please give the correct api_key and for NextGen resources please give the correct platform_api_key."

	forbiddenHint = "\nHint:\n" +
		"1) Please check if the token has the required permission for this operation.\n" +
		"2) Please check if the token has expired or is wrong."

	nonJSONHint = "\nReceived a non-JSON response from the Harness API. This usually indicates one of the following:\n" +
		"1) The 'endpoint' (HARNESS_ENDPOINT) is incorrect or unreachable.\n" +
		"2) A proxy or load balancer is returning an error page instead of the API response.\n" +
		"3) The Harness API service is temporarily unavailable.\n" +
		"Please verify your provider configuration:\n" +
		"  - endpoint: should be a valid Harness API URL (e.g., https://app.harness.io/gateway)\n" +
		"  - account_id: your Harness account identifier (env: HARNESS_ACCOUNT_ID)\n" +
		"  - platform_api_key: a valid API key (env: HARNESS_PLATFORM_API_KEY)"

	transportHint = "\nHint: the Harness API endpoint could not be reached. Check the 'endpoint' provider setting (HARNESS_ENDPOINT), proxy settings and network connectivity."
)

// Describe renders c for a Terraform diagnostic: the classified message plus the hints
// users of this provider have relied on for authentication, misconfigured endpoints and
// unreachable hosts.
func Describe(c Classified) string {
	if c.Kind == None {
		return ""
	}
	var b strings.Builder
	b.WriteString(c.Error())
	switch c.Kind {
	case Unauthorized:
		b.WriteString(unauthorizedHint)
	case Forbidden:
		b.WriteString(forbiddenHint)
	}
	if c.NonJSON {
		b.WriteString(nonJSONHint)
	}
	if c.Transport {
		b.WriteString(transportHint)
	}
	return b.String()
}

// GitRewrite turns the two Git-related 400 errors the Harness API returns for a bad
// connector reference or repository name into actionable messages. It reports false when
// c is not one of them.
func GitRewrite(c Classified, connectorRef, repoName string) (string, bool) {
	if c.Status != 400 {
		return "", false
	}
	if connectorRef != "" && strings.Contains(c.Message, "No connector found") && strings.Contains(c.Message, connectorRef) {
		return "Invalid connector reference: " + connectorRef + ". Please check if the connector exists and you have the correct permissions.", true
	}
	if repoName != "" && strings.Contains(c.Message, "Please check the requested file path") && strings.Contains(c.Message, repoName) {
		return "Invalid repository name: " + repoName + ". Please check if the repository exists and is accessible.", true
	}
	return "", false
}
