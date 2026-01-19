package main

import (
	"math/rand"
	"strings"
	"testing"
	"time"
)

func TestRandomDurationWithinRange(t *testing.T) {
	oldRng := delayRNG
	delayRNG = rand.New(rand.NewSource(1))
	defer func() { delayRNG = oldRng }()
	delay, err := randomDuration(DelayRange{Min: 0.1, Max: 0.3}, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if delay < 100*time.Millisecond || delay > 300*time.Millisecond {
		t.Fatalf("expected delay between 100ms and 300ms, got %v", delay)
	}
}

func TestRandomDurationDefaultsMaxToMin(t *testing.T) {
	oldRng := delayRNG
	delayRNG = rand.New(rand.NewSource(2))
	defer func() { delayRNG = oldRng }()
	expected := 1500 * time.Millisecond
	delay, err := randomDuration(DelayRange{Min: 1.5}, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if delay != expected {
		t.Fatalf("expected delay %v, got %v", expected, delay)
	}
}

func TestCapDelayForDeadlineZeroDeadline(t *testing.T) {
	delay := 1500 * time.Millisecond
	if capped := capDelayForDeadline(delay, time.Time{}); capped != delay {
		t.Fatalf("expected delay to remain %v, got %v", delay, capped)
	}
}

func TestCapDelayForDeadlineElapsed(t *testing.T) {
	delay := 2 * time.Second
	deadline := time.Now().Add(-time.Second)
	if capped := capDelayForDeadline(delay, deadline); capped != 0 {
		t.Fatalf("expected delay to be capped to 0, got %v", capped)
	}
}

func TestCapDelayForDeadlineShorterRemaining(t *testing.T) {
	delay := 2 * time.Second
	deadline := time.Now().Add(200 * time.Millisecond)
	capped := capDelayForDeadline(delay, deadline)
	if capped <= 0 || capped > 200*time.Millisecond {
		t.Fatalf("expected capped delay between 0 and 200ms, got %v", capped)
	}
}

func TestValidateConfigurationRejectsLegacyDelayAndHumanDelay(t *testing.T) {
	cfg := Configuration{
		Host:        "host",
		Port:        3270,
		LegacyDelay: 1,
		Steps:       []Step{{Type: "Connect"}},
	}
	if err := validateConfiguration(&cfg); err == nil || !strings.Contains(err.Error(), "Delay is no longer supported") {
		t.Fatalf("expected legacy Delay validation error, got %v", err)
	}

	cfg.LegacyDelay = 0
	cfg.Steps = []Step{{Type: "HumanDelay"}}
	if err := validateConfiguration(&cfg); err == nil || !strings.Contains(err.Error(), "HumanDelay is no longer supported") {
		t.Fatalf("expected HumanDelay validation error, got %v", err)
	}
}

func TestInjectDynamicValues(t *testing.T) {
	config := &Configuration{
		Host: "localhost",
		Port: 3270,
		Steps: []Step{
			{Type: "Connect"},
			{Type: "FillString", Text: "{{username}}"},
			{Type: "FillString", Text: "{{password}}"},
			{Type: "Disconnect"},
		},
	}

	injection := map[string]string{
		"{{username}}": "testuser",
		"{{password}}": "testpass",
	}

	result := injectDynamicValues(config, injection)

	// Verify placeholders were replaced
	if result.Steps[1].Text != "testuser" {
		t.Errorf("expected username to be 'testuser', got '%s'", result.Steps[1].Text)
	}
	if result.Steps[2].Text != "testpass" {
		t.Errorf("expected password to be 'testpass', got '%s'", result.Steps[2].Text)
	}

	// Verify original config was not modified
	if config.Steps[1].Text != "{{username}}" {
		t.Errorf("original config should not be modified")
	}
}

func TestInjectDynamicValuesPartialMatch(t *testing.T) {
	config := &Configuration{
		Host: "localhost",
		Port: 3270,
		Steps: []Step{
			{Type: "FillString", Text: "User: {{username}}, Pass: {{password}}"},
		},
	}

	injection := map[string]string{
		"{{username}}": "admin",
		"{{password}}": "secret",
	}

	result := injectDynamicValues(config, injection)

	expected := "User: admin, Pass: secret"
	if result.Steps[0].Text != expected {
		t.Errorf("expected '%s', got '%s'", expected, result.Steps[0].Text)
	}
}
