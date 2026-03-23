package model

import "github.com/zeromicro/go-zero/core/stores/sqlx"

var _ AuthTenantsModel = (*customAuthTenantsModel)(nil)

type (
	// AuthTenantsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAuthTenantsModel.
	AuthTenantsModel interface {
		authTenantsModel
		withSession(session sqlx.Session) AuthTenantsModel
	}

	customAuthTenantsModel struct {
		*defaultAuthTenantsModel
	}
)

// NewAuthTenantsModel returns a model for the database table.
func NewAuthTenantsModel(conn sqlx.SqlConn) AuthTenantsModel {
	return &customAuthTenantsModel{
		defaultAuthTenantsModel: newAuthTenantsModel(conn),
	}
}

func (m *customAuthTenantsModel) withSession(session sqlx.Session) AuthTenantsModel {
	return NewAuthTenantsModel(sqlx.NewSqlConnFromSession(session))
}
