package sweep_test

import (
	"os"
	"strings"
	"testing"

	"github.com/harness/harness-go-sdk/harness/cd"
	_ "github.com/harness/terraform-provider-harness/internal/service/cd/application"
	_ "github.com/harness/terraform-provider-harness/internal/service/cd/cloudprovider"
	_ "github.com/harness/terraform-provider-harness/internal/service/cd/secrets"
	_ "github.com/harness/terraform-provider-harness/internal/service/split"
	"github.com/harness/terraform-provider-harness/internal/sweep"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

// sweepRequested reports whether this test binary was invoked to run sweepers
// (`go test ./internal/sweep -sweep=...`). Only then do we need a live client;
// a plain `go test ./...` without credentials must not fail here.
func sweepRequested() bool {
	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "-sweep") || strings.HasPrefix(arg, "--sweep") {
			return true
		}
	}
	return false
}

func TestMain(m *testing.M) {
	if sweepRequested() || os.Getenv(resource.EnvTfAcc) != "" {
		client, err := cd.NewClient(cd.DefaultConfig())
		if err != nil {
			panic(err)
		}
		sweep.SweeperClient = client
	}

	resource.TestMain(m)
}
