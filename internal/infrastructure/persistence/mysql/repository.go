package mysql

import (
	"context"
	"errors"
	"time"

	"c2c_monitor/internal/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PricePointDAO represents the database schema for C2C prices
type PricePointDAO struct {
	ID              int64     `gorm:"primaryKey;autoIncrement"`
	CreatedAt       time.Time `gorm:"index:idx_query,priority:5"` // Part of composite index
	Exchange        string    `gorm:"type:varchar(32);index:idx_query,priority:1"`
	Symbol          string    `gorm:"type:varchar(10)"`
	Fiat            string    `gorm:"type:varchar(10)"`
	Side            string    `gorm:"type:varchar(10);index:idx_query,priority:2"`
	TargetAmount    float64   `gorm:"index:idx_query,priority:3"`
	Rank            int       `gorm:"index:idx_query,priority:4"`
	Price           float64   `gorm:"type:decimal(18,8)"`
	MerchantID      string    `gorm:"type:varchar(64);index"`
	PayMethods      string    `gorm:"type:text"`
	MinAmount       float64   `gorm:"type:decimal(18,8)"`
	MaxAmount       float64   `gorm:"type:decimal(18,8)"`
	AvailableAmount float64   `gorm:"type:decimal(18,8)"`
}

func (PricePointDAO) TableName() string {
	return "c2c_prices"
}

// C2CPriceHourlyDAO stores hourly aggregated lowest prices.
type C2CPriceHourlyDAO struct {
	ID           int64     `gorm:"primaryKey;autoIncrement"`
	BucketTime   time.Time `gorm:"uniqueIndex:idx_c2c_hour,priority:1;index"`
	Exchange     string    `gorm:"type:varchar(32);uniqueIndex:idx_c2c_hour,priority:2;index"`
	Symbol       string    `gorm:"type:varchar(10);uniqueIndex:idx_c2c_hour,priority:3"`
	Fiat         string    `gorm:"type:varchar(10);uniqueIndex:idx_c2c_hour,priority:4"`
	Side         string    `gorm:"type:varchar(10);uniqueIndex:idx_c2c_hour,priority:5;index"`
	TargetAmount float64   `gorm:"uniqueIndex:idx_c2c_hour,priority:6;index"`
	Rank         int       `gorm:"uniqueIndex:idx_c2c_hour,priority:7;index"`
	Price        float64   `gorm:"type:decimal(18,8)"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (C2CPriceHourlyDAO) TableName() string {
	return "c2c_prices_hourly"
}

// C2CPriceDailyDAO stores daily aggregated lowest prices.
type C2CPriceDailyDAO struct {
	ID           int64     `gorm:"primaryKey;autoIncrement"`
	BucketTime   time.Time `gorm:"uniqueIndex:idx_c2c_day,priority:1;index"`
	Exchange     string    `gorm:"type:varchar(32);uniqueIndex:idx_c2c_day,priority:2;index"`
	Symbol       string    `gorm:"type:varchar(10);uniqueIndex:idx_c2c_day,priority:3"`
	Fiat         string    `gorm:"type:varchar(10);uniqueIndex:idx_c2c_day,priority:4"`
	Side         string    `gorm:"type:varchar(10);uniqueIndex:idx_c2c_day,priority:5;index"`
	TargetAmount float64   `gorm:"uniqueIndex:idx_c2c_day,priority:6;index"`
	Rank         int       `gorm:"uniqueIndex:idx_c2c_day,priority:7;index"`
	Price        float64   `gorm:"type:decimal(18,8)"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (C2CPriceDailyDAO) TableName() string {
	return "c2c_prices_daily"
}

// MerchantDAO represents the database schema for merchants
type MerchantDAO struct {
	ID         int64  `gorm:"primaryKey;autoIncrement"`
	Exchange   string `gorm:"type:varchar(32);uniqueIndex:idx_merchant"`
	MerchantID string `gorm:"type:varchar(64);uniqueIndex:idx_merchant"`
	NickName   string `gorm:"type:varchar(128)"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (MerchantDAO) TableName() string {
	return "merchants"
}

// ForexRateDAO represents the database schema for Forex rates
type ForexRateDAO struct {
	ID        int64     `gorm:"primaryKey;autoIncrement"`
	CreatedAt time.Time `gorm:"index:idx_time"`
	Source    string    `gorm:"type:varchar(32)"`
	Pair      string    `gorm:"type:varchar(10)"`
	Rate      float64   `gorm:"type:decimal(18,6)"`
}

func (ForexRateDAO) TableName() string {
	return "forex_rates"
}

// ForexRateHourlyDAO stores hourly aggregated forex rate snapshots.
type ForexRateHourlyDAO struct {
	ID         int64     `gorm:"primaryKey;autoIncrement"`
	BucketTime time.Time `gorm:"uniqueIndex:idx_forex_hour,priority:1;index"`
	Pair       string    `gorm:"type:varchar(10);uniqueIndex:idx_forex_hour,priority:2;index"`
	Source     string    `gorm:"type:varchar(32)"`
	Rate       float64   `gorm:"type:decimal(18,6)"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (ForexRateHourlyDAO) TableName() string {
	return "forex_rates_hourly"
}

// ForexRateDailyDAO stores daily aggregated forex rate snapshots.
type ForexRateDailyDAO struct {
	ID         int64     `gorm:"primaryKey;autoIncrement"`
	BucketTime time.Time `gorm:"uniqueIndex:idx_forex_day,priority:1;index"`
	Pair       string    `gorm:"type:varchar(10);uniqueIndex:idx_forex_day,priority:2;index"`
	Source     string    `gorm:"type:varchar(32)"`
	Rate       float64   `gorm:"type:decimal(18,6)"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (ForexRateDailyDAO) TableName() string {
	return "forex_rates_daily"
}

// AlertStateDAO stores dynamic alert thresholds for restart recovery.
type AlertStateDAO struct {
	ID           int64     `gorm:"primaryKey;autoIncrement"`
	Exchange     string    `gorm:"type:varchar(32);uniqueIndex:idx_alert_state,priority:1"`
	Side         string    `gorm:"type:varchar(10);uniqueIndex:idx_alert_state,priority:2"`
	TargetAmount float64   `gorm:"type:decimal(18,8);uniqueIndex:idx_alert_state,priority:3"`
	TriggerPrice float64   `gorm:"type:decimal(18,8)"`
	LastAlertAt  time.Time `gorm:"index"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (AlertStateDAO) TableName() string {
	return "alert_states"
}

// MySQLRepository implements domain.IRepository
type MySQLRepository struct {
	db *gorm.DB
}

// NewMySQLRepository creates a new repository instance
func NewMySQLRepository(db *gorm.DB) *MySQLRepository {
	return &MySQLRepository{db: db}
}

// AutoMigrate creates the tables
func (r *MySQLRepository) AutoMigrate() error {
	return r.db.AutoMigrate(
		&PricePointDAO{},
		&C2CPriceHourlyDAO{},
		&C2CPriceDailyDAO{},
		&ForexRateDAO{},
		&ForexRateHourlyDAO{},
		&ForexRateDailyDAO{},
		&MerchantDAO{},
		&AlertStateDAO{},
	)
}

func bucketTime(t time.Time, granularity domain.HistoryGranularity) time.Time {
	switch granularity {
	case domain.HistoryGranularityHour:
		return t.Truncate(time.Hour)
	case domain.HistoryGranularityDay:
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	default:
		return t
	}
}

func c2cTableByGranularity(granularity domain.HistoryGranularity) string {
	switch granularity {
	case domain.HistoryGranularityHour:
		return "c2c_prices_hourly"
	case domain.HistoryGranularityDay:
		return "c2c_prices_daily"
	default:
		return "c2c_prices"
	}
}

func forexTableByGranularity(granularity domain.HistoryGranularity) string {
	switch granularity {
	case domain.HistoryGranularityHour:
		return "forex_rates_hourly"
	case domain.HistoryGranularityDay:
		return "forex_rates_daily"
	default:
		return "forex_rates"
	}
}

// --- Price Operations ---

func (r *MySQLRepository) SavePricePoints(ctx context.Context, points []*domain.PricePoint) error {
	if len(points) == 0 {
		return nil
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		daos := make([]*PricePointDAO, len(points))
		for i, p := range points {
			daos[i] = &PricePointDAO{
				CreatedAt:       p.CreatedAt,
				Exchange:        p.Exchange,
				Symbol:          p.Symbol,
				Fiat:            p.Fiat,
				Side:            p.Side,
				TargetAmount:    p.TargetAmount,
				Rank:            p.Rank,
				Price:           p.Price,
				MerchantID:      p.MerchantID,
				PayMethods:      p.PayMethods,
				MinAmount:       p.MinAmount,
				MaxAmount:       p.MaxAmount,
				AvailableAmount: p.AvailableAmount,
			}
		}
		if err := tx.Create(daos).Error; err != nil {
			return err
		}

		for _, p := range points {
			if err := upsertC2CAggregate(tx, p, domain.HistoryGranularityHour); err != nil {
				return err
			}
			if err := upsertC2CAggregate(tx, p, domain.HistoryGranularityDay); err != nil {
				return err
			}
		}

		return nil
	})
}

func upsertC2CAggregate(tx *gorm.DB, p *domain.PricePoint, granularity domain.HistoryGranularity) error {
	bucket := bucketTime(p.CreatedAt, granularity)
	switch granularity {
	case domain.HistoryGranularityHour:
		dao := &C2CPriceHourlyDAO{
			BucketTime:   bucket,
			Exchange:     p.Exchange,
			Symbol:       p.Symbol,
			Fiat:         p.Fiat,
			Side:         p.Side,
			TargetAmount: p.TargetAmount,
			Rank:         p.Rank,
			Price:        p.Price,
			CreatedAt:    bucket,
		}
		return tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "bucket_time"},
				{Name: "exchange"},
				{Name: "symbol"},
				{Name: "fiat"},
				{Name: "side"},
				{Name: "target_amount"},
				{Name: "rank"},
			},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"price":      gorm.Expr("LEAST(price, VALUES(price))"),
				"updated_at": gorm.Expr("NOW()"),
			}),
		}).Create(dao).Error
	case domain.HistoryGranularityDay:
		dao := &C2CPriceDailyDAO{
			BucketTime:   bucket,
			Exchange:     p.Exchange,
			Symbol:       p.Symbol,
			Fiat:         p.Fiat,
			Side:         p.Side,
			TargetAmount: p.TargetAmount,
			Rank:         p.Rank,
			Price:        p.Price,
			CreatedAt:    bucket,
		}
		return tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "bucket_time"},
				{Name: "exchange"},
				{Name: "symbol"},
				{Name: "fiat"},
				{Name: "side"},
				{Name: "target_amount"},
				{Name: "rank"},
			},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"price":      gorm.Expr("LEAST(price, VALUES(price))"),
				"updated_at": gorm.Expr("NOW()"),
			}),
		}).Create(dao).Error
	default:
		return nil
	}
}

