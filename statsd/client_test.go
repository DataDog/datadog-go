package statsd

import (
	"os"
	"testing"
)

func TestEntityID(t *testing.T) {
	initialValue, initiallySet := os.LookupEnv(entityIDEnvName)
	if initiallySet {
		defer os.Setenv(entityIDEnvName, initialValue)
	} else {
		defer os.Unsetenv(entityIDEnvName)
	}

	// Set to a valid value
	os.Setenv(entityIDEnvName, "testing")
	client, err := New("localhost:8125")
	if err != nil {
		t.Fatal(err)
	}
	if len(client.tags) != 1 {
		t.Errorf("Expecting one tag, got %d", len(client.tags))
	}
	if client.tags[0] != "dd.internal.entity_id:testing" {
		t.Errorf("Bad tag value, got %s", client.tags[0])
	}

	// Set to empty string
	os.Setenv(entityIDEnvName, "")
	client, err = New("localhost:8125")
	if err != nil {
		t.Fatal(err)
	}
	if len(client.tags) != 0 {
		t.Errorf("Expecting empty default tags, got %v", client.tags)
	}

	// Unset
	os.Unsetenv(entityIDEnvName)
	client, err = New("localhost:8125")
	if err != nil {
		t.Fatal(err)
	}
	if len(client.tags) != 0 {
		t.Errorf("Expecting empty default tags, got %v", client.tags)
	}
}
