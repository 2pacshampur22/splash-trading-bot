package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"splash-trading-bot/lib/models"
	"time"

	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

var DB *sql.DB
var dbDriver string // "postgres" или "sqlite3"

// InitDatabase пробует подключиться к Postgres.
// Если конфиг пустой или подключение упало — разворачивает локальный SQLite.
func InitDatabase(dataSourceName string) error {
	// Пробуем Postgres, только если все параметры заданы
	pgHost := os.Getenv("DB_HOST")
	pgPort := os.Getenv("DB_PORT")
	pgUser := os.Getenv("DB_USER")
	pgPassword := os.Getenv("DB_PASSWORD")
	pgName := os.Getenv("DB_NAME")

	if pgHost != "" && pgUser != "" && pgName != "" {
		connStr := fmt.Sprintf(
			"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
			pgHost, pgPort, pgUser, pgPassword, pgName,
		)
		db, err := sql.Open("postgres", connStr)
		if err == nil {
			db.SetMaxOpenConns(25)
			db.SetMaxIdleConns(10)
			if err = db.Ping(); err == nil {
				DB = db
				dbDriver = "postgres"
				log.Println("DB: Connected to remote PostgreSQL")
				return initSchema()
			}
			log.Printf("DB: Postgres ping failed (%v), falling back to SQLite", err)
			db.Close()
		}
	} else {
		log.Println("DB: No Postgres config found (set DB_HOST, DB_USER, DB_NAME env vars)")
	}

	// Fallback → SQLite
	return initSQLite()
}