func (r *MySQLRepository) GetPriceHistory(ctx context.Context, filter domain.PriceQueryFilter) ([]*domain.PricePoint, error) {
	return r.getPriceHistoryFromTable(ctx, filter, "c2c_prices", true)
}

func (r *MySQLRepository) GetPriceHistoryByGranularity(ctx context.Context, filter domain.PriceQueryFilter, granularity domain.HistoryGranularity) ([]*domain.PricePoint, error) {
	tableName := c2cTableByGranularity(granularity)
	withMerchant := granularity == domain.HistoryGranularityRaw
	return r.getPriceHistoryFromTable(ctx, filter, tableName, withMerchant)
}

func (r *MySQLRepository) getPriceHistoryFromTable(ctx context.Context, filter domain.PriceQueryFilter, tableName string, withMerchant bool) ([]*domain.PricePoint, error) {
	type resultRow struct {
		ID              int64     `gorm:"column:id"`
		CreatedAt       time.Time `gorm:"column:created_at"`
		Exchange        string    `gorm:"column:exchange"`
		Symbol          string    `gorm:"column:symbol"`
		Fiat            string    `gorm:"column:fiat"`
		Side            string    `gorm:"column:side"`
		TargetAmount    float64   `gorm:"column:target_amount"`
		Rank            int       `gorm:"column:rank"`
		Price           float64   `gorm:"column:price"`
		MerchantID      string    `gorm:"column:merchant_id"`
		PayMethods      string    `gorm:"column:pay_methods"`
		MinAmount       float64   `gorm:"column:min_amount"`
		MaxAmount       float64   `gorm:"column:max_amount"`
		AvailableAmount float64   `gorm:"column:available_amount"`
		NickName        string    `gorm:"column:nick_name"`
	}

	var rows []resultRow

	query := r.db.WithContext(ctx).Table(tableName)
	if withMerchant {
		query = query.Select(tableName + ".*, merchants.nick_name").
			Joins("LEFT JOIN merchants ON " + tableName + ".merchant_id = merchants.merchant_id AND " + tableName + ".exchange = merchants.exchange")
	} else {
		query = query.Select(tableName + ".*")
	}

	if filter.Exchange != "" {
		query = query.Where(tableName+".exchange = ?", filter.Exchange)
	}
	if filter.Symbol != "" {
		query = query.Where(tableName+".symbol = ?", filter.Symbol)
	}
	if filter.Fiat != "" {
		query = query.Where(tableName+".fiat = ?", filter.Fiat)
	}
	if filter.Side != "" {
		query = query.Where(tableName+".side = ?", filter.Side)
	}
	if filter.TargetAmount != nil {
		query = query.Where(tableName+".target_amount = ?", *filter.TargetAmount)
	}
	if filter.Rank > 0 {
		query = query.Where(tableName+".`rank` = ?", filter.Rank)
	}
	if !filter.StartTime.IsZero() {
		query = query.Where(tableName+".created_at >= ?", filter.StartTime)
	}
	if !filter.EndTime.IsZero() {
		query = query.Where(tableName+".created_at <= ?", filter.EndTime)
	}

	query = query.Order(tableName + ".created_at ASC")

	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}

	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}

	results := make([]*domain.PricePoint, len(rows))
	for i, row := range rows {
		results[i] = &domain.PricePoint{
			ID:              row.ID,
			CreatedAt:       row.CreatedAt,
			Exchange:        row.Exchange,
			Symbol:          row.Symbol,
			Fiat:            row.Fiat,
			Side:            row.Side,
			TargetAmount:    row.TargetAmount,
			Rank:            row.Rank,
			Price:           row.Price,
			MerchantID:      row.MerchantID,
			PayMethods:      row.PayMethods,
			MinAmount:       row.MinAmount,
			MaxAmount:       row.MaxAmount,
			AvailableAmount: row.AvailableAmount,
			Merchant:        row.NickName,
		}
	}
	return results, nil
}

