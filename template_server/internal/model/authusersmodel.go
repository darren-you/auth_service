package model

import "github.com/zeromicro/go-zero/core/stores/sqlx"

var _ AuthUsersModel = (*customAuthUsersModel)(nil)

type (
	// AuthUsersModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAuthUsersModel.
	AuthUsersModel interface {
		authUsersModel
		withSession(session sqlx.Session) AuthUsersModel
	}

	customAuthUsersModel struct {
		*defaultAuthUsersModel
	}
)

// NewAuthUsersModel returns a model for the database table.
func NewAuthUsersModel(conn sqlx.SqlConn) AuthUsersModel {
	return &customAuthUsersModel{
		defaultAuthUsersModel: newAuthUsersModel(conn),
	}
}

func (m *customAuthUsersModel) withSession(session sqlx.Session) AuthUsersModel {
	return NewAuthUsersModel(sqlx.NewSqlConnFromSession(session))
}
