package resolvers

import (
	"context"

	"github.com/SevenTV/ServerGo/mongo"
)

type reportResolver struct {
	ctx context.Context
	v   *mongo.Report

	fields map[string]*SelectedField
}

func GenerateReportResolver(ctx context.Context, report *mongo.Report, fields map[string]*SelectedField) (*reportResolver, error) {
	return &reportResolver{
		ctx:    ctx,
		v:      report,
		fields: fields,
	}, nil
}

func (r *reportResolver) ReporterID() *string {
	if r.v.ReporterID == nil {
		return nil
	}
	hex := r.v.ReporterID.Hex()
	return &hex
}

func (r *reportResolver) TargetID() *string {
	if r.v.Target.ID == nil {
		return nil
	}
	hex := r.v.Target.ID.Hex()
	return &hex
}

func (r *reportResolver) TargetType() string {
	return r.v.Target.Type
}

func (r *reportResolver) Reason() string {
	return r.v.Reason
}

func (r *reportResolver) Cleared() bool {
	return r.v.Cleared
}

func (r *reportResolver) UTarget() (*userResolver, error) {
	if r.v.Target.Type == "users" {
		return GenerateUserResolver(r.ctx, r.v.UTarget, r.v.Target.ID, r.fields["u_target"].children)
	}
	return nil, nil
}

func (r *reportResolver) ETarget() (*emoteResolver, error) {
	if r.v.Target.Type == "emotes" {
		return GenerateEmoteResolver(r.ctx, r.v.ETarget, r.v.Target.ID, r.fields["e_target"].children)
	}
	return nil, nil
}

func (r *reportResolver) Reporter() (*userResolver, error) {
	if r.v.ReporterID != nil {
		return GenerateUserResolver(r.ctx, r.v.Reporter, r.v.ReporterID, r.fields["reporter"].children)
	}
	return nil, nil
}

func (r *reportResolver) AuditEntries() ([]string, error) {
	if r.v.AuditEntries == nil {
		return nil, nil
	}
	e := *r.v.AuditEntries
	logs := make([]string, len(e))
	var err error
	for i, l := range e {
		logs[i], err = json.MarshalToString(l)
		if err != nil {
			return nil, err
		}
	}
	return logs, nil
}
