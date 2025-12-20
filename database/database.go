package database

import (
	"database/sql"
	"fmt"
	"log"
	"splash-trading-bot/lib/models"
	"time"
)

var DB *sql.DB

func InitDatabase(dataSourceName string) error {
	var err error
	DB, err = sql.Open("postgresql", dataSourceName)
	if err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}

	if err = DB.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	log.Println("Database connection established")

	createTablePSQL := `
	create table if not exists splash_records(
		id serial primary key,
		symbol varchar(30) not null,
		direction varchar(4) not null,

		trigger_level smallint not null,
		ref_last_price real not null,
		ref_fair_price real not null,

		trigger_last_price float8 not null,
		trigger_fair_price float8 not null,

		trigger_time timestamp with time zone not null,
		volume_24h float8 not null,

		returned boolean default false,
		return_time int default 0,
		max_deviation smallint default 0,

		long_probability smallint default 0,
		short_probability smallint default 0
	);`

	_, err = DB.Exec(createTablePSQL)
	if err != nil {
		return fmt.Errorf("failed to create splash_records table: %w", err)
	}

	log.Println("splash_records table is ready")

	return nil
}

func SaveSplashRecord(r models.SplashRecord) (int64, error) {
	insertPSQL := `
	insert into splash_records(
		symbol, direction,
		trigger_level, ref_last_price, ref_fair_price,
		trigger_last_price, trigger_fair_price,
		trigger_time, volume_24h,
		returned, return_time, max_deviation,
		long_probability, short_probability
	) values (
	 $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
	) RETURNING id;`
	var id int64
	err := DB.QueryRow(
		insertPSQL,
		r.Symbol, r.Direction,
		r.TriggerLevel, r.RefLastPrice, r.RefFairPrice,
		r.TriggerLastPrice, r.TriggerFairPrice,
		r.TriggerTime, r.Volume24h,
		r.Returned, r.ReturnTime, r.MaxDeviation,
		r.LongProbability, r.ShortProbability,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to insert splash record: %w", err)
	}

	log.Printf("DB: Splash record saved successfully ID: %d", id)
	return id, nil
}

func UpdateSplashRecord(r models.SplashRecord) error {
	if r.ID == 0 {
		return fmt.Errorf("cannot update splash record with ID 0")
	}

	returnTimeMs := r.ReturnTime.Seconds()
	updatePSQL := `
	update splash_records
	set returned = $1,
		return_time = $2,
		max_deviation = $3,
		long_probability = $4,
		short_probability = $5
	where id = $6;`

	_, err := DB.Exec(
		updatePSQL,
		r.Returned,
		returnTimeMs,
		r.MaxDeviation,
		r.LongProbability,
		r.ShortProbability,
		r.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update splash record ID %d: %w", r.ID, err)
	}

	log.Printf("DB: Splash record ID %d updated successfully", r.ID)
	return nil
}

func GetSplashRecordByID(id int64) (models.SplashRecord, error) {
	queryPSQL := `
	select 
		id, symbol, direction,
		trigger_level, ref_last_price, ref_fair_price,
		trigger_last_price, trigger_fair_price,
		trigger_time, volume_24h,
		returned, return_time, max_deviation,
		long_probability, short_probability
	from splash_records where id = $1;`
	r := models.SplashRecord{}
	var returnTime int64

	err := DB.QueryRow(queryPSQL, id).Scan(
		&r.ID, &r.Symbol, &r.Direction,
		&r.TriggerLevel, &r.RefLastPrice, &r.RefFairPrice,
		&r.TriggerLastPrice, &r.TriggerFairPrice,
		&r.TriggerTime, &r.Volume24h,
		&r.Returned, &r.ReturnTime, &r.MaxDeviation,
		&r.LongProbability, &r.ShortProbability,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return models.SplashRecord{}, fmt.Errorf("record with ID %d not found", id)
		}
		return models.SplashRecord{}, fmt.Errorf("failed to retrieve splash record ID %d: %w", id, err)
	}

	r.ReturnTime = time.Duration(returnTime) * time.Second

	return r, nil
}

func GetHistoricalSplashRecords(symbol string, level float64) ([]models.SplashRecord, error) {
	queryPSQL := `
	select 
		id, symbol, direction,
		trigger_level, ref_last_price, ref_fair_price,
		trigger_last_price, trigger_fair_price,
		trigger_time, volume_24h,
		returned, return_time, max_deviation,
		long_probability, short_probability
	from splash_records where symbol = $1 and trigger_level = $2 and return_time > 0
	order by trigger_time desc
	limit 100`

	rows, err := DB.Query(queryPSQL, symbol, level)
	if err != nil {
		return nil, fmt.Errorf("failed to query splash records: %w", err)
	}
	defer rows.Close()

	var records []models.SplashRecord
	for rows.Next() {
		var r models.SplashRecord
		var returnTime int64

		err := rows.Scan(
			&r.ID, &r.Symbol, &r.Direction,
			&r.TriggerLevel, &r.RefLastPrice, &r.RefFairPrice,
			&r.TriggerLastPrice, &r.TriggerFairPrice,
			&r.TriggerTime, &r.Volume24h,
			&r.Returned, &returnTime, &r.MaxDeviation,
			&r.LongProbability, &r.ShortProbability,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan splash record: %w", err)
		}

		r.ReturnTime = time.Duration(returnTime) * time.Second
		records = append(records, r)
	}

	return records, nil
}
