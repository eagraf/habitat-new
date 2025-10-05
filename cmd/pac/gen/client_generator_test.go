package gen

import (
	"strings"
	"testing"
)

func TestClientGenerator_GenerateClient(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantErr  bool
		contains []string
	}{
		{
			name: "simple note record",
			input: `{
				"lexicon": 1,
				"id": "com.example.note",
				"description": "A simple note",
				"defs": {
					"main": {
						"type": "record",
						"key": "com.example.note",
						"record": {
							"type": "object",
							"properties": {
								"text": {
									"type": "string",
									"description": "The note text"
								},
								"createdAt": {
									"type": "string",
									"format": "datetime"
								}
							},
							"required": ["text"]
						}
					}
				}
			}`,
			wantErr: false,
			contains: []string{
				// Check imports
				"import type { ComAtprotoRepoCreateRecord, ComAtprotoRepoGetRecord, ComAtprotoRepoListRecords } from '@atproto/api'",
				"import { HabitatClient, getUserDid, getDefaultAgent } from '../sdk/atproto'",
				"import type { PutRecordResponse, GetRecordResponse, ListRecordsResponse } from '../sdk/atproto'",
				"import type { Note } from '../types/note_types'",

				// Check public operations
				"export const createNoteRecord = async (record: Note)",
				"client.createRecord<Note>({",
				"export const listNotes = async ()",
				"client.listRecords<Note>({",
				"export const getNoteRecord = async (rkey: string)",
				"client.getRecord<Note>({",

				// Check private operations
				"export const putPrivateNoteRecord = async (record: Note, rkey?: string)",
				"client.putPrivateRecord<Note>({",
				"export const getPrivateNoteRecord = async (rkey: string)",
				"client.getPrivateRecord<Note>({",
				"export const listPrivateNotes = async ()",
				"client.listPrivateRecords<Note>({",

				// Check return types with generics
				"Promise<GetRecordResponse<Note>>",
				"Promise<ListRecordsResponse<Note>>",

				// Check collection is set
				"collection: 'com.example.note'",
			},
		},
		{
			name: "event record with multiple fields",
			input: `{
				"lexicon": 1,
				"id": "com.calendar.event",
				"description": "A calendar event",
				"defs": {
					"main": {
						"type": "record",
						"key": "com.calendar.event",
						"record": {
							"type": "object",
							"properties": {
								"title": {
									"type": "string"
								},
								"description": {
									"type": "string"
								},
								"startTime": {
									"type": "string",
									"format": "datetime"
								},
								"endTime": {
									"type": "string",
									"format": "datetime"
								},
								"location": {
									"type": "string"
								}
							},
							"required": ["title", "startTime"]
						}
					}
				}
			}`,
			wantErr: false,
			contains: []string{
				"import type { Event } from '../types/event_types'",
				"export const createEventRecord = async (record: Event)",
				"export const listEvents = async ()",
				"export const getEventRecord = async (rkey: string)",
				"export const putPrivateEventRecord = async (record: Event, rkey?: string)",
				"export const getPrivateEventRecord = async (rkey: string)",
				"export const listPrivateEvents = async ()",
				"client.createRecord<Event>({",
				"client.listRecords<Event>({",
				"client.getRecord<Event>({",
				"client.putPrivateRecord<Event>({",
				"client.getPrivateRecord<Event>({",
				"client.listPrivateRecords<Event>({",
			},
		},
		{
			name: "pluralization - simple",
			input: `{
				"lexicon": 1,
				"id": "com.example.post",
				"defs": {
					"main": {
						"type": "record",
						"key": "com.example.post",
						"record": {
							"type": "object",
							"properties": {
								"text": {"type": "string"}
							}
						}
					}
				}
			}`,
			wantErr: false,
			contains: []string{
				"export const listPosts = async ()",
				"export const listPrivatePosts = async ()",
			},
		},
		{
			name: "pluralization - ends with y",
			input: `{
				"lexicon": 1,
				"id": "com.example.story",
				"defs": {
					"main": {
						"type": "record",
						"key": "com.example.story",
						"record": {
							"type": "object",
							"properties": {
								"text": {"type": "string"}
							}
						}
					}
				}
			}`,
			wantErr: false,
			contains: []string{
				"export const listStories = async ()",
				"export const listPrivateStories = async ()",
			},
		},
		{
			name: "no console.log in generated code",
			input: `{
				"lexicon": 1,
				"id": "com.example.test",
				"defs": {
					"main": {
						"type": "record",
						"key": "com.example.test",
						"record": {
							"type": "object",
							"properties": {
								"text": {"type": "string"}
							}
						}
					}
				}
			}`,
			wantErr: false,
			contains: []string{
				"export const listPrivateTests = async ()",
			},
		},
		{
			name: "entity name extraction from lexicon ID",
			input: `{
				"lexicon": 1,
				"id": "app.bsky.feed.post",
				"defs": {
					"main": {
						"type": "record",
						"key": "app.bsky.feed.post",
						"record": {
							"type": "object",
							"properties": {
								"text": {"type": "string"}
							}
						}
					}
				}
			}`,
			wantErr: false,
			contains: []string{
				"import type { Post } from '../types/post_types'",
				"export const createPostRecord = async (record: Post)",
			},
		},
		{
			name: "missing record definition",
			input: `{
				"lexicon": 1,
				"id": "com.example.query",
				"defs": {
					"main": {
						"type": "query",
						"parameters": {
							"type": "params",
							"properties": {}
						}
					}
				}
			}`,
			wantErr:  true,
			contains: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator := NewClientGenerator()
			reader := strings.NewReader(tt.input)

			result, err := generator.GenerateClient(reader)

			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			resultStr := string(result)

			// Check that all expected strings are present
			for _, expected := range tt.contains {
				if !strings.Contains(resultStr, expected) {
					t.Errorf("GenerateClient() result missing expected string:\n%q\n\nFull output:\n%s", expected, resultStr)
				}
			}

			// Additional check: ensure no console.log is present
			if strings.Contains(resultStr, "console.log") {
				t.Error("GenerateClient() result should not contain console.log statements")
			}
		})
	}
}

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
