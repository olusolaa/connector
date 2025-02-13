package connector

type CreateInput struct {
	WorkspaceID    string
	TenantID       string
	Token          string
	DefaultChannel string
}
