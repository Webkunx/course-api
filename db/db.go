package db

import (
	"database/sql"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	_ "github.com/go-sql-driver/mysql"

	"course-api/config"
)

func Open(dsn string, v *viper.Viper) *sql.DB {
	database, err := sql.Open("mysql", dsn)
	if err != nil {
		logrus.WithError(err).Panic("failed to open database")
	}
	if err := database.Ping(); err != nil {
		logrus.WithError(err).Panic("failed to ping database")
	}
	database.SetMaxOpenConns(v.GetInt(config.DB_MAX_OPEN_CONNS))
	database.SetMaxIdleConns(v.GetInt(config.DB_MAX_IDLE_CONNS))
	database.SetConnMaxLifetime(time.Duration(v.GetInt(config.DB_CONN_MAX_LIFETIME_S)) * time.Second)
	database.SetConnMaxIdleTime(time.Duration(v.GetInt(config.DB_CONN_MAX_IDLE_S)) * time.Second)
	return database
}
