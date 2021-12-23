package query_resolvers

import (
	"context"

	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
)

type cosmeticResolver struct {
	ctx context.Context
	v   *datastructure.Cosmetic

	fields map[string]*SelectedField
}

func GenerateCosmeticResolver(ctx context.Context, cos *datastructure.Cosmetic, fields map[string]*SelectedField) *cosmeticResolver {
	return &cosmeticResolver{
		ctx:    ctx,
		v:      cos,
		fields: fields,
	}
}

func (r *cosmeticResolver) ID() string {
	return r.v.ID.Hex()
}

func (r *cosmeticResolver) Kind() string {
	return string(r.v.Kind)
}

func (r *cosmeticResolver) Name() string {
	return r.v.Name
}

func (r *cosmeticResolver) Selected() bool {
	return r.v.Selected
}

func (r *cosmeticResolver) Data() (string, error) {
	if r.v.Data == nil {
		return "{}", nil
	}
	var j []byte
	var i map[string]interface{}
	var err error

	if err = bson.Unmarshal(r.v.Data, &i); err != nil {
		logrus.WithError(err).Error("failed unmarshal data of cosmetic")
		return "{}", err
	}

	j, err = json.Marshal(i)
	if err != nil {
		logrus.WithError(err).Error("failed marshal data of cosmetic")
		return "{}", err
	}

	return string(j), nil
}
