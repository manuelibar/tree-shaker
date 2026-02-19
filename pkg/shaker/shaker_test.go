package shaker

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestShakeIncludeSingleField(t *testing.T) {
	input := []byte(`{"name":"John","age":30,"email":"john@example.com"}`)
	out, err := Shake(input, Include("$.name"))
	if err != nil {
		t.Fatal(err)
	}

	var result map[string]any
	json.Unmarshal(out, &result)
	if result["name"] != "John" {
		t.Errorf("got %v", result)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 field, got %d", len(result))
	}
}

func TestShakeIncludeMultipleFields(t *testing.T) {
	input := []byte(`{"name":"John","age":30,"email":"john@example.com"}`)
	out, err := Shake(input, Include("$.name", "$.email"))
	if err != nil {
		t.Fatal(err)
	}

	var result map[string]any
	json.Unmarshal(out, &result)
	if result["name"] != "John" || result["email"] != "john@example.com" {
		t.Errorf("got %v", result)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 fields, got %d", len(result))
	}
}

func TestShakeExcludePassword(t *testing.T) {
	input := []byte(`{"name":"John","password":"secret","email":"john@example.com"}`)
	out, err := Shake(input, Exclude("$.password"))
	if err != nil {
		t.Fatal(err)
	}

	var result map[string]any
	json.Unmarshal(out, &result)
	if _, ok := result["password"]; ok {
		t.Error("password should be excluded")
	}
	if result["name"] != "John" || result["email"] != "john@example.com" {
		t.Errorf("got %v", result)
	}
}

func TestShakeExcludeRecursiveDescent(t *testing.T) {
	input := []byte(`{"data":{"name":"John","secret":"x","nested":{"secret":"y","value":1}}}`)
	out, err := Shake(input, Exclude("$..secret"))
	if err != nil {
		t.Fatal(err)
	}

	var result map[string]any
	json.Unmarshal(out, &result)
	data := result["data"].(map[string]any)
	if _, ok := data["secret"]; ok {
		t.Error("data.secret should be excluded")
	}
	nested := data["nested"].(map[string]any)
	if _, ok := nested["secret"]; ok {
		t.Error("nested.secret should be excluded")
	}
	if nested["value"] != float64(1) {
		t.Error("nested.value should be kept")
	}
}

func TestShakeComposability(t *testing.T) {
	input := []byte(`{"name":"John","password":"secret","age":30,"email":"john@example.com"}`)
	// First: exclude password
	out1, err := Shake(input, Exclude("$.password"))
	if err != nil {
		t.Fatal(err)
	}

	// Second: include only name and age
	out2, err := Shake(out1, Include("$.name", "$.age"))
	if err != nil {
		t.Fatal(err)
	}

	var result map[string]any
	json.Unmarshal(out2, &result)
	if result["name"] != "John" {
		t.Error("expected name")
	}
	if result["age"] != float64(30) {
		t.Error("expected age")
	}
	if len(result) != 2 {
		t.Errorf("expected 2 fields, got %d", len(result))
	}
}

func TestShakeErrorAggregation(t *testing.T) {
	input := []byte(`{"name":"John"}`)
	_, err := Shake(input, Include("$.invalid[", "$[bad", "$.valid"))
	if err == nil {
		t.Fatal("expected error")
	}

	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Errorf("expected ParseError in chain, got: %v", err)
	}
}

func TestShakeNoPartialApplication(t *testing.T) {
	input := []byte(`{"name":"John","valid":"yes"}`)
	_, err := Shake(input, Include("$.valid", "$.invalid["))
	if err == nil {
		t.Fatal("expected error — no partial application")
	}
}

