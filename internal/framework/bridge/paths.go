package bridge

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	sdk "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// pathValidators are the cross-attribute constraints of one SDK attribute, as framework
// path expressions. The attribute's own path is left out: the framework validators add
// it themselves.
type pathValidators struct {
	exactlyOneOf, conflictsWith, atLeastOneOf, alsoRequires []path.Expression
}

func constraintsOf(s *sdk.Schema, self string, root map[string]*sdk.Schema) (pathValidators, error) {
	convert := func(keys []string) ([]path.Expression, error) {
		var out []path.Expression
		for _, k := range keys {
			if k == self {
				continue
			}
			e, err := expression(k, root)
			if err != nil {
				return nil, err
			}
			out = append(out, e)
		}
		return out, nil
	}
	var pv pathValidators
	var err error
	if pv.exactlyOneOf, err = convert(s.ExactlyOneOf); err != nil {
		return pv, err
	}
	if pv.conflictsWith, err = convert(s.ConflictsWith); err != nil {
		return pv, err
	}
	if pv.atLeastOneOf, err = convert(s.AtLeastOneOf); err != nil {
		return pv, err
	}
	if pv.alsoRequires, err = convert(s.RequiredWith); err != nil {
		return pv, err
	}
	return pv, nil
}

// expression converts an SDK attribute path ("credentials.0.http.0.username") into a
// framework path expression, resolving numeric segments against the SDK schema: an index
// into a list, any value of a set.
func expression(key string, root map[string]*sdk.Schema) (path.Expression, error) {
	segs := strings.Split(key, ".")
	var expr path.Expression
	current := root
	var parent *sdk.Schema
	for i, seg := range segs {
		if n, err := strconv.Atoi(seg); err == nil {
			if parent == nil {
				return expr, fmt.Errorf("path %q: index without a collection", key)
			}
			switch parent.Type {
			case sdk.TypeList:
				expr = expr.AtListIndex(n)
			case sdk.TypeSet:
				expr = expr.AtAnySetValue()
			default:
				return expr, fmt.Errorf("path %q: %q is not a list or set", key, segs[i-1])
			}
			continue
		}
		s, ok := current[seg]
		if !ok {
			return expr, fmt.Errorf("path %q: unknown attribute %q", key, seg)
		}
		if i == 0 {
			expr = path.MatchRoot(seg)
		} else {
			expr = expr.AtName(seg)
		}
		parent = s
		if res, ok := s.Elem.(*sdk.Resource); ok {
			current = res.Schema
		} else {
			current = nil
		}
	}
	return expr, nil
}

// joinKey builds the SDK key path of a nested attribute: list and set elements are
// addressed with ".0." as the SDK constraint lists do.
func joinKey(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + ".0." + name
}