// --- Merchant Operations ---

func (r *MySQLRepository) SaveMerchant(ctx context.Context, m *domain.Merchant) error {
	dao := &MerchantDAO{
		Exchange:   m.Exchange,
		MerchantID: m.MerchantID,
		NickName:   m.NickName,
		CreatedAt:  m.CreatedAt,
		UpdatedAt:  m.UpdatedAt,
	}

	// Upsert based on (exchange, merchant_id)
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "exchange"}, {Name: "merchant_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"nick_name", "updated_at"}),
	}).Create(dao).Error
}

// --- Forex Operations ---

func (r *MySQLRepository) SaveForexRate(ctx context.Context, rate *domain.ForexRate) error {
	dao := &ForexRateDAO{
		CreatedAt: rate.CreatedAt,
		Source:    rate.Source,
		Pair:      rate.Pair,
		Rate:      rate.Rate,
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(dao).Error; err != nil {
			return err
		}
		if err := upsertForexAggregate(tx, rate, domain.HistoryGranularityHour); err != nil {
			return err
		}
		if err := upsertForexAggregate(tx, rate, domain.HistoryGranularityDay); err != nil {
			return err
		}
		return nil
	})
}

func (r *MySQLRepository) GetLatestForexRate(ctx context.Context, pair string) (*domain.ForexRate, error) {
	var dao ForexRateDAO
	err := r.db.WithContext(ctx).Where("pair = ?", pair).Order("created_at DESC").First(&dao).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // Return nil if no rate found, let service handle it
		}
		return nil, err
	}
	return &domain.ForexRate{
		ID:        dao.ID,
		CreatedAt: dao.CreatedAt,
		Source:    dao.Source,
		Pair:      dao.Pair,
		Rate:      dao.Rate,
	}, nil
}

