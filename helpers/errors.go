package helpers

// Deprecated wrappers. These keep the historical names compiling while call sites are
// migrated to the lifecycle helpers in lifecycle.go (HandleRead, HandleDelete, HandleWrite,
// HandleWriteGit, HandleError). They will be deleted once the migration is complete; do not
// add new callers.
//
// Behavioural note: the read-mode wrappers now treat every HTTP 404 (and every body the
// classifier recognises as not-found) as "gone", for every SDK client. Previously only the
// nextgen client with specific body codes cleared state, and other bodies could panic.

import (
	"context"
	"net/http"
	"strings"

	"github.com/harness/terraform-provider-harness/internal/apierr"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// messageContains is a transitional predicate for the chaos helpers below; it has no minimum
// phrase length, unlike apierr.MessageHasPhrase, so it reproduces the old behaviour exactly.
func messageContains(phrases ...string) apierr.Predicate {
	return func(c apierr.Classified) bool {
		msg := strings.ToLower(c.Message)
		for _, p := range phrases {
			if p != "" && strings.Contains(msg, strings.ToLower(p)) {
				return true
			}
		}
		return false
	}
}

// Deprecated: use HandleWrite (create/update), HandleDelete (delete) or HandleError (data
// sources and auxiliary calls).
func HandleApiError(err error, d *schema.ResourceData, httpResp *http.Response) diag.Diagnostics {
	return HandleError(context.TODO(), d, err, httpResp)
}

// Deprecated: use HandleRead.
func HandleReadApiError(err error, d *schema.ResourceData, httpResp *http.Response) diag.Diagnostics {
	return HandleRead(context.TODO(), d, err, httpResp)
}

// Deprecated: use HandleWriteGit.
func HandleGitApiError(err error, d *schema.ResourceData, httpResp *http.Response, connectorRef string, repoName string) diag.Diagnostics {
	if err == nil {
		return nil
	}
	c := apierr.Classify(err, httpResp)
	if msg, ok := apierr.GitRewrite(c, connectorRef, repoName); ok {
		return diag.Errorf("%s", msg)
	}
	return HandleWrite(context.TODO(), d, err, httpResp)
}

// Deprecated: use HandleWriteGit.
func HandleGitApiErrorWithResourceData(err error, d *schema.ResourceData, httpResp *http.Response) diag.Diagnostics {
	return HandleWriteGit(context.TODO(), d, err, httpResp)
}

// Deprecated: use HandleWrite, HandleDelete or HandleError.
func HandleDBOpsApiError(err error, d *schema.ResourceData, httpResp *http.Response) diag.Diagnostics {
	return HandleError(context.TODO(), d, err, httpResp)
}

// Deprecated: use HandleRead.
func HandleDBOpsReadApiError(err error, d *schema.ResourceData, httpResp *http.Response) diag.Diagnostics {
	return HandleRead(context.TODO(), d, err, httpResp)
}

// Deprecated: use HandleRead.
func HandleIacmReadApiError(err error, d *schema.ResourceData, httpResp *http.Response) diag.Diagnostics {
	return HandleRead(context.TODO(), d, err, httpResp)
}

// Deprecated: use HandleRead for reads and HandleWrite/HandleDelete elsewhere. The policy
// helper historically cleared state on 404 for every operation, which this preserves.
func HandlePolicyApiError(err error, d *schema.ResourceData, httpResp *http.Response) diag.Diagnostics {
	return HandleRead(context.TODO(), d, err, httpResp)
}

// Deprecated: use HandleWrite, HandleDelete or HandleError.
func HandleCodeApiError(err error, d *schema.ResourceData, httpResp *http.Response) diag.Diagnostics {
	return HandleError(context.TODO(), d, err, httpResp)
}

// Deprecated: use HandleRead.
func HandleCodeReadApiError(err error, d *schema.ResourceData, httpResp *http.Response) diag.Diagnostics {
	return HandleRead(context.TODO(), d, err, httpResp)
}

// Deprecated: use HandleWrite, HandleDelete or HandleError.
func HandleChaosApiError(err error, d *schema.ResourceData, httpResp *http.Response) diag.Diagnostics {
	return HandleError(context.TODO(), d, err, httpResp)
}

// Deprecated: use HandleRead.
func HandleChaosReadApiError(err error, d *schema.ResourceData, httpResp *http.Response) diag.Diagnostics {
	return HandleRead(context.TODO(), d, err, httpResp)
}

// Deprecated: use HandleRead with NotFoundWhen(apierr.And(apierr.StatusIs(500),
// apierr.MessageHasPhrase(...))) and exact phrases.
func HandleChaosReadApiErrorWithGracefulDestroy(err error, d *schema.ResourceData, httpResp *http.Response, gracefulErrorPatterns []string) diag.Diagnostics {
	return HandleRead(context.TODO(), d, err, httpResp, NotFoundWhen(apierr.And(apierr.StatusIs(500), messageContains(gracefulErrorPatterns...))))
}

// Deprecated: use HandleWrite, HandleDelete or HandleError.
func HandleChaosGraphQLError(err error, d *schema.ResourceData, operation string) diag.Diagnostics {
	return HandleError(context.TODO(), d, err, nil)
}

// Deprecated: use HandleRead with a NotFoundWhen predicate naming the object ("hub not found").
func HandleChaosGraphQLReadError(err error, d *schema.ResourceData, operation string) diag.Diagnostics {
	return HandleRead(context.TODO(), d, err, nil, NotFoundWhen(messageContains("not found")))
}
