package main

import "testing"

func TestValidateInput(t *testing.T) {
	name := "Somchai"
	service := "video_call"
	status := "new"
	createdAt := "2026-06-30T08:12:00.000Z"

	if err := validateInput(taskInput{CustomerName: &name, ServiceType: &service, Status: &status, CreatedAt: &createdAt}, true); err != nil {
		t.Fatalf("expected valid input, got %v", err)
	}
}

func TestValidateInputRejectsUnknownValues(t *testing.T) {
	service := "email"
	if err := validateInput(taskInput{ServiceType: &service}, false); err == nil {
		t.Fatal("expected invalid service type error")
	}

	status := "cancelled"
	if err := validateInput(taskInput{Status: &status}, false); err == nil {
		t.Fatal("expected invalid status error")
	}
}

func TestValidateInputAllowsPending(t *testing.T) {
	status := "pending"
	if err := validateInput(taskInput{Status: &status}, false); err != nil {
		t.Fatalf("expected pending status to be valid, got %v", err)
	}
}
