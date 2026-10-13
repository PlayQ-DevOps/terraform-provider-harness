package provider

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/harness/harness-go-sdk/harness/cd"
	"github.com/harness/harness-go-sdk/harness/chaos"
	"github.com/harness/harness-go-sdk/harness/code"
	"github.com/harness/harness-go-sdk/harness/dbops"
	"github.com/harness/harness-go-sdk/harness/har"
	"github.com/harness/harness-go-sdk/harness/helpers"
	"github.com/harness/harness-go-sdk/harness/idp"
	"github.com/harness/harness-go-sdk/harness/nextgen"
	"github.com/harness/harness-go-sdk/harness/po"
	"github.com/harness/harness-go-sdk/harness/policymgmt"
	"github.com/harness/harness-go-sdk/harness/split"
	"github.com/harness/harness-go-sdk/harness/svcdiscovery"
	openapi_client_nextgen "github.com/harness/harness-openapi-go-client/nextgen"
	"github.com/harness/terraform-provider-harness/internal"
	"github.com/harness/terraform-provider-harness/internal/httpclient"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// The provider configuration and the session built from it are shared by the two halves
// of the muxed provider: the SDK v2 provider's configure and the plugin-framework
// provider's Configure both fill a SessionConfig and call NewSession, so every API client
// is built exactly one way.

const (
	defaultMaxRetries     = 5
	defaultRequestTimeout = 120 * time.Second

	envMaxRetries          = "HARNESS_MAX_RETRIES"
	envRequestTimeout      = "HARNESS_REQUEST_TIMEOUT"
	envFMEAdminAPIEndpoint = "FME_ADMIN_API_ENDPOINT"
)

// SessionConfig is the provider configuration after defaults and environment variables
// have been applied.
type SessionConfig struct {
	Endpoint string
	// FMEAdminAPIEndpoint overrides the FME admin API endpoint; when empty it is derived
	// from Endpoint.
	FMEAdminAPIEndpoint string
	AccountID           string
	APIKey              string
	PlatformAPIKey      string
	MaxRetries          int
	RequestTimeout      time.Duration
	Version             string
}

// Warning is a configure-time problem that does not stop the provider.
type Warning struct {
	Summary string
	Detail  string
}

// Validate reports what the provider has always reported at configure time: a malformed
// endpoint is an error; a missing account id or API key is a warning, because a
// configuration may only use resources that need neither.
func (c SessionConfig) Validate() ([]Warning, error) {
	parsed, err := url.Parse(c.Endpoint)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf(
			"Invalid Harness API endpoint %q. The endpoint must be a valid URL with a scheme (http/https) and host. "+
				"Example: https://app.harness.io/gateway. "+
				"This can be set using the 'endpoint' provider attribute or the %s environment variable.",
			c.Endpoint, helpers.EnvVars.Endpoint.String(),
		)
	}
	var warnings []Warning
	if c.AccountID == "" {
		warnings = append(warnings, Warning{
			Summary: "Harness account_id is not configured",
			Detail: fmt.Sprintf(
				"The account_id is not set. Most Harness resources require an account_id to function correctly. "+
					"Set it in the provider configuration or via the %s environment variable.",
				helpers.EnvVars.AccountId.String(),
			),
		})
	}
	if c.APIKey == "" && c.PlatformAPIKey == "" {
		warnings = append(warnings, Warning{
			Summary: "No Harness API key configured",
			Detail: fmt.Sprintf(
				"Neither api_key nor platform_api_key is set. API operations will fail without authentication. "+
					"Set platform_api_key (env: %s) for NextGen/Platform resources or api_key (env: %s) for FirstGen resources. "+
					"See https://registry.terraform.io/providers/harness/harness/latest/docs#api_key-1 for more details.",
				helpers.EnvVars.PlatformApiKey.String(), helpers.EnvVars.ApiKey.String(),
			),
		})
	}
	return warnings, nil
}

// envInt reads an integer environment variable, def when it is unset.
func envInt(key string, def int) (int, error) {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer, got %q", key, v)
	}
	return n, nil
}

func envIntDefaultFunc(key string, def int) schema.SchemaDefaultFunc {
	return func() (interface{}, error) {
		return envInt(key, def)
	}
}

