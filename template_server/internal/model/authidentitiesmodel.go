package model

import "github.com/zeromicro/go-zero/core/stores/sqlx"

var _ AuthIdentitiesModel = (*customAuthIdentitiesModel)(nil)

type (
	// AuthIdentitiesModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAuthIdentitiesModel.
	AuthIdentitiesModel interface {
		authIdentitiesModel
		withSession(session sqlx.Session) AuthIdentitiesModel
	}

	customAuthIdentitiesModel struct {
		*defaultAuthIdentitiesModel
	}
)

// NewAuthIdentitiesModel returns a model for the database table.
func NewAuthIdentitiesModel(conn sqlx.SqlConn) AuthIdentitiesModel {
	return &customAuthIdentitiesModel{
		defaultAuthIdentitiesModel: newAuthIdentitiesModel(conn),
	}
}

func (m *customAuthIdentitiesModel) withSession(session sqlx.Session) AuthIdentitiesModel {
	return NewAuthIdentitiesModel(sqlx.NewSqlConnFromSession(session))
}