func (r *MySQLRepository) GetForexHistory(ctx context.Context, pair string, start, end time.Time) ([]*domain.ForexRate, error) {
	return r.getForexHistoryFromTable(ctx, pair, start, end, "forex_rates")
}

func (r *MySQLRepository) GetForexHistoryByGranularity(ctx context.Context, pair string, start, end time.Time, granularity domain.HistoryGranularity) ([]*domain.ForexRate, error) {
	tableName := forexTableByGranularity(granularity)
	return r.getForexHistoryFromTable(ctx, pair, start, end, tableName)
}

func (r *MySQLRepository) getForexHistoryFromTable(ctx context.Context, pair string, start, end time.Time, tableName string) ([]*domain.ForexRate, error) {
	type forexRow struct {
		ID        int64     `gorm:"column:id"`
		CreatedAt time.Time `gorm:"column:created_at"`
		Source    string    `gorm:"column:source"`
		Pair      string    `gorm:"column:pair"`
		Rate      float64   `gorm:"column:rate"`
	}

	var rows []forexRow
	query := r.db.WithContext(ctx).Table(tableName).Where(tableName+".pair = ?", pair)

	if !start.IsZero() {
		query = query.Where(tableName+".created_at >= ?", start)
	}
	if !end.IsZero() {
		query = query.Where(tableName+".created_at <= ?", end)
	}

	query = query.Order(tableName + ".created_at ASC")

	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}

	results := make([]*domain.ForexRate, len(rows))
	for i, row := range rows {
		results[i] = &domain.ForexRate{
			ID:        row.ID,
			CreatedAt: row.CreatedAt,
			Source:    row.Source,
			Pair:      row.Pair,
			Rate:      row.Rate,
		}
	}
	return results, nil
}

