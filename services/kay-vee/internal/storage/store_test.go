package storage

import (
	"testing"
	"time"

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

	nonRecursive, _, err := store.GetParametersByPath("/app", false, false, 0, "")
	if err != nil {
		t.Fatalf("unexpected non-recursive error: %v", err)
	}
	if len(nonRecursive) != 1 {
		t.Fatalf("expected 1 non-recursive parameter, got %d", len(nonRecursive))
	}
	if nonRecursive[0].Name != "/app/url" {
		t.Fatalf("expected /app/url, got %s", nonRecursive[0].Name)
	}

	recursive, _, err := store.GetParametersByPath("/app", true, true, 0, "")
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

	listed, err := store.ListSecrets(0, "")
	if err != nil {
		t.Fatalf("list secrets failed: %v", err)
	}
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

func TestLabelParameterVersion(t *testing.T) {
	store := NewStore("us-east-1", "000000000000")

	_, _ = store.PutParameter(model.PutParameterRequest{Name: "/svc/labeled", Type: "String", Value: "v1"})
	_, _ = store.PutParameter(model.PutParameterRequest{Name: "/svc/labeled", Type: "String", Value: "v2", Overwrite: true})

	_, err := store.LabelParameterVersion(model.LabelParameterVersionRequest{
		Name:             "/svc/labeled",
		Labels:           []string{"stable"},
		ParameterVersion: 1,
	})
	if err != nil {
		t.Fatalf("label parameter version failed: %v", err)
	}

	param, err := store.GetParameter("/svc/labeled:stable", false)
	if err != nil {
		t.Fatalf("get by label failed: %v", err)
	}
	if param.Value != "v1" {
		t.Fatalf("expected labeled value v1, got %q", param.Value)
	}
}

func TestUpdateSecretVersionStage(t *testing.T) {
	store := NewStore("us-east-1", "000000000000")

	v1 := "one"
	created, err := store.CreateSecret(model.CreateSecretRequest{Name: "svc/stage", SecretString: &v1})
	if err != nil {
		t.Fatalf("create secret failed: %v", err)
	}

	v2 := "two"
	updated, err := store.PutSecretValue(model.PutSecretValueRequest{SecretID: "svc/stage", SecretString: &v2})
	if err != nil {
		t.Fatalf("put secret value failed: %v", err)
	}

	_, err = store.UpdateSecretVersionStage(model.UpdateSecretVersionStageRequest{
		SecretID:            "svc/stage",
		VersionStage:        "AWSCURRENT",
		MoveToVersionID:     created.VersionID,
		RemoveFromVersionID: updated.VersionID,
	})
	if err != nil {
		t.Fatalf("update secret version stage failed: %v", err)
	}

	current, err := store.GetSecretValue(model.GetSecretValueRequest{SecretID: "svc/stage"})
	if err != nil {
		t.Fatalf("get current secret failed: %v", err)
	}
	if current.SecretString == nil || *current.SecretString != "one" {
		t.Fatalf("expected AWSCURRENT to point back to v1, got %#v", current.SecretString)
	}
}

func TestDescribeParameters(t *testing.T) {
	store := NewStore("us-east-1", "000000000000")

	_, _ = store.PutParameter(model.PutParameterRequest{Name: "/b/param", Type: "String", Value: "b"})
	_, _ = store.PutParameter(model.PutParameterRequest{Name: "/a/param", Type: "String", Value: "a"})

	params, _, err := store.DescribeParameters(0, "")
	if err != nil {
		t.Fatalf("describe parameters failed: %v", err)
	}
	if len(params) != 2 {
		t.Fatalf("expected 2 described params, got %d", len(params))
	}
	if params[0].Name != "/a/param" {
		t.Fatalf("expected sorted params with /a/param first, got %s", params[0].Name)
	}
}

func TestGetParameterHistory(t *testing.T) {
	store := NewStore("us-east-1", "000000000000")

	_, _ = store.PutParameter(model.PutParameterRequest{Name: "/hist/p", Type: "String", Value: "v1"})
	_, _ = store.PutParameter(model.PutParameterRequest{Name: "/hist/p", Type: "String", Value: "v2", Overwrite: true})

	history, _, err := store.GetParameterHistory("/hist/p", false, 0, "")
	if err != nil {
		t.Fatalf("get parameter history failed: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 history records, got %d", len(history))
	}
	if history[0].Version != 1 || history[0].Value != "v1" {
		t.Fatalf("unexpected first history item: %#v", history[0])
	}
	if history[1].Version != 2 || history[1].Value != "v2" {
		t.Fatalf("unexpected second history item: %#v", history[1])
	}
}

func TestPaginationAcrossSSMAndSecrets(t *testing.T) {
	store := NewStore("us-east-1", "000000000000")

	_, _ = store.PutParameter(model.PutParameterRequest{Name: "/page/a", Type: "String", Value: "a"})
	_, _ = store.PutParameter(model.PutParameterRequest{Name: "/page/b", Type: "String", Value: "b"})

	described, token, err := store.DescribeParameters(1, "")
	if err != nil {
		t.Fatalf("describe with pagination failed: %v", err)
	}
	if len(described) != 1 || token == "" {
		t.Fatalf("expected one describe item and next token, got len=%d token=%q", len(described), token)
	}

	history, historyToken, err := store.GetParameterHistory("/page/b", false, 1, "")
	if err != nil {
		t.Fatalf("history with pagination failed: %v", err)
	}
	if len(history) != 1 || historyToken != "" {
		t.Fatalf("expected one history record and no token, got len=%d token=%q", len(history), historyToken)
	}

	v1 := "one"
	_, _ = store.CreateSecret(model.CreateSecretRequest{Name: "page/one", SecretString: &v1})
	v2 := "two"
	_, _ = store.CreateSecret(model.CreateSecretRequest{Name: "page/two", SecretString: &v2})

	listed, err := store.ListSecrets(1, "")
	if err != nil {
		t.Fatalf("list secrets with pagination failed: %v", err)
	}
	listToken := listed.NextToken
	if len(listed.SecretList) != 1 || listToken == "" {
		t.Fatalf("expected one secret and token, got len=%d token=%q", len(listed.SecretList), listToken)
	}

	listed2, err := store.ListSecrets(1, listToken)
	if err != nil {
		t.Fatalf("list secrets page 2 failed: %v", err)
	}
	if len(listed2.SecretList) != 1 {
		t.Fatalf("expected one secret on second page, got %d", len(listed2.SecretList))
	}
}

func TestExportImportRoundTrip(t *testing.T) {
	store := NewStore("us-east-1", "000000000000")

	_, _ = store.PutParameter(model.PutParameterRequest{Name: "/roundtrip/p", Type: "String", Value: "v1"})
	_, _ = store.PutParameter(model.PutParameterRequest{Name: "/roundtrip/p", Type: "String", Value: "v2", Overwrite: true})
	_, _ = store.LabelParameterVersion(model.LabelParameterVersionRequest{Name: "/roundtrip/p", Labels: []string{"stable"}, ParameterVersion: 1})

	secretValue := "s1"
	_, _ = store.CreateSecret(model.CreateSecretRequest{Name: "roundtrip/secret", SecretString: &secretValue})

	exported := store.ExportState()

	newStore := NewStore("us-west-2", "111111111111")
	res := newStore.ImportState(model.AdminImportRequest(exported))
	if res.ImportedParameters != 1 || res.ImportedSecrets != 1 {
		t.Fatalf("unexpected import counts: %#v", res)
	}

	param, err := newStore.GetParameter("/roundtrip/p:stable", false)
	if err != nil {
		t.Fatalf("expected labeled parameter after import: %v", err)
	}
	if param.Value != "v1" {
		t.Fatalf("expected labeled value v1 after import, got %q", param.Value)
	}

	secret, err := newStore.GetSecretValue(model.GetSecretValueRequest{SecretID: "roundtrip/secret"})
	if err != nil {
		t.Fatalf("expected secret after import: %v", err)
	}
	if secret.SecretString == nil || *secret.SecretString != "s1" {
		t.Fatalf("unexpected secret value after import: %#v", secret.SecretString)
	}
}

func TestActivityLogPaginationAndOrdering(t *testing.T) {
	store := NewStore("us-east-1", "000000000000")

	store.RecordActivity(model.AdminActivityEntry{Method: "POST", Path: "/", Target: "AmazonSSM.PutParameter", StatusCode: 200, Timestamp: time.Date(2026, 2, 13, 10, 0, 0, 0, time.UTC)})
	store.RecordActivity(model.AdminActivityEntry{Method: "POST", Path: "/", Target: "AmazonSSM.GetParameter", StatusCode: 404, ErrorType: "ParameterNotFound", Timestamp: time.Date(2026, 2, 13, 10, 1, 0, 0, time.UTC)})

	firstPage, token, err := store.ListActivity(1, "")
	if err != nil {
		t.Fatalf("list activity first page failed: %v", err)
	}
	if len(firstPage) != 1 {
		t.Fatalf("expected one activity entry on first page, got %d", len(firstPage))
	}
	if firstPage[0].Target != "AmazonSSM.GetParameter" {
		t.Fatalf("expected newest event first, got target %q", firstPage[0].Target)
	}
	if token == "" {
		t.Fatalf("expected next token on first page")
	}

	secondPage, token2, err := store.ListActivity(1, token)
	if err != nil {
		t.Fatalf("list activity second page failed: %v", err)
	}
	if len(secondPage) != 1 {
		t.Fatalf("expected one activity entry on second page, got %d", len(secondPage))
	}
	if secondPage[0].Target != "AmazonSSM.PutParameter" {
		t.Fatalf("expected older event on second page, got target %q", secondPage[0].Target)
	}
	if token2 != "" {
		t.Fatalf("expected no next token on final page, got %q", token2)
	}
}
