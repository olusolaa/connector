CREATE TABLE IF NOT EXISTS connectors (
                                          id UUID PRIMARY KEY,
                                          workspace_id TEXT NOT NULL,
                                          tenant_id TEXT NOT NULL,
                                          default_channel_id TEXT NOT NULL,
                                          created_at TIMESTAMPTZ NOT NULL,
                                          updated_at TIMESTAMPTZ NOT NULL,
                                          secret_version TEXT NOT NULL
);
