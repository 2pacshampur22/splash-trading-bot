package database

import (
	"database/sql"
	"fmt"
	"log"
	"splash-trading-bot/lib/models"
	"time"

	_ "github.com/lib/pq"
)

var DB *sql.DB

func InitDatabase(dataSourceName string) error {
	const (
		host     = ""
		port     = ""
		user     = ""
		password = ""
		dbname   = ""
	)

	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	var err error
	DB, err = sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("ошибка открытия базы данных: %w", err)
	}

	DB.SetMaxOpenConns(25)
	DB.SetMaxIdleConns(10)
	DB.SetConnMaxLifetime(time.Minute * 5)

	if err = DB.Ping(); err != nil {
		return fmt.Errorf("не удалось подключиться к VPS (Ping): %w", err)
	}

	log.Println("Успешное подключение к удаленной базе данных на VPS")

	createTablePSQL := `
	create table if not exists splash_records(
		id serial primary key,
        symbol varchar(30) not null,
        direction varchar(10) not null,
        trigger_level smallint not null,
        trigger_time timestamp with time zone not null,
        ref_last_price float8 not null,
        ref_fair_price float8 not null,
        trigger_last_price float8 not null,
        trigger_fair_price float8 not null,
        basis_gap float8 default 0.0,
        trigger_speed_sec float8,
        volume_24h bigint not null,
        returned boolean default false,
        return_time float8 default 0,
        max_deviation float8 default 0,
        prob_win float8 default 0
	);`

	_, err = DB.Exec(createTablePSQL)
	if err != nil {
		return fmt.Errorf("failed to create splash_records table: %w", err)
	}

	var exists bool
	checkQuery := `SELECT EXISTS (
		SELECT FROM information_schema.tables 
		WHERE table_schema = 'public' 
		AND table_name = 'splash_records'
	);`

	err = DB.QueryRow(checkQuery).Scan(&exists)
	if err != nil {
		log.Printf("Warning: Could not verify table existence: %v", err)
	} else if exists {
		log.Println("Verification successful: Table 'splash_records' exists in the database.")
	} else {
		log.Println("Alert: Table was NOT created even though no error was returned.")
	}
	return nil
}

func GetContextStats(direction string, level int, volume int64, basisGap float64) (total int, wins int, err error) {
	volMin, volMax := int64(float64(volume)*0.5), int64(float64(volume)*2)

	gapMin, gapMax := basisGap-0.5, basisGap+0.5

	queryPSQL := `
	select 
		count(*) as total,
		coalesce(sum(case when returned = true then 1 else 0 end), 0) as wins
	from splash_records
	where direction = $1
		and trigger_level = $2
		and volume_24h between $3 and $4
		and basis_gap between $5 and $6
		and (returned = true or trigger_time < (now() - (interval '5 minutes' + (trigger_level * interval '2 minutes'))));`

	err = DB.QueryRow(queryPSQL, direction, level, volMin, volMax, gapMin, gapMax).Scan(&total, &wins)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to select query context stats: %w", err)
	}
	return total, wins, nil
}

func SaveSplashRecord(r models.SplashRecord, basisGap float64, speedSeconds float64) (int64, error) {
	insertPSQL := `
	insert into splash_records(
		symbol, direction, trigger_level, trigger_time, 
        ref_last_price, ref_fair_price,
        trigger_last_price, trigger_fair_price, 
        basis_gap, trigger_speed_sec, volume_24h, prob_win
	) values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12) returning id;`

	var id int64
	err := DB.QueryRow(
		insertPSQL,
		r.Symbol,
		r.Direction,
		r.TriggerLevel,
		r.TriggerTime,
		r.RefLastPrice,
		r.RefFairPrice,
		r.TriggerLastPrice,
		r.TriggerFairPrice,
		basisGap,
		speedSeconds,
		r.Volume24h,
		r.LongProbability,
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
		max_deviation = $3
	where id = $4;`

	_, err := DB.Exec(
		updatePSQL,
		r.Returned,
		returnTimeMs,
		r.MaxDeviation,
		r.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update splash record ID %d: %w", r.ID, err)
	}

	log.Printf("DB: Splash record ID %d updated successfully", r.ID)
	return nil
}

func UpdateSplashLevel(id int64, level int, lastPrice float64, fairPrice float64, volume24 int64, prob_win float64) error {
	updatePSQL := `
	update splash_records
	set trigger_level = $1,
		trigger_last_price = $2,
		trigger_fair_price = $3,
		volume_24h = $4,
		prob_win = $5
	where id = $6;`

	_, err := DB.Exec(
		updatePSQL,
		level,
		lastPrice,
		fairPrice,
		volume24,
		prob_win,
		id,
	)
	return err
}

func GetSplashRecordByID(id int64) (models.SplashRecord, error) {
	queryPSQL := `
	select 
		id, symbol, direction,
        trigger_level, ref_last_price, ref_fair_price,
        trigger_last_price, trigger_fair_price,
        trigger_time, volume_24h,
        returned, return_time, max_deviation,
        prob_win
	from splash_records where id = $1;`

	r := models.SplashRecord{}
	var returnTime int64

	err := DB.QueryRow(queryPSQL, id).Scan(
		&r.ID, &r.Symbol, &r.Direction,
		&r.TriggerLevel, &r.RefLastPrice, &r.RefFairPrice,
		&r.TriggerLastPrice, &r.TriggerFairPrice,
		&r.TriggerTime, &r.Volume24h,
		&r.Returned, &r.ReturnTime, &r.MaxDeviation,
		&r.LongProbability,
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
