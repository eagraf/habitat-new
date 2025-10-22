package gen

import (
	"os"
	"strings"
	"testing"
)

func TestClientGenerator_Pluralization(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"post", "posts"},
		{"event", "events"},
		{"story", "stories"},
		{"entry", "entries"},
		{"box", "boxes"},
		{"class", "classes"},
		{"bus", "buses"},
		{"dish", "dishes"},
		{"note", "notes"},
		{"user", "users"},
	}

	generator := NewClientGenerator()
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := generator.pluralize(tt.input)
			if got != tt.want {
				t.Errorf("pluralize(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestClientGenerator_Singularization(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"posts", "post"},
		{"events", "event"},
		{"stories", "story"},
		{"entries", "entry"},
		{"boxes", "box"},
		{"classes", "class"},
		{"buses", "bus"},
		{"dishes", "dish"},
		{"notes", "note"},
		{"users", "user"},
	}

	generator := NewClientGenerator()
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := generator.singularize(tt.input)
			if got != tt.want {
				t.Errorf("singularize(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestClientGenerator_ToPascalCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"post", "Post"},
		{"user_profile", "UserProfile"},
		{"my-event", "MyEvent"},
		{"app.bsky.feed", "AppBskyFeed"},
		{"hello_world-test", "HelloWorldTest"},
		{"main", "Main"},
	}

	generator := NewClientGenerator()
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := generator.toPascalCase(tt.input)
			if got != tt.want {
				t.Errorf("toPascalCase(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestClientGenerator_GetEntityName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"com.example.post", "post"},
		{"app.bsky.feed.post", "post"},
		{"com.calendar.event", "event"},
		{"dev.eagraf.note", "note"},
		{"single", "single"},
	}

	generator := NewClientGenerator()
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := generator.getEntityName(tt.input)
			if got != tt.want {
				t.Errorf("getEntityName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestClientGenerator_GenerateClient_EventTest(t *testing.T) {
	// Read the input JSON file
	inputBytes, err := os.ReadFile("test_files/event_test/event.json")
	if err != nil {
		t.Fatalf("Failed to read input file: %v", err)
	}

	// Read the expected output file
	expectedBytes, err := os.ReadFile("test_files/event_test/event_client.ts")
	if err != nil {
		t.Fatalf("Failed to read expected output file: %v", err)
	}

	// Generate the client
	generator := NewClientGenerator()
	reader := strings.NewReader(string(inputBytes))
	result, err := generator.GenerateClient(reader)
	if err != nil {
		t.Fatalf("GenerateClient() failed: %v", err)
	}

	// Compare the result with the expected output
	resultStr := string(result)
	expectedStr := string(expectedBytes)

	if resultStr != expectedStr {
		t.Errorf("GenerateClient() output does not match expected.\n\nExpected:\n%s\n\nGot:\n%s", expectedStr, resultStr)
	}
}
