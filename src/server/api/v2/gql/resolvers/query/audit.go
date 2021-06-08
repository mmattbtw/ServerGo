package query_resolvers

import (
	"context"

	"github.com/SevenTV/ServerGo/src/cache"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/utils"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
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

func (r *auditResolver) Target() (*auditTarget, error) {
	return &auditTarget{
		ID:   r.v.Target.ID.Hex(),
		Type: r.v.Target.Type,
	}, nil
}

func (r *auditResolver) Changes() []*auditChange {
	changes := make([]*auditChange, len(r.v.Changes))
	for i, c := range r.v.Changes {
		// Handle legacy Malformatted logs
		if skip := shouldSkipLegacyChangeStructure(r.v.Type, c); skip == true {
			c.OldValue = nil
			c.NewValue = nil
		}

		old, err1 := json.MarshalToString(c.OldValue)
		new, err2 := json.MarshalToString(c.NewValue)
		if err1 != nil || err2 != nil {
			log.Errorf("AuditLogResolver, err1=%v, err2=%v", err1, err2)
		}

		changes[i] = &auditChange{
			Key: c.Key,
			Values: []string{
				utils.Ternary(c.OldValue != nil, old, "").(string),
				utils.Ternary(c.NewValue != nil, new, "").(string),
			},
		}
	}

	return changes
}

func (r *auditResolver) Reason() *string {
	return r.v.Reason
}

func (r *auditResolver) CreatedBy() string {
	return r.v.CreatedBy.Hex()
}

func shouldSkipLegacyChangeStructure(t int32, c *datastructure.AuditLogChange) bool {
	if t == datastructure.AuditLogTypeUserChannelEmoteAdd && c.OldValue != nil && utils.IsSliceArray(c.OldValue) {
		return true
	}
	if t == datastructure.AuditLogTypeUserChannelEmoteRemove && c.OldValue != nil && utils.IsSliceArray(c.OldValue) {
		return true
	}
	if t == datastructure.AuditLogTypeUserChannelEditorAdd && c.OldValue != nil && utils.IsSliceArray(c.OldValue) {
		return true
	}
	if t == datastructure.AuditLogTypeUserChannelEditorRemove && c.OldValue != nil && utils.IsSliceArray(c.OldValue) {
		return true
	}

	return false
}

func resolveTarget(ctx context.Context, t *auditTarget) (string, error) {
	var result interface{}
	if err := cache.FindOne(ctx, t.Type, "", bson.M{
		"_id": t.ID,
	}, &result); err != nil {
		return "", err
	}

	s, err := json.MarshalToString(result)
	if err != nil {
		return "", err
	}

	return s, nil
}

type auditChange struct {
	Key    string `json:"key"`
	Values []string
}

type auditTarget struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}
