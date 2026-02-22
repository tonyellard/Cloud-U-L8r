// SPDX-License-Identifier: Apache-2.0
package store

import (
	"testing"

	"github.com/tonyellard/scheduler/internal/model"
)

func TestNewStore_HasDefaultGroup(t *testing.T) {
	s := NewStore("us-east-1", "000000000000")
	g, err := s.GetScheduleGroup("default")
	if err != nil {
		t.Fatal(err)
	}
	if g.Name != "default" {
		t.Errorf("expected 'default', got %s", g.Name)
	}
	if g.State != "ACTIVE" {
		t.Errorf("expected ACTIVE, got %s", g.State)
	}
}

func TestCreateScheduleGroup(t *testing.T) {
	s := NewStore("us-east-1", "000000000000")
	resp, err := s.CreateScheduleGroup(model.CreateScheduleGroupRequest{Name: "test-group"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.ScheduleGroupArn == "" {
		t.Error("expected non-empty ARN")
	}

	// Duplicate should fail
	_, err = s.CreateScheduleGroup(model.CreateScheduleGroupRequest{Name: "test-group"})
	if err == nil {
		t.Error("expected conflict error")
	}
}

func TestDeleteScheduleGroup(t *testing.T) {
	s := NewStore("us-east-1", "000000000000")
	s.CreateScheduleGroup(model.CreateScheduleGroupRequest{Name: "temp"})
	if err := s.DeleteScheduleGroup("temp"); err != nil {
		t.Fatal(err)
	}

	// Can't delete default
	if err := s.DeleteScheduleGroup("default"); err == nil {
		t.Error("expected error deleting default group")
	}
}

func TestCreateSchedule(t *testing.T) {
	s := NewStore("us-east-1", "000000000000")
	resp, err := s.CreateSchedule(model.CreateScheduleRequest{
		Name:               "my-schedule",
		ScheduleExpression: "rate(5 minutes)",
		Target: &model.ScheduleTarget{
			Arn: "arn:aws:sqs:us-east-1:000000000000:my-queue",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.ScheduleArn == "" {
		t.Error("expected non-empty ARN")
	}

	// Get it back
	sched, err := s.GetSchedule("my-schedule", "default")
	if err != nil {
		t.Fatal(err)
	}
	if sched.ScheduleExpression != "rate(5 minutes)" {
		t.Errorf("expected 'rate(5 minutes)', got %s", sched.ScheduleExpression)
	}
	if sched.State != "ENABLED" {
		t.Errorf("expected ENABLED, got %s", sched.State)
	}
}

func TestUpdateSchedule(t *testing.T) {
	s := NewStore("us-east-1", "000000000000")
	s.CreateSchedule(model.CreateScheduleRequest{
		Name:               "my-schedule",
		ScheduleExpression: "rate(5 minutes)",
	})

	_, err := s.UpdateSchedule(model.UpdateScheduleRequest{
		Name:               "my-schedule",
		ScheduleExpression: "rate(1 hour)",
		State:              "DISABLED",
	})
	if err != nil {
		t.Fatal(err)
	}

	sched, _ := s.GetSchedule("my-schedule", "default")
	if sched.ScheduleExpression != "rate(1 hour)" {
		t.Errorf("expected 'rate(1 hour)', got %s", sched.ScheduleExpression)
	}
	if sched.State != "DISABLED" {
		t.Errorf("expected DISABLED, got %s", sched.State)
	}
}

func TestDeleteSchedule(t *testing.T) {
	s := NewStore("us-east-1", "000000000000")
	s.CreateSchedule(model.CreateScheduleRequest{
		Name:               "to-delete",
		ScheduleExpression: "rate(1 minute)",
	})

	if err := s.DeleteSchedule("to-delete", "default"); err != nil {
		t.Fatal(err)
	}

	_, err := s.GetSchedule("to-delete", "default")
	if err == nil {
		t.Error("expected not-found error")
	}
}

func TestListSchedules(t *testing.T) {
	s := NewStore("us-east-1", "000000000000")
	s.CreateSchedule(model.CreateScheduleRequest{
		Name:               "sched-a",
		ScheduleExpression: "rate(1 minute)",
	})
	s.CreateSchedule(model.CreateScheduleRequest{
		Name:               "sched-b",
		ScheduleExpression: "rate(5 minutes)",
		State:              "DISABLED",
	})

	resp := s.ListSchedules(model.ListSchedulesRequest{})
	if len(resp.Schedules) != 2 {
		t.Errorf("expected 2, got %d", len(resp.Schedules))
	}

	// Filter by state
	resp = s.ListSchedules(model.ListSchedulesRequest{State: "ENABLED"})
	if len(resp.Schedules) != 1 {
		t.Errorf("expected 1 enabled, got %d", len(resp.Schedules))
	}
}

func TestEnabledSchedules(t *testing.T) {
	s := NewStore("us-east-1", "000000000000")
	s.CreateSchedule(model.CreateScheduleRequest{
		Name:               "enabled-one",
		ScheduleExpression: "rate(1 minute)",
	})
	s.CreateSchedule(model.CreateScheduleRequest{
		Name:               "disabled-one",
		ScheduleExpression: "rate(1 minute)",
		State:              "DISABLED",
	})

	enabled := s.EnabledSchedules()
	if len(enabled) != 1 {
		t.Errorf("expected 1 enabled, got %d", len(enabled))
	}
	if enabled[0].Name != "enabled-one" {
		t.Errorf("expected 'enabled-one', got %s", enabled[0].Name)
	}
}

func TestSummary(t *testing.T) {
	s := NewStore("us-east-1", "000000000000")
	s.CreateSchedule(model.CreateScheduleRequest{
		Name:               "s1",
		ScheduleExpression: "rate(1 minute)",
	})
	s.CreateScheduleGroup(model.CreateScheduleGroupRequest{Name: "extra"})

	summary := s.Summary()
	if summary.ScheduleGroups != 2 {
		t.Errorf("expected 2 groups, got %d", summary.ScheduleGroups)
	}
	if summary.Schedules != 1 {
		t.Errorf("expected 1 schedule, got %d", summary.Schedules)
	}
}
