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
	delay := randomDuration(DelayRange{Min: 0.1, Max: 0.3})
	if delay < 100*time.Millisecond || delay > 300*time.Millisecond {
		t.Fatalf("expected delay between 100ms and 300ms, got %v", delay)
	}
}

func TestRandomDurationDefaultsMaxToMin(t *testing.T) {
	oldRng := delayRNG
	delayRNG = rand.New(rand.NewSource(2))
	defer func() { delayRNG = oldRng }()
	expected := time.Duration(1500 * time.Millisecond)
	delay := randomDuration(DelayRange{Min: 1.5})
	if delay != expected {
		t.Fatalf("expected delay %v, got %v", expected, delay)
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
