package types

type CreateDatabaseRequest struct {
	Name       string                 `json:"name"`
	SchemaType string                 `json:"schema_type"`
	InitState  map[string]interface{} `json:"init_state"`
}

type CreateDatabaseResponse struct {
	DatabaseID string `json:"database_id"`
}

type QueryDatabaseRequest struct {
	DatabaseID   string `json:"id"`
	DatabaseName string `json:"name"`
}

type QueryDatabaseResponse struct {
}

type ProposeTransitionsRequest struct {
	DatabaseID string `json:"database_id"`
}

type ProposeTransitionsResponse struct {
}

type GetDatabaseResponse struct {
	DatabaseID string                   `json:"database_id"`
	State      map[string]interface{}   `json:"state"`
	Dex        []map[string]interface{} `json:"dex,omitempty"`
}
