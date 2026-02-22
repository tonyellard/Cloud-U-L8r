// SPDX-License-Identifier: Apache-2.0
package matching

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Match checks whether a JSON event matches an EventBridge event pattern.
//
// Supported operators:
//   - Exact value:  {"source": ["my.source"]}
//   - Prefix:       {"source": [{"prefix": "my."}]}
//   - Suffix:       {"source": [{"suffix": ".src"}]}
//   - Anything-but: {"source": [{"anything-but": "bad"}]}
//   - Numeric:      {"detail": {"count": [{"numeric": [">=", 10]}]}}
//   - Exists:       {"detail": {"key": [{"exists": true}]}}
//   - Nested:       {"detail": {"sub": {"key": ["value"]}}}
func Match(pattern, event string) (bool, error) {
	var patMap map[string]interface{}
	if err := json.Unmarshal([]byte(pattern), &patMap); err != nil {
		return false, fmt.Errorf("invalid event pattern: %w", err)
	}

	var evtMap map[string]interface{}
	if err := json.Unmarshal([]byte(event), &evtMap); err != nil {
		return false, fmt.Errorf("invalid event: %w", err)
	}

	return matchObject(patMap, evtMap)
}

func matchObject(pattern, event map[string]interface{}) (bool, error) {
	for key, patVal := range pattern {
		evtVal, exists := event[key]

		switch pv := patVal.(type) {
		case map[string]interface{}:
			evtObj, ok := evtVal.(map[string]interface{})
			if !ok {
				if s, ok := evtVal.(string); ok {
					var parsed map[string]interface{}
					if err := json.Unmarshal([]byte(s), &parsed); err == nil {
						evtObj = parsed
					} else {
						return false, nil
					}
				} else if !exists {
					if allExistsFalse(pv) {
						continue
					}
					return false, nil
				} else {
					return false, nil
				}
			}
			matched, err := matchObject(pv, evtObj)
			if err != nil || !matched {
				return false, err
			}

		case []interface{}:
			if !exists {
				hasExistsFalse := false
				for _, m := range pv {
					if obj, ok := m.(map[string]interface{}); ok {
						if ex, ok := obj["exists"]; ok {
							if b, ok := ex.(bool); ok && !b {
								hasExistsFalse = true
								break
							}
						}
					}
				}
				if hasExistsFalse {
					continue
				}
				return false, nil
			}
			matched, err := matchValue(pv, evtVal)
			if err != nil {
				return false, err
			}
			if !matched {
				return false, nil
			}

		default:
			return false, fmt.Errorf("invalid pattern value type for key %q", key)
		}
	}
	return true, nil
}

func allExistsFalse(pattern map[string]interface{}) bool {
	for _, v := range pattern {
		switch pv := v.(type) {
		case []interface{}:
			for _, m := range pv {
				obj, ok := m.(map[string]interface{})
				if !ok {
					return false
				}
				ex, ok := obj["exists"]
				if !ok {
					return false
				}
				b, ok := ex.(bool)
				if !ok || b {
					return false
				}
			}
		case map[string]interface{}:
			if !allExistsFalse(pv) {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func matchValue(matchers []interface{}, eventVal interface{}) (bool, error) {
	for _, m := range matchers {
		switch matcher := m.(type) {
		case string:
			if s, ok := eventVal.(string); ok && s == matcher {
				return true, nil
			}
		case float64:
			if n, ok := eventVal.(float64); ok && n == matcher {
				return true, nil
			}
		case bool:
			if b, ok := eventVal.(bool); ok && b == matcher {
				return true, nil
			}
		case nil:
			if eventVal == nil {
				return true, nil
			}
		case map[string]interface{}:
			matched, err := matchSpecial(matcher, eventVal)
			if err != nil {
				return false, err
			}
			if matched {
				return true, nil
			}
		}
	}
	return false, nil
}

func matchSpecial(matcher map[string]interface{}, eventVal interface{}) (bool, error) {
	if prefix, ok := matcher["prefix"]; ok {
		p, ok := prefix.(string)
		if !ok {
			return false, nil
		}
		s, ok := eventVal.(string)
		if !ok {
			return false, nil
		}
		return strings.HasPrefix(s, p), nil
	}

	if suffix, ok := matcher["suffix"]; ok {
		sf, ok := suffix.(string)
		if !ok {
			return false, nil
		}
		s, ok := eventVal.(string)
		if !ok {
			return false, nil
		}
		return strings.HasSuffix(s, sf), nil
	}

	if ab, ok := matcher["anything-but"]; ok {
		return matchAnythingBut(ab, eventVal)
	}

	if num, ok := matcher["numeric"]; ok {
		return matchNumeric(num, eventVal)
	}

	if ex, ok := matcher["exists"]; ok {
		b, ok := ex.(bool)
		if !ok {
			return false, nil
		}
		fieldExists := eventVal != nil
		return fieldExists == b, nil
	}

	return false, nil
}

func matchAnythingBut(excluded, eventVal interface{}) (bool, error) {
	switch ex := excluded.(type) {
	case string:
		s, ok := eventVal.(string)
		if !ok {
			return true, nil
		}
		return s != ex, nil
	case float64:
		n, ok := eventVal.(float64)
		if !ok {
			return true, nil
		}
		return n != ex, nil
	case []interface{}:
		for _, item := range ex {
			switch v := item.(type) {
			case string:
				if s, ok := eventVal.(string); ok && s == v {
					return false, nil
				}
			case float64:
				if n, ok := eventVal.(float64); ok && n == v {
					return false, nil
				}
			}
		}
		return true, nil
	}
	return true, nil
}

func matchNumeric(spec, eventVal interface{}) (bool, error) {
	arr, ok := spec.([]interface{})
	if !ok || len(arr) == 0 {
		return false, nil
	}

	evtNum, ok := eventVal.(float64)
	if !ok {
		return false, nil
	}

	for i := 0; i+1 < len(arr); i += 2 {
		op, ok := arr[i].(string)
		if !ok {
			return false, nil
		}
		val, ok := arr[i+1].(float64)
		if !ok {
			return false, nil
		}

		switch op {
		case "=":
			if evtNum != val {
				return false, nil
			}
		case ">":
			if evtNum <= val {
				return false, nil
			}
		case ">=":
			if evtNum < val {
				return false, nil
			}
		case "<":
			if evtNum >= val {
				return false, nil
			}
		case "<=":
			if evtNum > val {
				return false, nil
			}
		default:
			return false, fmt.Errorf("unknown numeric operator: %s", op)
		}
	}

	return true, nil
}
