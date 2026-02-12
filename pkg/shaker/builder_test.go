package shaker

import (
	"encoding/json"
	"testing"
)

func TestIncludePipeline(t *testing.T) {
	input := []byte(`{"name":"John","age":30,"email":"john@example.com"}`)
	out, err := From(input).
		Include(".name", ".email").
		Shake()
	if err != nil {
		t.Fatal(err)
	}

	var result map[string]any
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatal(err)
	}
	if result["name"] != "John" || result["email"] != "john@example.com" {
		t.Errorf("got %v", result)
	}
	if _, ok := result["age"]; ok {
		t.Error("age should be excluded")
	}
}

func TestExcludePipeline(t *testing.T) {
	input := []byte(`{"name":"John","password":"secret","email":"john@example.com"}`)
	out, err := From(input).
		Exclude(".password").
		Shake()
	if err != nil {
		t.Fatal(err)
	}

	var result map[string]any
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatal(err)
	}
	if _, ok := result["password"]; ok {
		t.Error("password should be excluded")
	}
	if result["name"] != "John" {
		t.Error("name should be kept")
	}
}

func TestPipelineWithPrefix(t *testing.T) {
	input := []byte(`{"data":{"name":"John","age":30},"meta":"kept"}`)
	out, err := From(input).
		Prefix("$.data").
		Include(".name").
		Shake()
	if err != nil {
		t.Fatal(err)
	}

	var result map[string]any
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatal(err)
	}
	data := result["data"].(map[string]any)
	if data["name"] != "John" {
		t.Error("expected name under data")
	}
}

func TestPipelineChainedInclude(t *testing.T) {
	input := []byte(`{"name":"John","age":30,"email":"john@example.com","phone":"123"}`)
	out, err := From(input).
		Include(".name").
		Include(".email").
		Shake()
	if err != nil {
		t.Fatal(err)
	}

	var result map[string]any
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatal(err)
	}
	if result["name"] != "John" || result["email"] != "john@example.com" {
		t.Errorf("got %v", result)
	}
	if _, ok := result["age"]; ok {
		t.Error("age should be excluded")
	}
}

func TestMustShakePipeline(t *testing.T) {
	input := []byte(`{"name":"John","age":30}`)
	out := From(input).
		Include(".name").
		MustShake()

	var result map[string]any
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatal(err)
	}
	if result["name"] != "John" {
		t.Error("expected name")
	}
}

func TestMustShakePipelinePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic")
		}
	}()

	input := []byte(`{"name":"John"}`)
	From(input).
		Include(".[invalid").
		MustShake()
}
