package config

import "github.com/spf13/viper"

func CreateViper() *viper.Viper {
	v := viper.New()
	v.AutomaticEnv()

	v.SetDefault(PORT, "3000")
	v.SetDefault(DB_URL, "root:root@tcp(localhost:3306)/course_api")
	v.SetDefault(MIGRATIONS_DIR, "db/migrations")
	v.SetDefault(MIN_AVG_DWELL_MS, 5000)
	v.SetDefault(MIN_SINGLE_DWELL_MS, 1500)
	v.SetDefault(DWELL_WINDOW, 10)
	v.SetDefault(MAX_DAILY_LESSONS, 100)
	v.SetDefault(DATASET_SCALE, 0.01)
	v.SetDefault(DATASET_SEED, 42)
	v.SetDefault(DB_MAX_OPEN_CONNS, 25)
	v.SetDefault(DB_MAX_IDLE_CONNS, 10)
	v.SetDefault(DB_CONN_MAX_LIFETIME_S, 180)
	v.SetDefault(DB_CONN_MAX_IDLE_S, 90)

	return v
}
