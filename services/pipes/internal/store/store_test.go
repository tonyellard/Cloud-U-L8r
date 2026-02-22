// SPDX-License-Identifier: Apache-2.0
package store

import (
	"strings"
	"testing"

	"github.com/tonyellard/pipes/internal/model"
)

func newTestStore() *Store {
	return New("us-east-1", "000000000000")
}

func createTestPipe(t *testing.T, s *Store, name string) *model.Pipe {
	t.Helper()
	p, err := s.CreatePipe(name, model.CreatePipeRequest{
		Source:  "arn:aws:sqs:us-east-1:000000000000:source-queue",
		Target:  "arn:aws:sqs:us-east-1:000000000000:target-queue",
		RoleArn: "arn:aws:iam::000000000000:role/pipe-role",
	})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCreatePipe(t *testing.T) {
	s := newTestStore()
	p := createTestPipe(t, s, "test-pipe")

	if p.Name != "test-pipe" {
		t.Errorf("expected name test-pipe, got %s", p.Name)
	}
	if p.CurrentState != "RUNNING" {
		t.Errorf("expected CurrentState RUNNING, got %s", p.CurrentState)
	}
	if !strings.Contains(p.Arn, "pipe/test-pipe") {
		t.Errorf("expected ARN to contain pipe/test-pipe, got %s", p.Arn)
	}
}

func TestCreatePipeConflict(t *testing.T) {
	s := newTestStore()
	createTestPipe(t, s, "dupe")

	_, err := s.CreatePipe("dupe", model.CreatePipeRequest{
		Source:  "arn:aws:sqs:us-east-1:000000000000:q",
		Target:  "arn:aws:sqs:us-east-1:000000000000:q2",
		RoleArn: "arn:aws:iam::000000000000:role/r",
	})
	if err == nil {
		t.Fatal("expected ConflictException")
	}
	if !strings.Contains(err.Error(), "ConflictException") {
		t.Errorf("expected ConflictException, got %s", err)
	}
}

func TestCreatePipeDesiredStateStopped(t *testing.T) {
	s := newTestStore()
	p, err := s.CreatePipe("stopped-pipe", model.CreatePipeRequest{
		Source:       "arn:aws:sqs:us-east-1:000000000000:q",
		Target:       "arn:aws:sqs:us-east-1:000000000000:q2",
		RoleArn:      "arn:aws:iam::000000000000:role/r",
		DesiredState: "STOPPED",
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.CurrentState != "STOPPED" {
		t.Errorf("expected STOPPED, got %s", p.CurrentState)
	}
}

func TestDescribePipe(t *testing.T) {
	s := newTestStore()
	createTestPipe(t, s, "my-pipe")

	p, err := s.DescribePipe("my-pipe")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "my-pipe" {
		t.Errorf("expected my-pipe, got %s", p.Name)
	}
}

func TestDescribePipeNotFound(t *testing.T) {
	s := newTestStore()
	_, err := s.DescribePipe("nope")
	if err == nil {
		t.Fatal("expected NotFoundException")
	}
	if !strings.Contains(err.Error(), "NotFoundException") {
		t.Errorf("expected NotFoundException, got %s", err)
	}
}

func TestUpdatePipe(t *testing.T) {
	s := newTestStore()
	createTestPipe(t, s, "upd-pipe")

	p, err := s.UpdatePipe("upd-pipe", model.UpdatePipeRequest{
		Description:  "updated",
		DesiredState: "STOPPED",
		RoleArn:      "arn:aws:iam::000000000000:role/new-role",
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.Description != "updated" {
		t.Errorf("expected updated description")
	}
	if p.CurrentState != "STOPPED" {
		t.Errorf("expected STOPPED, got %s", p.CurrentState)
	}
}

func TestDeletePipe(t *testing.T) {
	s := newTestStore()
	createTestPipe(t, s, "del-pipe")

	p, err := s.DeletePipe("del-pipe")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "del-pipe" {
		t.Errorf("expected del-pipe, got %s", p.Name)
	}

	_, err = s.DescribePipe("del-pipe")
	if err == nil {
		t.Fatal("expected NotFoundException after delete")
	}
}

func TestListPipes(t *testing.T) {
	s := newTestStore()
	createTestPipe(t, s, "alpha")
	createTestPipe(t, s, "beta")

	list := s.ListPipes("", "", "", "", "", 0)
	if len(list) != 2 {
		t.Fatalf("expected 2 pipes, got %d", len(list))
	}
	if list[0].Name != "alpha" {
		t.Errorf("expected alpha first, got %s", list[0].Name)
	}
}

func TestListPipesWithNamePrefix(t *testing.T) {
	s := newTestStore()
	createTestPipe(t, s, "prod-pipe1")
	createTestPipe(t, s, "dev-pipe1")

	list := s.ListPipes("prod-", "", "", "", "", 0)
	if len(list) != 1 {
		t.Fatalf("expected 1 pipe, got %d", len(list))
	}
	if list[0].Name != "prod-pipe1" {
		t.Errorf("expected prod-pipe1, got %s", list[0].Name)
	}
}

func TestStartStopPipe(t *testing.T) {
	s := newTestStore()
	createTestPipe(t, s, "toggle-pipe")

	p, err := s.StopPipe("toggle-pipe")
	if err != nil {
		t.Fatal(err)
	}
	if p.CurrentState != "STOPPED" {
		t.Errorf("expected STOPPED, got %s", p.CurrentState)
	}

	p, err = s.StartPipe("toggle-pipe")
	if err != nil {
		t.Fatal(err)
	}
	if p.CurrentState != "RUNNING" {
		t.Errorf("expected RUNNING, got %s", p.CurrentState)
	}
}

func TestRunningPipes(t *testing.T) {
	s := newTestStore()
	createTestPipe(t, s, "running1")
	createTestPipe(t, s, "stopped1")
	s.StopPipe("stopped1")

	running := s.RunningPipes()
	if len(running) != 1 {
		t.Fatalf("expected 1 running pipe, got %d", len(running))
	}
	if running[0].Name != "running1" {
		t.Errorf("expected running1, got %s", running[0].Name)
	}
}

func TestTagOperations(t *testing.T) {
	s := newTestStore()
	p := createTestPipe(t, s, "tagged-pipe")

	err := s.TagResource(p.Arn, map[string]string{"env": "test", "team": "backend"})
	if err != nil {
		t.Fatal(err)
	}

	tags, err := s.ListTagsForResource(p.Arn)
	if err != nil {
		t.Fatal(err)
	}
	if tags["env"] != "test" {
		t.Errorf("expected tag env=test")
	}

	err = s.UntagResource(p.Arn, []string{"team"})
	if err != nil {
		t.Fatal(err)
	}

	tags, err = s.ListTagsForResource(p.Arn)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := tags["team"]; ok {
		t.Error("expected team tag to be removed")
	}
}

func TestSummary(t *testing.T) {
	s := newTestStore()
	createTestPipe(t, s, "p1")
	createTestPipe(t, s, "p2")
	s.StopPipe("p2")

	summary := s.Summary()
	if summary.Pipes != 2 {
		t.Errorf("expected 2 pipes, got %d", summary.Pipes)
	}
	if summary.RunningPipes != 1 {
		t.Errorf("expected 1 running, got %d", summary.RunningPipes)
	}
}