func initSQLite() error {
	dbPath := "./terminus_local.db"
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return fmt.Errorf("sqlite open error: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite — single writer
	if err = db.Ping(); err != nil {
		return fmt.Errorf("sqlite ping error: %w", err)
	}
	DB = db
	dbDriver = "sqlite3"
	log.Printf("DB: Using local SQLite at %s", dbPath)
	return initSchema()
}

func initSchema() error {
	isSQLite := dbDriver == "sqlite3"

	// ─── splash_records ───────────────────────────────────────────────────────
	splashTable := `
	CREATE TABLE IF NOT EXISTS splash_records (
		id               INTEGER PRIMARY KEY ` + autoIncrement(isSQLite) + `,
		symbol           VARCHAR(30)  NOT NULL,
		direction        VARCHAR(10)  NOT NULL,
		trigger_level    SMALLINT     NOT NULL,
		trigger_time     TIMESTAMP    NOT NULL,
		ref_last_price   FLOAT8       NOT NULL,
		ref_fair_price   FLOAT8       NOT NULL,
		trigger_last_price FLOAT8     NOT NULL,
		trigger_fair_price FLOAT8     NOT NULL,
		basis_gap        FLOAT8       DEFAULT 0.0,
		trigger_speed_sec FLOAT8,
		volume_24h       FLOAT8       NOT NULL,
		returned         BOOLEAN      DEFAULT FALSE,
		return_time      FLOAT8       DEFAULT 0,
		max_deviation    FLOAT8       DEFAULT 0,
		prob_win         FLOAT8       DEFAULT 0,
		time_window      SMALLINT     NOT NULL
	);`

	if _, err := DB.Exec(splashTable); err != nil {
		return fmt.Errorf("splash_records table: %w", err)
	}

	// Уникальный индекс — синтаксис одинаков для обеих СУБД
	if !isSQLite {
		DB.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_active_splash 
			ON splash_records (symbol, trigger_level) WHERE (returned = false)`)
	}

	// ─── spread_records ───────────────────────────────────────────────────────
	spreadTable := `
	CREATE TABLE IF NOT EXISTS spread_records (
		id              INTEGER PRIMARY KEY ` + autoIncrement(isSQLite) + `,
		symbol          VARCHAR(30)  NOT NULL,
		buy_exchange    VARCHAR(50)  NOT NULL,
		sell_exchange   VARCHAR(50)  NOT NULL,
		buy_price       FLOAT8       NOT NULL,
		sell_price      FLOAT8       NOT NULL,
		spread_pct      FLOAT8       NOT NULL,
		volume_24h      FLOAT8       NOT NULL,
		source          VARCHAR(20)  NOT NULL,
		detected_at     TIMESTAMP    NOT NULL
	);`

	if _, err := DB.Exec(spreadTable); err != nil {
		return fmt.Errorf("spread_records table: %w", err)
	}

	// Индекс для быстрой истории по символу
	DB.Exec(`CREATE INDEX IF NOT EXISTS idx_spread_symbol ON spread_records (symbol)`)
	DB.Exec(`CREATE INDEX IF NOT EXISTS idx_spread_time ON spread_records (detected_at)`)

	log.Printf("DB: Schema ready [driver=%s]", dbDriver)
	return nil
}

func autoIncrement(isSQLite bool) string {
	if isSQLite {
		return "AUTOINCREMENT"
	}
	return ""
}

// ─── Spread CRUD ──────────────────────────────────────────────────────────────

func SaveSpreadRecord(r models.SpreadRecord) error {
	q := `INSERT INTO spread_records
		(symbol, buy_exchange, sell_exchange, buy_price, sell_price, spread_pct, volume_24h, source, detected_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	if dbDriver == "postgres" {
		q = `INSERT INTO spread_records
		(symbol, buy_exchange, sell_exchange, buy_price, sell_price, spread_pct, volume_24h, source, detected_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	}

	_, err := DB.Exec(q,
		r.Symbol, r.BuyExchange, r.SellExchange,
		r.BuyPrice, r.SellPrice, r.SpreadPct,
		r.Volume24h, r.Source, r.DetectedAt,
	)
	return err
}

func GetSpreadHistory(symbol string, limit int) ([]models.SpreadRecord, error) {
	q := `SELECT id, symbol, buy_exchange, sell_exchange, buy_price, sell_price, spread_pct, volume_24h, source, detected_at
		FROM spread_records WHERE symbol = ? ORDER BY detected_at DESC LIMIT ?`
	if dbDriver == "postgres" {
		q = `SELECT id, symbol, buy_exchange, sell_exchange, buy_price, sell_price, spread_pct, volume_24h, source, detected_at
		FROM spread_records WHERE symbol = $1 ORDER BY detected_at DESC LIMIT $2`
	}

	rows, err := DB.Query(q, symbol, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []models.SpreadRecord
	for rows.Next() {
		var r models.SpreadRecord
		if err := rows.Scan(&r.ID, &r.Symbol, &r.BuyExchange, &r.SellExchange,
			&r.BuyPrice, &r.SellPrice, &r.SpreadPct, &r.Volume24h, &r.Source, &r.DetectedAt); err != nil {
			continue
		}
		records = append(records, r)
	}
	return records, nil
}

// ─── Splash CRUD ──────────────────────────────────────────────────────────────

func GetContextStats(direction string, level int, volume float64, basisGap float64, window int) (total int, wins int, err error) {
	volMin, volMax := volume*0.5, volume*2.0
	gapMin, gapMax := basisGap-0.5, basisGap+0.5

	var q string
	if dbDriver == "postgres" {
		q = `SELECT count(*), coalesce(sum(case when returned = true then 1 else 0 end), 0)
			FROM splash_records
			WHERE direction = $1 AND trigger_level = $2
			  AND volume_24h BETWEEN $3 AND $4
			  AND basis_gap  BETWEEN $5 AND $6
			  AND time_window = $7
			  AND (returned = true OR trigger_time < (NOW() - (time_window * interval '1 minute')))`
		err = DB.QueryRow(q, direction, level, volMin, volMax, gapMin, gapMax, window).Scan(&total, &wins)
	} else {
		q = `SELECT count(*), coalesce(sum(case when returned = 1 then 1 else 0 end), 0)
			FROM splash_records
			WHERE direction = ? AND trigger_level = ?
			  AND volume_24h BETWEEN ? AND ?
			  AND basis_gap  BETWEEN ? AND ?
			  AND time_window = ?
			  AND (returned = 1 OR trigger_time < datetime('now', '-' || time_window || ' minutes'))`
		err = DB.QueryRow(q, direction, level, volMin, volMax, gapMin, gapMax, window).Scan(&total, &wins)
	}
	if err != nil {
		return 0, 0, fmt.Errorf("GetContextStats: %w", err)
	}
	return total, wins, nil
}

func SaveSplashRecord(r models.SplashRecord, basisGap float64, speedSeconds float64) (int64, error) {
	var id int64
	var err error

	if dbDriver == "postgres" {
		q := `INSERT INTO splash_records
			(symbol, direction, trigger_level, trigger_time,
			 ref_last_price, ref_fair_price, trigger_last_price, trigger_fair_price,
			 basis_gap, trigger_speed_sec, volume_24h, prob_win, time_window)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
			ON CONFLICT (symbol, trigger_level) WHERE (returned = false) DO NOTHING
			RETURNING id`
		err = DB.QueryRow(q,
			r.Symbol, r.Direction, r.TriggerLevel, r.TriggerTime,
			r.RefLastPrice, r.RefFairPrice, r.TriggerLastPrice, r.TriggerFairPrice,
			basisGap, speedSeconds, r.Volume24h, r.LongProbability, r.TimeWindow,
		).Scan(&id)
	} else {
		// SQLite — нет partial unique index, используем INSERT OR IGNORE
		q := `INSERT OR IGNORE INTO splash_records
			(symbol, direction, trigger_level, trigger_time,
			 ref_last_price, ref_fair_price, trigger_last_price, trigger_fair_price,
			 basis_gap, trigger_speed_sec, volume_24h, prob_win, time_window)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`
		res, e := DB.Exec(q,
			r.Symbol, r.Direction, r.TriggerLevel, r.TriggerTime,
			r.RefLastPrice, r.RefFairPrice, r.TriggerLastPrice, r.TriggerFairPrice,
			basisGap, speedSeconds, r.Volume24h, r.LongProbability, r.TimeWindow,
		)
		if e != nil {
			err = e
		} else {
			id, err = res.LastInsertId()
		}
	}

	if err != nil {
		return 0, fmt.Errorf("SaveSplashRecord: %w", err)
	}
	log.Printf("DB: Splash record saved ID: %d", id)
	return id, nil
}

func UpdateSplashRecord(r models.SplashRecord) error {
	if r.ID == 0 {
		return fmt.Errorf("cannot update splash record with ID 0")
	}
	returnTimeSec := r.ReturnTime.Seconds()

	var q string
	if dbDriver == "postgres" {
		q = `UPDATE splash_records SET returned=$1, return_time=$2, max_deviation=$3 WHERE id=$4`
	} else {
		q = `UPDATE splash_records SET returned=?, return_time=?, max_deviation=? WHERE id=?`
	}
	_, err := DB.Exec(q, r.Returned, returnTimeSec, r.MaxDeviation, r.ID)
	if err != nil {
		return fmt.Errorf("UpdateSplashRecord ID %d: %w", r.ID, err)
	}
	log.Printf("DB: Splash record ID %d updated", r.ID)
	return nil
}

func UpdateSplashLevel(id int64, level int, lastPrice, fairPrice, volume24, probWin float64, window int) error {
	var q string
	if dbDriver == "postgres" {
		q = `UPDATE splash_records SET trigger_level=$1, trigger_last_price=$2, trigger_fair_price=$3, volume_24h=$4, prob_win=$5, time_window=$6 WHERE id=$7`
	} else {
		q = `UPDATE splash_records SET trigger_level=?, trigger_last_price=?, trigger_fair_price=?, volume_24h=?, prob_win=?, time_window=? WHERE id=?`
	}
	_, err := DB.Exec(q, level, lastPrice, fairPrice, volume24, probWin, window, id)
	return err
}

func GetSplashRecordByID(id int64) (models.SplashRecord, error) {
	var q string
	if dbDriver == "postgres" {
		q = `SELECT id, symbol, direction, trigger_level, ref_last_price, ref_fair_price,
			trigger_last_price, trigger_fair_price, trigger_time, volume_24h,
			returned, return_time, max_deviation, prob_win, time_window
			FROM splash_records WHERE id = $1`
	} else {
		q = `SELECT id, symbol, direction, trigger_level, ref_last_price, ref_fair_price,
			trigger_last_price, trigger_fair_price, trigger_time, volume_24h,
			returned, return_time, max_deviation, prob_win, time_window
			FROM splash_records WHERE id = ?`
	}

	r := models.SplashRecord{}
	var returnTimeSec float64
	err := DB.QueryRow(q, id).Scan(
		&r.ID, &r.Symbol, &r.Direction,
		&r.TriggerLevel, &r.RefLastPrice, &r.RefFairPrice,
		&r.TriggerLastPrice, &r.TriggerFairPrice,
		&r.TriggerTime, &r.Volume24h,
		&r.Returned, &returnTimeSec, &r.MaxDeviation,
		&r.LongProbability, &r.TimeWindow,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return models.SplashRecord{}, fmt.Errorf("record ID %d not found", id)
		}
		return models.SplashRecord{}, fmt.Errorf("GetSplashRecordByID %d: %w", id, err)
	}
	r.ReturnTime = secToDuration(returnTimeSec)
	return r, nil
}

func secToDuration(s float64) time.Duration {
	return time.Duration(s * float64(time.Second))
}

// GetTopSpreads — топ спредов за последние N часов
func GetTopSpreads(hours int, limit int) ([]models.SpreadRecord, error) {
	q := `SELECT symbol, buy_exchange, sell_exchange, MAX(spread_pct) as max_spread, AVG(spread_pct) as avg_spread, COUNT(*) as cnt
		FROM spread_records
		WHERE detected_at > datetime('now', ?)
		GROUP BY symbol, buy_exchange, sell_exchange
		ORDER BY max_spread DESC LIMIT ?`
	if dbDriver == "postgres" {
		q = `SELECT symbol, buy_exchange, sell_exchange, MAX(spread_pct), AVG(spread_pct), COUNT(*)
		FROM spread_records
		WHERE detected_at > NOW() - INTERVAL '%d hours'
		GROUP BY symbol, buy_exchange, sell_exchange
		ORDER BY MAX(spread_pct) DESC LIMIT $1`
		q = fmt.Sprintf(q, hours)
	}

	var rows *sql.Rows
	var err error
	if dbDriver == "sqlite3" {
		rows, err = DB.Query(q, fmt.Sprintf("-%d hours", hours), limit)
	} else {
		rows, err = DB.Query(q, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []models.SpreadRecord
	for rows.Next() {
		var r models.SpreadRecord
		var maxSpread, avgSpread float64
		var cnt int
		if err := rows.Scan(&r.Symbol, &r.BuyExchange, &r.SellExchange, &maxSpread, &avgSpread, &cnt); err != nil {
			continue
		}
		r.SpreadPct = maxSpread
		records = append(records, r)
	}
	return records, nil
}
