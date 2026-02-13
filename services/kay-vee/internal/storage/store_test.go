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

func TestGetParametersByPath(t *testing.T) {
	store := NewStore("us-east-1", "000000000000")

	_, _ = store.PutParameter(model.PutParameterRequest{Name: "/app/url", Type: "String", Value: "u1"})
	_, _ = store.PutParameter(model.PutParameterRequest{Name: "/app/db/password", Type: "SecureString", Value: "p1"})
	_, _ = store.PutParameter(model.PutParameterRequest{Name: "/other/name", Type: "String", Value: "o1"})

	nonRecursive, err := store.GetParametersByPath("/app", false, false)
	if err != nil {
		t.Fatalf("unexpected non-recursive error: %v", err)
	}
	if len(nonRecursive) != 1 {
		t.Fatalf("expected 1 non-recursive parameter, got %d", len(nonRecursive))
	}
	if nonRecursive[0].Name != "/app/url" {
		t.Fatalf("expected /app/url, got %s", nonRecursive[0].Name)
	}

	recursive, err := store.GetParametersByPath("/app", true, true)
	if err != nil {
		t.Fatalf("unexpected recursive error: %v", err)
	}
	if len(recursive) != 2 {
		t.Fatalf("expected 2 recursive parameters, got %d", len(recursive))
	}

	foundSecret := false
	for _, p := range recursive {
		if p.Name == "/app/db/password" {
			foundSecret = true
			if p.Value != "p1" {
				t.Fatalf("expected decrypted secure string value, got %q", p.Value)
			}
		}
	}
	if !foundSecret {
		t.Fatalf("expected /app/db/password in recursive results")
	}
}

func TestDescribeAndListSecrets(t *testing.T) {
	store := NewStore("us-east-1", "000000000000")

	v1 := "alpha"
	first, err := store.CreateSecret(model.CreateSecretRequest{Name: "svc/one", SecretString: &v1})
	if err != nil {
		t.Fatalf("create secret one failed: %v", err)
	}

	v2 := "beta"
	_, err = store.CreateSecret(model.CreateSecretRequest{Name: "svc/two", SecretString: &v2})
	if err != nil {
		t.Fatalf("create secret two failed: %v", err)
	}

	described, err := store.DescribeSecret(first.ARN)
	if err != nil {
		t.Fatalf("describe secret failed: %v", err)
	}
	if described.Name != "svc/one" {
		t.Fatalf("expected described secret name svc/one, got %s", described.Name)
	}
	if len(described.VersionIDsToStages) == 0 {
		t.Fatalf("expected version stage mappings in describe response")
	}

	listed := store.ListSecrets()
	if len(listed.SecretList) != 2 {
		t.Fatalf("expected 2 secrets in list, got %d", len(listed.SecretList))
	}
	if listed.SecretList[0].Name != "svc/one" {
		t.Fatalf("expected sorted list with svc/one first, got %s", listed.SecretList[0].Name)
	}
}

func TestDeleteParameters(t *testing.T) {
	store := NewStore("us-east-1", "000000000000")

	_, _ = store.PutParameter(model.PutParameterRequest{Name: "/app/one", Type: "String", Value: "1"})
	_, _ = store.PutParameter(model.PutParameterRequest{Name: "/app/two", Type: "String", Value: "2"})

	if err := store.DeleteParameter("/app/one"); err != nil {
		t.Fatalf("delete parameter failed: %v", err)
	}
	if _, err := store.GetParameter("/app/one", false); err == nil {
		t.Fatalf("expected deleted parameter to be missing")
	}

	deleted, invalid := store.DeleteParameters([]string{"/app/two", "/app/missing"})
	if len(deleted) != 1 || deleted[0] != "/app/two" {
		t.Fatalf("unexpected deleted list: %#v", deleted)
	}
	if len(invalid) != 1 || invalid[0] != "/app/missing" {
		t.Fatalf("unexpected invalid list: %#v", invalid)
	}
}

func TestDeleteAndRestoreSecret(t *testing.T) {
	store := NewStore("us-east-1", "000000000000")

	secretValue := "secret"
	created, err := store.CreateSecret(model.CreateSecretRequest{Name: "svc/delete-me", SecretString: &secretValue})
	if err != nil {
		t.Fatalf("create secret failed: %v", err)
	}

	deleted, err := store.DeleteSecret(model.DeleteSecretRequest{SecretID: created.ARN})
	if err != nil {
		t.Fatalf("delete secret failed: %v", err)
	}
	if deleted.Name != "svc/delete-me" {
		t.Fatalf("unexpected deleted secret name: %s", deleted.Name)
	}

	if _, err := store.GetSecretValue(model.GetSecretValueRequest{SecretID: created.ARN}); err == nil {
		t.Fatalf("expected get secret value to fail for deleted secret")
	}

	restored, err := store.RestoreSecret("svc/delete-me")
	if err != nil {
		t.Fatalf("restore secret failed: %v", err)
	}
	if restored.ARN == "" {
		t.Fatalf("expected restored ARN")
	}

	if _, err := store.GetSecretValue(model.GetSecretValueRequest{SecretID: created.ARN}); err != nil {
		t.Fatalf("expected get secret value to succeed after restore: %v", err)
	}

	summary := store.Summary()
	if summary.SecretsDeleted != 0 {
		t.Fatalf("expected 0 deleted secrets, got %d", summary.SecretsDeleted)
	}
}
