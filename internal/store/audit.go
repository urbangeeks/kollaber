package store

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgtype"
)

type AuditLog struct {
	ID         pgtype.UUID        `json:"id"`
	OrgID      pgtype.UUID        `json:"org_id"`
	ActorID    pgtype.UUID        `json:"actor_id"`
	ActorEmail string             `json:"actor_email"`
	Action     string             `json:"action"`
	TargetType string             `json:"target_type"`
	TargetID   string             `json:"target_id"`
	Metadata   []byte             `json:"metadata"`
	CreatedAt  pgtype.Timestamptz `json:"created_at"`
}

type WriteAuditLogParams struct {
	OrgID      pgtype.UUID
	ActorID    pgtype.UUID
	ActorEmail string
	Action     string
	TargetType string
	TargetID   string
	Metadata   map[string]any
}

func (q *Queries) WriteAuditLog(ctx context.Context, p WriteAuditLogParams) {
	meta, _ := json.Marshal(p.Metadata)
	if meta == nil {
		meta = []byte("{}")
	}
	_, _ = q.db.Exec(ctx, `
		INSERT INTO audit_logs (org_id, actor_id, actor_email, action, target_type, target_id, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		p.OrgID, p.ActorID, p.ActorEmail, p.Action, p.TargetType, p.TargetID, meta,
	)
}

func (q *Queries) ListAuditLogs(ctx context.Context, orgID pgtype.UUID, limit, offset int32) ([]AuditLog, error) {
	rows, err := q.db.Query(ctx, `
		SELECT id, org_id, actor_id, actor_email, action, target_type, target_id, metadata, created_at
		FROM audit_logs
		WHERE org_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`,
		orgID, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var logs []AuditLog
	for rows.Next() {
		var l AuditLog
		if err := rows.Scan(&l.ID, &l.OrgID, &l.ActorID, &l.ActorEmail, &l.Action, &l.TargetType, &l.TargetID, &l.Metadata, &l.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}