// resolveFMEAdminAPIEndpoint derives the FME admin API endpoint from the Harness endpoint
// unless an override is given.
func resolveFMEAdminAPIEndpoint(endpoint, override string) (string, error) {
	if override != "" {
		return override, nil
	}
	endpoint = strings.TrimRight(endpoint, "/")
	if !strings.HasSuffix(endpoint, "/gateway") {
		return "", fmt.Errorf("cannot derive FME admin API endpoint from %q: endpoint must end in /gateway", endpoint)
	}
	return strings.TrimSuffix(endpoint, "/gateway") + "/fme", nil
}

func (c SessionConfig) userAgent() string {
	return fmt.Sprintf("terraform-provider-harness-%s", c.Version)
}

func (c SessionConfig) platformUserAgent() string {
	return fmt.Sprintf("terraform-provider-harness-platform-%s", c.Version)
}

// httpConfig is the shared HTTP client policy (retries, timeout, user agent, redaction)
// every SDK client is built from.
func (c SessionConfig) httpConfig() httpclient.Config {
	return httpclient.Config{
		MaxRetries:     c.MaxRetries,
		RequestTimeout: c.RequestTimeout,
		UserAgent:      c.userAgent(),
		Secrets:        []string{c.APIKey, c.PlatformAPIKey},
	}
}

// NewSession builds every API client from the configuration. The FME (Split) client is
// only built when the FME admin API endpoint can be derived; the session records why it
// could not, so FME resources can report it.
func NewSession(ctx context.Context, c SessionConfig) *internal.Session {
	hc := c.httpConfig()
	session := &internal.Session{
		AccountId:    c.AccountID,
		Endpoint:     c.Endpoint,
		CDClient:     c.cdClient(ctx, hc),
		PLClient:     c.platformClient(hc),
		Client:       c.openAPIClient(hc),
		CodeClient:   c.codeClient(hc),
		DBOpsClient:  c.dbOpsClient(hc),
		ChaosClient:  c.chaosClient(hc),
		SDClient:     c.serviceDiscoveryClient(hc),
		HARClient:    c.harClient(hc),
		IDPClient:    c.idpClient(hc),
		POClient:     c.poClient(hc),
		PolicyClient: c.policyClient(hc),
	}
	fmeEndpoint, err := resolveFMEAdminAPIEndpoint(c.Endpoint, c.FMEAdminAPIEndpoint)
	session.FMEAdminAPIEndpointError = err
	if err == nil {
		session.SplitClient = c.splitClient(fmeEndpoint, hc)
	}
	return session
}

func (c SessionConfig) cdClient(ctx context.Context, hc httpclient.Config) *cd.ApiClient {
	cfg := cd.DefaultConfig()
	cfg.AccountId = c.AccountID
	cfg.Endpoint = c.Endpoint
	cfg.APIKey = c.APIKey
	cfg.UserAgent = c.userAgent()
	cfg.HTTPClient = httpclient.NewRetryable(hc)
	cfg.DebugLogging = false
	client, err := cd.NewClient(cfg)
	if err != nil {
		tflog.Warn(ctx, "error creating CD client", map[string]interface{}{"error": err.Error()})
	}
	return client
}

func (c SessionConfig) platformClient(hc httpclient.Config) *nextgen.APIClient {
	return nextgen.NewAPIClient(&nextgen.Configuration{
		AccountId:  c.AccountID,
		BasePath:   c.Endpoint,
		ApiKey:     c.PlatformAPIKey,
		UserAgent:  c.platformUserAgent(),
		HTTPClient: httpclient.NewRetryable(hc),
	})
}

func (c SessionConfig) dbOpsClient(hc httpclient.Config) *dbops.APIClient {
	return dbops.NewAPIClient(&dbops.Configuration{
		AccountId:  c.AccountID,
		BasePath:   c.Endpoint,
		ApiKey:     c.PlatformAPIKey,
		UserAgent:  c.platformUserAgent(),
		HTTPClient: httpclient.NewStandard(hc),
	})
}

func (c SessionConfig) openAPIClient(hc httpclient.Config) *openapi_client_nextgen.APIClient {
	return openapi_client_nextgen.NewAPIClient(&openapi_client_nextgen.Configuration{
		AccountId:  c.AccountID,
		BasePath:   c.Endpoint,
		ApiKey:     c.PlatformAPIKey,
		UserAgent:  c.platformUserAgent(),
		HTTPClient: httpclient.NewRetryable(hc),
	})
}

