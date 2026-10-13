package bridge

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	sdk "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

var ctx = context.Background()

// sample covers the shapes the connector schemas use.
func sample() *sdk.Resource {
	return &sdk.Resource{
		Description: "Sample resource.",
		Schema: map[string]*sdk.Schema{
			"identifier": {Type: sdk.TypeString, Required: true, ForceNew: true, Description: "Identifier."},
			"url": {Type: sdk.TypeString, Required: true, Description: "URL.", DiffSuppressFunc: func(_, old, new string, _ *sdk.ResourceData) bool {
				return strings.TrimSuffix(old, "/") == strings.TrimSuffix(new, "/")
			}},
			"kind":      {Type: sdk.TypeString, Optional: true, Default: "Account", ValidateFunc: validation.StringInSlice([]string{"Account", "Repo"}, false), Description: "Kind."},
			"enabled":   {Type: sdk.TypeBool, Optional: true, Default: true, Description: "Enabled."},
			"retries":   {Type: sdk.TypeInt, Optional: true, Description: "Count."},
			"selectors": {Type: sdk.TypeSet, Optional: true, Elem: &sdk.Schema{Type: sdk.TypeString}, Description: "Selectors."},
			"labels":    {Type: sdk.TypeMap, Optional: true, Elem: &sdk.Schema{Type: sdk.TypeString}, Description: "Labels."},
			"token":     {Type: sdk.TypeString, Optional: true, Sensitive: true, Deprecated: "use token_ref", Description: "Token."},
			"credentials": {Type: sdk.TypeList, Required: true, MaxItems: 1, Description: "Credentials.", Elem: &sdk.Resource{Schema: map[string]*sdk.Schema{
				"http": {Type: sdk.TypeList, Optional: true, MaxItems: 1, ConflictsWith: []string{"credentials.0.ssh"}, ExactlyOneOf: []string{"credentials.0.http", "credentials.0.ssh"}, Description: "HTTP.", Elem: &sdk.Resource{Schema: map[string]*sdk.Schema{
					"username":     {Type: sdk.TypeString, Optional: true, ExactlyOneOf: []string{"credentials.0.http.0.username", "credentials.0.http.0.username_ref"}, Description: "Username."},
					"username_ref": {Type: sdk.TypeString, Optional: true, ExactlyOneOf: []string{"credentials.0.http.0.username", "credentials.0.http.0.username_ref"}, Description: "Username ref."},
					"token_ref":    {Type: sdk.TypeString, Required: true, Description: "Token ref."},
				}}},
				"ssh": {Type: sdk.TypeList, Optional: true, MaxItems: 1, ConflictsWith: []string{"credentials.0.http"}, ExactlyOneOf: []string{"credentials.0.http", "credentials.0.ssh"}, Description: "SSH.", Elem: &sdk.Resource{Schema: map[string]*sdk.Schema{
					"ssh_key_ref": {Type: sdk.TypeString, Required: true, Description: "Key ref."},
				}}},
			}}},
			"status": {Type: sdk.TypeList, Computed: true, Description: "Status.", Elem: &sdk.Resource{Schema: map[string]*sdk.Schema{
				"state": {Type: sdk.TypeString, Computed: true, Description: "State."},
			}}},
		},
	}
}

func TestResourceSchema(t *testing.T) {
	s, err := ResourceSchema(sample())
	if err != nil {
		t.Fatal(err)
	}
	if diags := s.ValidateImplementation(ctx); diags.HasError() {
		t.Fatalf("framework rejects the converted schema: %v", diags)
	}
	if s.Attributes["id"] == nil || !s.Attributes["id"].IsComputed() {
		t.Error("the implicit id attribute is added as Computed")
	}
	if a := s.Attributes["identifier"].(schema.StringAttribute); len(a.PlanModifiers) != 1 || !a.IsRequired() {
		t.Errorf("identifier: ForceNew becomes RequiresReplace: %+v", a)
	}
	if a := s.Attributes["kind"].(schema.StringAttribute); !a.IsComputed() || a.Default == nil || len(a.Validators) != 1 {
		t.Errorf("kind: a Default makes the attribute Computed with a static default and keeps the ValidateFunc: %+v", a)
	}
	if a := s.Attributes["url"].(schema.StringAttribute); len(a.PlanModifiers) != 1 {
		t.Errorf("url: a DiffSuppressFunc becomes a plan modifier: %+v", a)
	}
	if a := s.Attributes["token"].(schema.StringAttribute); !a.IsSensitive() || a.GetDeprecationMessage() == "" {
		t.Error("token: Sensitive and Deprecated are kept")
	}
	creds, ok := s.Blocks["credentials"].(schema.ListNestedBlock)
	if !ok {
		t.Fatalf("credentials is a list nested block, got %T", s.Blocks["credentials"])
	}
	if len(creds.Validators) != 2 {
		t.Errorf("a Required block with MaxItems 1 gets size validators, got %d", len(creds.Validators))
	}
	httpBlock := creds.NestedObject.Blocks["http"].(schema.ListNestedBlock)
	if len(httpBlock.Validators) != 3 {
		t.Errorf("http: MaxItems, ConflictsWith and ExactlyOneOf become validators, got %d", len(httpBlock.Validators))
	}
	if u := httpBlock.NestedObject.Attributes["username"].(schema.StringAttribute); len(u.Validators) != 1 {
		t.Errorf("username: ExactlyOneOf becomes one validator (self excluded), got %d", len(u.Validators))
	}
	if st, ok := s.Attributes["status"].(schema.ListAttribute); !ok || !st.IsComputed() {
		t.Errorf("a Computed-only nested resource becomes a list of objects attribute, got %T", s.Attributes["status"])
	}
	if _, err := expression("credentials.0.http.0.username", sample().Schema); err != nil {
		t.Error(err)
	}
	if _, err := expression("credentials.0.nope", sample().Schema); err == nil {
		t.Error("an unknown path is an error")
	}
}

func TestDataSourceSchema(t *testing.T) {
	s, err := DataSourceSchema(sample())
	if err != nil {
		t.Fatal(err)
	}
	if diags := s.ValidateImplementation(ctx); diags.HasError() {
		t.Fatalf("framework rejects the converted data source schema: %v", diags)
	}
	if _, ok := s.Blocks["credentials"]; !ok {
		t.Error("settable nested resources stay blocks in data sources")
	}
}
