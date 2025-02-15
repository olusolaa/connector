package postgres

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"time"

	"github.com/connector-recruitment/internal/domain"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type ConnectorRepository struct {
	db *sqlx.DB
}

func NewConnectorRepository(db *sqlx.DB) domain.ConnectorRepository {
	return &ConnectorRepository{db: db}
}

func (r *ConnectorRepository) Create(ctx context.Context, c *domain.Connector) error {
	query := `
        INSERT INTO connectors 
        (id, workspace_id, tenant_id, default_channel_id, created_at, updated_at, secret_version)
        VALUES (:id, :workspace_id, :tenant_id, :default_channel_id, :created_at, :updated_at, :secret_version)
    `
	_, err := r.db.NamedExecContext(ctx, query, c)
	return err
}

func (r *ConnectorRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Connector, error) {
	query := `
        SELECT id, workspace_id, tenant_id, default_channel_id, created_at, updated_at, secret_version
        FROM connectors
        WHERE id = $1
    `
	var conn domain.Connector
	if err := r.db.GetContext(ctx, &conn, query, id); err != nil {
		return nil, err
	}
	return &conn, nil
}

func (r *ConnectorRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM connectors WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *ConnectorRepository) ListConnectors(ctx context.Context, limit int, cursor *domain.ListCursor) ([]domain.Connector, *domain.ListCursor, error) {
	if limit <= 0 {
		limit = 50
	}

	var args []interface{}
	var conditions []string

	if cursor != nil {
		conditions = append(conditions, "(updated_at, id) > ($1, $2)")
		args = append(args, cursor.UpdatedAt, cursor.ID)
	}

	query := `
		SELECT id, workspace_id, tenant_id, default_channel_id, created_at, updated_at, secret_version
		FROM connectors
	`
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY updated_at ASC, id ASC LIMIT $" + strconv.Itoa(len(args)+1)
	args = append(args, limit+1)

	var connectors []domain.Connector
	if err := r.db.SelectContext(ctx, &connectors, query, args...); err != nil {
		return nil, nil, err
	}

	var nextCursor *domain.ListCursor
	if len(connectors) > limit {
		last := connectors[limit-1]
		nextCursor = &domain.ListCursor{
			UpdatedAt: last.UpdatedAt,
			ID:        last.ID,
		}
		connectors = connectors[:limit]
	}

	return connectors, nextCursor, nil
}

func (r *ConnectorRepository) UpdateConnector(ctx context.Context, id uuid.UUID, token string) error {
	query := `
		UPDATE connectors
		SET secret_version = $1, updated_at = $2
		WHERE id = $3
	`
	result, err := r.db.ExecContext(ctx, query, token, time.Now().UTC(), id)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}
	return nil
}