func upsertForexAggregate(tx *gorm.DB, rate *domain.ForexRate, granularity domain.HistoryGranularity) error {
	bucket := bucketTime(rate.CreatedAt, granularity)
	switch granularity {
	case domain.HistoryGranularityHour:
		dao := &ForexRateHourlyDAO{
			BucketTime: bucket,
			Pair:       rate.Pair,
			Source:     rate.Source,
			Rate:       rate.Rate,
			CreatedAt:  bucket,
		}
		return tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "bucket_time"},
				{Name: "pair"},
			},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"source":     rate.Source,
				"rate":       rate.Rate,
				"updated_at": gorm.Expr("NOW()"),
			}),
		}).Create(dao).Error
	case domain.HistoryGranularityDay:
		dao := &ForexRateDailyDAO{
			BucketTime: bucket,
			Pair:       rate.Pair,
			Source:     rate.Source,
			Rate:       rate.Rate,
			CreatedAt:  bucket,
		}
		return tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "bucket_time"},
				{Name: "pair"},
			},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"source":     rate.Source,
				"rate":       rate.Rate,
				"updated_at": gorm.Expr("NOW()"),
			}),
		}).Create(dao).Error
	default:
		return nil
	}
}

// --- Alert State Operations ---

func (r *MySQLRepository) UpsertAlertState(ctx context.Context, state *domain.AlertState) error {
	dao := &AlertStateDAO{
		Exchange:     state.Exchange,
		Side:         state.Side,
		TargetAmount: state.TargetAmount,
		TriggerPrice: state.TriggerPrice,
		LastAlertAt:  state.LastAlertAt,
	}

	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "exchange"},
			{Name: "side"},
			{Name: "target_amount"},
		},
		DoUpdates: clause.AssignmentColumns([]string{"trigger_price", "last_alert_at", "updated_at"}),
	}).Create(dao).Error
}

func (r *MySQLRepository) DeleteAlertState(ctx context.Context, exchange, side string, amount float64) error {
	return r.db.WithContext(ctx).
		Where("exchange = ? AND side = ? AND target_amount = ?", exchange, side, amount).
		Delete(&AlertStateDAO{}).Error
}

func (r *MySQLRepository) GetAlertStates(ctx context.Context) ([]*domain.AlertState, error) {
	var daos []AlertStateDAO
	if err := r.db.WithContext(ctx).Find(&daos).Error; err != nil {
		return nil, err
	}

	results := make([]*domain.AlertState, len(daos))
	for i, d := range daos {
		results[i] = &domain.AlertState{
			ID:           d.ID,
			Exchange:     d.Exchange,
			Side:         d.Side,
			TargetAmount: d.TargetAmount,
			TriggerPrice: d.TriggerPrice,
			LastAlertAt:  d.LastAlertAt,
			CreatedAt:    d.CreatedAt,
			UpdatedAt:    d.UpdatedAt,
		}
	}

	return results, nil
}
