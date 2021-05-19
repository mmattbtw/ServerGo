package query_resolvers

import (
	"context"

	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
)

type auditResolver struct {
	ctx context.Context
	v   *datastructure.AuditLog

	fields map[string]*SelectedField
}

func GenerateAuditResolver(ctx context.Context, audit *datastructure.AuditLog, fields map[string]*SelectedField) (*auditResolver, error) {
	return &auditResolver{
		ctx:    ctx,
		v:      audit,
		fields: fields,
	}, nil
}

func (r *auditResolver) ID() string {
	return r.v.ID.Hex()
}

func (r *auditResolver) Type() int32 {
	return r.v.Type
}

func (r *auditResolver) Target() auditTarget {
	return auditTarget{}
}

func (r *auditResolver) Changes() []auditChange {
	return []auditChange{}
}

func (r *auditResolver) Reason() *string {
	return r.v.Reason
}

func (r *auditResolver) CreatedBy() string {
	return r.v.CreatedBy.Hex()
}

type auditChange struct {
	Key    string `json:"key"`
	Values []string
}

type auditTarget struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}
