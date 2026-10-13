package idp

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/harness/terraform-provider-harness/helpers"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type IDPErrorBody struct {
	Message string `json:"message"`
	Code    string `json:"code"`
}

func toIDPErrBody(err error) (*IDPErrorBody, error) {
	var body IDPErrorBody
	apiErr, ok := err.(interface{ Body() []byte })
	if !ok {
		return nil, err
	}

	newErr := json.Unmarshal(apiErr.Body(), &body)
	if newErr != nil {
		return nil, newErr
	}

	return &body, nil
}

func isOnlyVersionDeleteError(err error) bool {
	body, newErr := toIDPErrBody(err)
	if newErr != nil {
		return false
	}

	msg := strings.ToLower(body.Message)
	return strings.Contains(msg, "cannot delete version") &&
		strings.Contains(msg, "at least one version must exist")
}

func isAlreadyExistsError(err error) bool {
	body, newErr := toIDPErrBody(err)
	if newErr != nil {
		return false
	}
	msg := strings.ToLower(body.Message)
	return strings.Contains(msg, "already created") || strings.Contains(msg, "already exists")
}

func isReferencedError(err error) bool {
	body, newErr := toIDPErrBody(err)
	if newErr != nil {
		return false
	}
	return strings.Contains(strings.ToLower(body.Message), "referenced by")
}

// idpNotFound treats an IDP error whose message says "not found" as a missing object: the
// service answers 500 for some missing checks and scorecards.
var idpNotFound = helpers.NotFoundWhen(helpers.MessageContains("not found"))

// idpMessage returns the message of an IDP error body ("CODE: message" when it carries a
// code), or the error text.
func idpMessage(err error) string {
	body, newErr := toIDPErrBody(err)
	if newErr != nil || body.Message == "" {
		return err.Error()
	}
	if body.Code != "" {
		return body.Code + ": " + body.Message
	}
	return body.Message
}

// idpHandleWrite is HandleWrite plus the import hint for IDP's "already exists/created"
// errors, which the service reports as 500s rather than conflicts.
func idpHandleWrite(ctx context.Context, d *schema.ResourceData, terraformType string, err error, httpResp *http.Response) diag.Diagnostics {
	if isAlreadyExistsError(err) {
		id, _ := d.Get("identifier").(string)
		if id == "" {
			id = d.Id()
		}
		return diag.Errorf("%s. Import the existing resource instead of creating it again: terraform import %s.<name> %s", idpMessage(err), terraformType, id)
	}
	return helpers.HandleWrite(ctx, d, err, httpResp)
}

// idpHandleDelete is HandleDelete plus the "referenced by other scorecards" hint.
func idpHandleDelete(ctx context.Context, d *schema.ResourceData, err error, httpResp *http.Response) diag.Diagnostics {
	if isReferencedError(err) {
		return diag.Errorf("%s. Remove the check from those scorecards first, then retry destroy.", idpMessage(err))
	}
	return helpers.HandleDelete(ctx, d, err, httpResp, idpNotFound)
}
