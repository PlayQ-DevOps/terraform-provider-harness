package helpers

import (
	"sort"
	"strings"

	"github.com/harness/harness-go-sdk/harness/nextgen"
)

// ExpandTags converts Terraform "key:value" tag strings to the map the Harness APIs take.
// Only the first colon separates key from value, so values may themselves contain colons
// (URLs, Harness expressions such as "<+condition?a:b>", timestamps). A tag without a
// colon is a key with an empty value.
func ExpandTags(tags []interface{}) map[string]string {
	result := make(map[string]string, len(tags))
	for _, tag := range tags {
		parts := strings.SplitN(tag.(string), ":", 2)
		if len(parts) == 1 {
			result[parts[0]] = ""
		} else {
			result[parts[0]] = parts[1]
		}
	}
	return result
}

// FlattenTags is the inverse of ExpandTags. The result is sorted so list-typed tag
// attributes do not show spurious ordering diffs.
func FlattenTags(tags map[string]string) []string {
	result := make([]string, 0, len(tags))
	for k, v := range tags {
		if v == "" {
			result = append(result, k)
		} else {
			result = append(result, k+":"+v)
		}
	}
	sort.Strings(result)
	return result
}

func ExpandScopeSelector(scopeSelectors []interface{}) []nextgen.ScopeSelector {
	var result []nextgen.ScopeSelector
	for _, scopeSelector := range scopeSelectors {
		v := scopeSelector.(map[string]interface{})

		var resultScopeSelector nextgen.ScopeSelector
		resultScopeSelector.Filter = v["filter"].(string)
		resultScopeSelector.AccountIdentifier = v["account_id"].(string)
		resultScopeSelector.OrgIdentifier = v["org_id"].(string)
		resultScopeSelector.ProjectIdentifier = v["project_id"].(string)
		result = append(result, resultScopeSelector)
	}
	return result
}

func ExpandField(permissions []interface{}) []string {
	var result []string
	for _, permission := range permissions {
		result = append(result, permission.(string))
	}
	return result
}