func TestShakeInvalidJSON(t *testing.T) {
	_, err := Shake([]byte(`{invalid`), Include("$.name"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestShakeIncludeNoMatchReturnsEmpty(t *testing.T) {
	input := []byte(`{"name":"John"}`)
	out, err := Shake(input, Include("$.nonexistent"))
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "{}" {
		t.Errorf("expected {}, got %s", out)
	}
}

func TestShakeExcludeNoMatchReturnsUnchanged(t *testing.T) {
	input := []byte(`{"name":"John"}`)
	out, err := Shake(input, Exclude("$.nonexistent"))
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	json.Unmarshal(out, &result)
	if result["name"] != "John" {
		t.Error("expected unchanged output")
	}
}

func TestShakeIncludeArrayNoMatch(t *testing.T) {
	input := []byte(`[1,2,3]`)
	out, err := Shake(input, Include("$.nonexistent"))
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "[]" {
		t.Errorf("expected [], got %s", out)
	}
}

func TestMustShakePanicsOnError(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic")
		}
	}()
	MustShake([]byte(`{invalid`), Include("$.name"))
}

func TestMustShakeSuccess(t *testing.T) {
	input := []byte(`{"name":"John","age":30}`)
	out := MustShake(input, Include("$.name"))
	var result map[string]any
	json.Unmarshal(out, &result)
	if result["name"] != "John" {
		t.Error("expected name")
	}
}

func TestShakePrecompiledQuery(t *testing.T) {
	q, err := Include("$.name", "$.email").Compile()
	if err != nil {
		t.Fatal(err)
	}

	docs := []string{
		`{"name":"A","email":"a@x.com","age":1}`,
		`{"name":"B","email":"b@x.com","age":2}`,
		`{"name":"C","email":"c@x.com","age":3}`,
	}

	for _, doc := range docs {
		out, err := Shake([]byte(doc), q)
		if err != nil {
			t.Fatal(err)
		}
		var result map[string]any
		json.Unmarshal(out, &result)
		if result["name"] == nil || result["email"] == nil {
			t.Errorf("expected name and email, got %v", result)
		}
		if _, ok := result["age"]; ok {
			t.Error("age should be excluded")
		}
	}
}

func TestShakeNestedArrayWildcard(t *testing.T) {
	input := []byte(`{"users":[{"name":"A","role":"admin"},{"name":"B","role":"user"}]}`)
	out, err := Shake(input, Include("$.users[*].name"))
	if err != nil {
		t.Fatal(err)
	}

	var result map[string]any
	json.Unmarshal(out, &result)
	users := result["users"].([]any)
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
	for _, u := range users {
		obj := u.(map[string]any)
		if obj["name"] == nil {
			t.Error("expected name")
		}
		if _, ok := obj["role"]; ok {
			t.Error("role should be excluded")
		}
	}
}

func TestShakeMultiSelector(t *testing.T) {
	input := []byte(`[10,20,30,40,50]`)
	out, err := Shake(input, Include("$[0,2,4]"))
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "[10,30,50]" {
		t.Errorf("got %s", out)
	}
}

func TestShakeRequestUnmarshalInclude(t *testing.T) {
	data := []byte(`{"mode":"include","paths":[".name",".email"]}`)
	var r ShakeRequest
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatal(err)
	}
	if !r.Query.IsInclude() {
		t.Error("expected include mode")
	}
}

func TestShakeRequestUnmarshalExclude(t *testing.T) {
	data := []byte(`{"mode":"exclude","paths":[".password"]}`)
	var r ShakeRequest
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatal(err)
	}
	if r.Query.IsInclude() {
		t.Error("expected exclude mode")
	}
}

func TestShakeRequestUnmarshalInvalidMode(t *testing.T) {
	data := []byte(`{"mode":"invalid","paths":[".name"]}`)
	var r ShakeRequest
	if err := json.Unmarshal(data, &r); err == nil {
		t.Error("expected error for invalid mode")
	}
}

func TestShakeRequestUnmarshalEmptyPaths(t *testing.T) {
	data := []byte(`{"mode":"include","paths":[]}`)
	var r ShakeRequest
	if err := json.Unmarshal(data, &r); err == nil {
		t.Error("expected error for empty paths")
	}
}

func TestShakeRequestUnmarshalEmptyMode(t *testing.T) {
	data := []byte(`{"mode":"","paths":[".name"]}`)
	var r ShakeRequest
	if err := json.Unmarshal(data, &r); err == nil {
		t.Error("expected error for empty mode")
	}
}

func TestShakePreservesLargeNumbers(t *testing.T) {
	// 9007199254740993 is 2^53 + 1, beyond float64 exact precision.
	// json.Number (via dec.UseNumber) preserves the string representation.
	input := []byte(`{"id":9007199254740993,"name":"big"}`)

	t.Run("include", func(t *testing.T) {
		out, err := Shake(input, Include("$.id"))
		if err != nil {
			t.Fatal(err)
		}
		if string(out) != `{"id":9007199254740993}` {
			t.Errorf("got %s, number may have lost precision", out)
		}
	})

	t.Run("exclude", func(t *testing.T) {
		out, err := Shake(input, Exclude("$.name"))
		if err != nil {
			t.Fatal(err)
		}
		if string(out) != `{"id":9007199254740993}` {
			t.Errorf("got %s, number may have lost precision", out)
		}
	})
}
