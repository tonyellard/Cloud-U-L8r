package storage

import (
	"testing"

	"github.com/tonyellard/kay-vee/internal/model"
)

func TestPutParameterVersioning(t *testing.T) {
	store := NewStore("us-east-1", "000000000000")

	first, err := store.PutParameter(model.PutParameterRequest{
		Name:  "/app/dev/key",
		Type:  "String",
		Value: "one",
	})
	if err != nil {
		t.Fatalf("unexpected error on first put: %v", err)
	}
	if first.Version != 1 {
		t.Fatalf("expected version 1, got %d", first.Version)
	}

	second, err := store.PutParameter(model.PutParameterRequest{
		Name:      "/app/dev/key",
		Type:      "String",
		Value:     "two",
		Overwrite: true,
	})
	if err != nil {
		t.Fatalf("unexpected error on overwrite: %v", err)
	}
	if second.Version != 2 {
		t.Fatalf("expected version 2, got %d", second.Version)
	}

	param, err := store.GetParameter("/app/dev/key", false)
	if err != nil {
		t.Fatalf("unexpected error on get: %v", err)
	}
	if param.Value != "two" {
		t.Fatalf("expected latest value 'two', got %q", param.Value)
	}
}

func TestSecretStageTransitions(t *testing.T) {
	store := NewStore("us-east-1", "000000000000")

	secretValue1 := "first"
	created, err := store.CreateSecret(model.CreateSecretRequest{
		Name:         "app/dev/secret",
		SecretString: &secretValue1,
	})
	if err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}
	if created.VersionID == "" {
		t.Fatalf("expected version id on create")
	}

	secretValue2 := "second"
	updated, err := store.PutSecretValue(model.PutSecretValueRequest{
		SecretID:     "app/dev/secret",
		SecretString: &secretValue2,
	})
	if err != nil {
		t.Fatalf("unexpected put secret value error: %v", err)
	}
	if updated.VersionID == created.VersionID {
		t.Fatalf("expected new version id after put secret value")
	}

	current, err := store.GetSecretValue(model.GetSecretValueRequest{SecretID: "app/dev/secret"})
	if err != nil {
		t.Fatalf("unexpected get current error: %v", err)
	}
	if current.SecretString == nil || *current.SecretString != "second" {
		t.Fatalf("expected current value 'second', got %#v", current.SecretString)
	}

	previous, err := store.GetSecretValue(model.GetSecretValueRequest{SecretID: "app/dev/secret", VersionStage: "AWSPREVIOUS"})
	if err != nil {
		t.Fatalf("unexpected get previous error: %v", err)
	}
	if previous.SecretString == nil || *previous.SecretString != "first" {
		t.Fatalf("expected previous value 'first', got %#v", previous.SecretString)
	}
}
