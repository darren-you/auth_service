package model

import "github.com/zeromicro/go-zero/core/stores/sqlx"

var _ AuthProviderConfigsModel = (*customAuthProviderConfigsModel)(nil)

type (
	// AuthProviderConfigsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAuthProviderConfigsModel.
	AuthProviderConfigsModel interface {
		authProviderConfigsModel
		withSession(session sqlx.Session) AuthProviderConfigsModel
	}

	customAuthProviderConfigsModel struct {
		*defaultAuthProviderConfigsModel
	}
)

// NewAuthProviderConfigsModel returns a model for the database table.
func NewAuthProviderConfigsModel(conn sqlx.SqlConn) AuthProviderConfigsModel {
	return &customAuthProviderConfigsModel{
		defaultAuthProviderConfigsModel: newAuthProviderConfigsModel(conn),
	}
}

func (m *customAuthProviderConfigsModel) withSession(session sqlx.Session) AuthProviderConfigsModel {
	return NewAuthProviderConfigsModel(sqlx.NewSqlConnFromSession(session))
}
