// SPDX-License-Identifier: Apache-2.0
package matching

import "testing"

func TestExactMatch(t *testing.T) {
	pattern := `{"source": ["my.app"]}`
	event := `{"source": "my.app", "detail-type": "Test"}`
	matched, err := Match(pattern, event)
	if err != nil {
		t.Fatal(err)
	}
	if !matched {
		t.Error("expected match")
	}
}

func TestExactMatchNoMatch(t *testing.T) {
	pattern := `{"source": ["other.app"]}`
	event := `{"source": "my.app"}`
	matched, err := Match(pattern, event)
	if err != nil {
		t.Fatal(err)
	}
	if matched {
		t.Error("expected no match")
	}
}

func TestMultipleValuesOR(t *testing.T) {
	pattern := `{"source": ["app.a", "app.b"]}`
	event := `{"source": "app.b"}`
	matched, err := Match(pattern, event)
	if err != nil {
		t.Fatal(err)
	}
	if !matched {
		t.Error("expected match on second value")
	}
}

func TestMultipleFieldsAND(t *testing.T) {
	pattern := `{"source": ["my.app"], "detail-type": ["OrderPlaced"]}`
	event := `{"source": "my.app", "detail-type": "OrderPlaced"}`
	matched, err := Match(pattern, event)
	if err != nil {
		t.Fatal(err)
	}
	if !matched {
		t.Error("expected match on both fields")
	}
}

func TestMultipleFieldsANDFail(t *testing.T) {
	pattern := `{"source": ["my.app"], "detail-type": ["OrderPlaced"]}`
	event := `{"source": "my.app", "detail-type": "OrderCancelled"}`
	matched, err := Match(pattern, event)
	if err != nil {
		t.Fatal(err)
	}
	if matched {
		t.Error("expected no match when one field mismatches")
	}
}

func TestNestedField(t *testing.T) {
	pattern := `{"detail": {"status": ["active"]}}`
	event := `{"source": "x", "detail": {"status": "active", "count": 5}}`
	matched, err := Match(pattern, event)
	if err != nil {
		t.Fatal(err)
	}
	if !matched {
		t.Error("expected nested match")
	}
}

func TestPrefix(t *testing.T) {
	pattern := `{"source": [{"prefix": "aws."}]}`
	event := `{"source": "aws.ec2"}`
	matched, err := Match(pattern, event)
	if err != nil {
		t.Fatal(err)
	}
	if !matched {
		t.Error("expected prefix match")
	}
}

func TestPrefixNoMatch(t *testing.T) {
	pattern := `{"source": [{"prefix": "aws."}]}`
	event := `{"source": "my.app"}`
	matched, err := Match(pattern, event)
	if err != nil {
		t.Fatal(err)
	}
	if matched {
		t.Error("expected no prefix match")
	}
}

func TestSuffix(t *testing.T) {
	pattern := `{"source": [{"suffix": ".ec2"}]}`
	event := `{"source": "aws.ec2"}`
	matched, err := Match(pattern, event)
	if err != nil {
		t.Fatal(err)
	}
	if !matched {
		t.Error("expected suffix match")
	}
}

func TestAnythingBut(t *testing.T) {
	pattern := `{"source": [{"anything-but": "bad.source"}]}`
	event := `{"source": "good.source"}`
	matched, err := Match(pattern, event)
	if err != nil {
		t.Fatal(err)
	}
	if !matched {
		t.Error("expected anything-but match")
	}
}

func TestAnythingButReject(t *testing.T) {
	pattern := `{"source": [{"anything-but": "bad.source"}]}`
	event := `{"source": "bad.source"}`
	matched, err := Match(pattern, event)
	if err != nil {
		t.Fatal(err)
	}
	if matched {
		t.Error("expected anything-but to reject")
	}
}

func TestAnythingButArray(t *testing.T) {
	pattern := `{"source": [{"anything-but": ["a", "b"]}]}`
	event := `{"source": "c"}`
	matched, err := Match(pattern, event)
	if err != nil {
		t.Fatal(err)
	}
	if !matched {
		t.Error("expected match for value not in exclusion list")
	}
}

func TestNumericGreaterThan(t *testing.T) {
	pattern := `{"detail": {"price": [{"numeric": [">", 100]}]}}`
	event := `{"detail": {"price": 200}}`
	matched, err := Match(pattern, event)
	if err != nil {
		t.Fatal(err)
	}
	if !matched {
		t.Error("expected numeric > match")
	}
}

func TestNumericRange(t *testing.T) {
	pattern := `{"detail": {"price": [{"numeric": [">=", 10, "<=", 100]}]}}`
	event := `{"detail": {"price": 50}}`
	matched, err := Match(pattern, event)
	if err != nil {
		t.Fatal(err)
	}
	if !matched {
		t.Error("expected numeric range match")
	}
}

func TestNumericRangeOutside(t *testing.T) {
	pattern := `{"detail": {"price": [{"numeric": [">=", 10, "<=", 100]}]}}`
	event := `{"detail": {"price": 200}}`
	matched, err := Match(pattern, event)
	if err != nil {
		t.Fatal(err)
	}
	if matched {
		t.Error("expected no match outside range")
	}
}

func TestExistsTrue(t *testing.T) {
	pattern := `{"detail": {"key": [{"exists": true}]}}`
	event := `{"detail": {"key": "value"}}`
	matched, err := Match(pattern, event)
	if err != nil {
		t.Fatal(err)
	}
	if !matched {
		t.Error("expected exists=true match")
	}
}

func TestExistsFalse(t *testing.T) {
	pattern := `{"detail": {"key": [{"exists": false}]}}`
	event := `{"detail": {"other": "value"}}`
	matched, err := Match(pattern, event)
	if err != nil {
		t.Fatal(err)
	}
	if !matched {
		t.Error("expected exists=false match when key is absent")
	}
}

func TestExistsFalseFieldPresent(t *testing.T) {
	pattern := `{"detail": {"key": [{"exists": false}]}}`
	event := `{"detail": {"key": "value"}}`
	matched, err := Match(pattern, event)
	if err != nil {
		t.Fatal(err)
	}
	if matched {
		t.Error("expected no match when key exists but pattern says exists=false")
	}
}

func TestMissingFieldNoMatch(t *testing.T) {
	pattern := `{"source": ["x"]}`
	event := `{"other": "y"}`
	matched, err := Match(pattern, event)
	if err != nil {
		t.Fatal(err)
	}
	if matched {
		t.Error("expected no match when field is missing")
	}
}

func TestEmptyPatternMatchesEverything(t *testing.T) {
	pattern := `{}`
	event := `{"source": "anything"}`
	matched, err := Match(pattern, event)
	if err != nil {
		t.Fatal(err)
	}
	if !matched {
		t.Error("empty pattern should match everything")
	}
}

func TestInvalidPatternJSON(t *testing.T) {
	_, err := Match(`not json`, `{"source": "x"}`)
	if err == nil {
		t.Error("expected error for invalid pattern JSON")
	}
}

func TestInvalidEventJSON(t *testing.T) {
	_, err := Match(`{"source": ["x"]}`, `not json`)
	if err == nil {
		t.Error("expected error for invalid event JSON")
	}
}
