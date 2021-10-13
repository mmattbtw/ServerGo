package query_resolvers

import (
	"context"
	"time"

	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/server/api/v2/gql/resolvers"
	"github.com/SevenTV/ServerGo/src/utils"
	"github.com/hashicorp/go-multierror"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
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

func (r *auditResolver) Timestamp() string {
	return r.v.ID.Timestamp().Format(time.RFC3339)
}

func (r *auditResolver) ActionUserID() string {
	return r.v.CreatedBy.Hex()
}

func (r *auditResolver) ActionUser() (*UserResolver, error) {
	resolver, err := GenerateUserResolver(r.ctx, nil, &r.v.CreatedBy, r.fields)
	if err != nil {
		return nil, err
	}

	return resolver, nil
}

func (r *auditResolver) Target() (*auditTarget, error) {
	target := &auditTarget{
		ID:   r.v.Target.ID.Hex(),
		Type: r.v.Target.Type,
	}
	if data, err := resolveTarget(r.ctx, r.v.Target); err != nil {
		return nil, err
	} else {
		target.Data = data
	}

	return target, nil
}

func (r *auditResolver) Changes() []*auditChange {
	changes := make([]*auditChange, len(r.v.Changes))
	for i, c := range r.v.Changes {
		// Handle legacy Malformatted logs
		if skip := shouldSkipLegacyChangeStructure(r.v.Type, c); skip {
			c.OldValue = nil
			c.NewValue = nil
		}

		old, err1 := json.MarshalToString(c.OldValue)
		new, err2 := json.MarshalToString(c.NewValue)
		if err1 != nil || err2 != nil {
			logrus.WithError(multierror.Append(err1, err2)).Error("AuditLogResolver")
			continue
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

func resolveTarget(ctx context.Context, t *datastructure.Target) (string, error) {
	var targetUser auditTargetUser
	var targetEmote auditTargetEmote

	cur := mongo.Database.Collection(t.Type).FindOne(ctx, bson.M{
		"_id": t.ID,
	})
	err := cur.Err()
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return "", nil
		} else {
			logrus.WithError(err).Error("mongo")
			return "", resolvers.ErrInternalServer
		}
	}

	var decodeError error
	var s string
	switch t.Type {
	case "users":
		decodeError = cur.Decode(&targetUser)
		if decodeError == nil {
			s, decodeError = json.MarshalToString(&targetUser)
		}
	case "emotes":
		decodeError = cur.Decode(&targetEmote)
		if decodeError == nil {
			s, decodeError = json.MarshalToString(&targetEmote)
		}
	}
	if decodeError != nil {
		return "", decodeError
	}

	return s, nil
}

type auditChange struct {
	Key    string `json:"key"`
	Values []string
}

type auditTarget struct {
	ID   string `json:"id"`
	Data string `json:"data"`
	Type string `json:"type"`
}

type auditTargetUser struct {
	ID          primitive.ObjectID `bson:"_id" json:"id"`
	Login       string             `bson:"login" json:"login"`
	DisplayName string             `bson:"display_name" json:"display_name"`
}

type auditTargetEmote struct {
	ID   primitive.ObjectID `bson:"_id" id:"id"`
	Name string             `bson:"name" json:"name"`
}
