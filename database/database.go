package database

// import (
// 	"database/sql"
// 	"fmt"
// 	"log"
// )

// var DB *sql.DB

// func InitDatabase(dataSourceName string) error {
// 	var err error
// 	DB, err = sql.Open("postgresql", dataSourceName)
// 	if err != nil {
// 		return fmt.Errorf("failed to open database connection: %w", err)
// 	}

// 	if err = DB.Ping(); err != nil {
// 		return fmt.Errorf("failed to ping database: %w", err)
// 	}

// 	log.Println("Database connection established")

// 	createTablePSQL := `
// 	create table if not exists splash_records(
// 		id serial primary key,
// 		symbol varchar(30) not null,
// 		direction VARCHAR(4) not null,
// 		trigger_level smallint not null,
// 		ref_last_price real not null,
// 		ref_fair_price real not null,
// 		trigger_last_price float8 not null,
// 		trigger_fair_price float8 not null,
// 		trigger_time timestamp with time zone not null,
// 		volume_24h float8 not null,
// 		returned boolean default false,
// 		return_time int default 0,
// 		max_deviation smallint default 0,
// 		long_probability smallint default 0,
// 		short_probability smallint default 0
// 	);`
// }