func (c SessionConfig) codeClient(hc httpclient.Config) *code.APIClient {
	return code.NewAPIClient(&code.Configuration{
		AccountId:     c.AccountID,
		BasePath:      c.Endpoint + "/code/api/v1", // the SDK does not add the service prefix itself
		ApiKey:        c.PlatformAPIKey,
		UserAgent:     c.platformUserAgent(),
		HTTPClient:    httpclient.NewRetryable(hc),
		DefaultHeader: map[string]string{"X-Api-Key": c.PlatformAPIKey},
	})
}

func (c SessionConfig) chaosClient(hc httpclient.Config) *chaos.APIClient {
	return chaos.NewAPIClient(&chaos.Configuration{
		AccountId:     c.AccountID,
		BasePath:      c.Endpoint + "/chaos/manager/api",
		ApiKey:        c.PlatformAPIKey,
		UserAgent:     c.platformUserAgent(),
		HTTPClient:    httpclient.NewStandard(hc),
		DefaultHeader: map[string]string{"X-Api-Key": c.PlatformAPIKey},
	})
}

func (c SessionConfig) serviceDiscoveryClient(hc httpclient.Config) *svcdiscovery.APIClient {
	return svcdiscovery.NewAPIClient(&svcdiscovery.Configuration{
		AccountId:     c.AccountID,
		BasePath:      c.Endpoint + "/servicediscovery",
		ApiKey:        c.PlatformAPIKey,
		UserAgent:     c.platformUserAgent(),
		HTTPClient:    httpclient.NewStandard(hc),
		DefaultHeader: map[string]string{"X-Api-Key": c.PlatformAPIKey},
	})
}

func (c SessionConfig) harClient(hc httpclient.Config) *har.APIClient {
	return har.NewAPIClient(&har.Configuration{
		AccountId:     c.AccountID,
		BasePath:      c.Endpoint + "/har/api/v1",
		BasePathV3:    c.Endpoint + "/har/api/v3",
		ApiKey:        c.PlatformAPIKey,
		UserAgent:     c.platformUserAgent(),
		HTTPClient:    httpclient.NewRetryable(hc),
		DefaultHeader: map[string]string{"X-Api-Key": c.PlatformAPIKey},
	})
}

func (c SessionConfig) idpClient(hc httpclient.Config) *idp.APIClient {
	return idp.NewAPIClient(&idp.Configuration{
		AccountId:  c.AccountID,
		BasePath:   c.Endpoint,
		ApiKey:     c.PlatformAPIKey,
		UserAgent:  c.platformUserAgent(),
		HTTPClient: httpclient.NewRetryable(hc),
	})
}

func (c SessionConfig) poClient(hc httpclient.Config) *po.APIClient {
	return po.NewAPIClient(&po.Configuration{
		AccountId:     c.AccountID,
		BasePath:      c.Endpoint + "/po",
		ApiKey:        c.PlatformAPIKey,
		UserAgent:     c.platformUserAgent(),
		HTTPClient:    httpclient.NewRetryable(hc),
		DefaultHeader: map[string]string{"X-Api-Key": c.PlatformAPIKey},
	})
}

func (c SessionConfig) policyClient(hc httpclient.Config) *policymgmt.APIClient {
	cfg := policymgmt.NewConfiguration()
	cfg.BasePath = c.Endpoint + "/pm"
	cfg.UserAgent = c.platformUserAgent()
	cfg.HTTPClient = httpclient.NewRetryable(hc)
	cfg.DebugLogging = false
	return policymgmt.NewAPIClient(cfg)
}

// splitClient returns the FME (Split) client. The Split SDK layers its own rate-limit and
// retry transport on the plain client it is given.
func (c SessionConfig) splitClient(basePath string, hc httpclient.Config) *split.APIClient {
	return split.NewAPIClient(&split.Configuration{
		AccountId:     c.AccountID,
		BasePath:      basePath,
		ApiKey:        c.PlatformAPIKey,
		UserAgent:     c.userAgent(),
		HTTPClient:    httpclient.NewPlain(hc),
		DefaultHeader: map[string]string{"X-Api-Key": c.PlatformAPIKey},
	})
}
