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

// MerchantDAO represents the database schema for merchants
type MerchantDAO struct {
	ID         int64     `gorm:"primaryKey;autoIncrement"`
	Exchange   string    `gorm:"type:varchar(32);uniqueIndex:idx_merchant"`
	MerchantID string    `gorm:"type:varchar(64);uniqueIndex:idx_merchant"`
	NickName   string    `gorm:"type:varchar(128)"`
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
	return r.db.AutoMigrate(&PricePointDAO{}, &ForexRateDAO{}, &MerchantDAO{})
}

// --- Price Operations ---

func (r *MySQLRepository) SavePricePoints(ctx context.Context, points []*domain.PricePoint) error {
	if len(points) == 0 {
		return nil
	}
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
	return r.db.WithContext(ctx).Create(daos).Error
}

func (r *MySQLRepository) GetPriceHistory(ctx context.Context, filter domain.PriceQueryFilter) ([]*domain.PricePoint, error) {
	// Define a composite struct to hold the join result
	type Result struct {
		PricePointDAO
		NickName string `gorm:"column:nick_name"`
	}
	var daos []Result

	// Start with table to avoid model issues with Select/Join
	query := r.db.WithContext(ctx).Table("c2c_prices").
		Select("c2c_prices.*, merchants.nick_name").
		Joins("LEFT JOIN merchants ON c2c_prices.merchant_id = merchants.merchant_id AND c2c_prices.exchange = merchants.exchange")

	if filter.Exchange != "" {
		query = query.Where("c2c_prices.exchange = ?", filter.Exchange)
	}
	if filter.Symbol != "" {
		query = query.Where("c2c_prices.symbol = ?", filter.Symbol)
	}
	if filter.Fiat != "" {
		query = query.Where("c2c_prices.fiat = ?", filter.Fiat)
	}
	if filter.Side != "" {
		query = query.Where("c2c_prices.side = ?", filter.Side)
	}
	if filter.TargetAmount != nil {
		query = query.Where("c2c_prices.target_amount = ?", *filter.TargetAmount)
	}
	if filter.Rank > 0 {
		query = query.Where("c2c_prices.`rank` = ?", filter.Rank)
	}
	if !filter.StartTime.IsZero() {
		query = query.Where("c2c_prices.created_at >= ?", filter.StartTime)
	}
	if !filter.EndTime.IsZero() {
		query = query.Where("c2c_prices.created_at <= ?", filter.EndTime)
	}

	query = query.Order("c2c_prices.created_at ASC")

	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}

	if err := query.Scan(&daos).Error; err != nil {
		return nil, err
	}

	results := make([]*domain.PricePoint, len(daos))
	for i, d := range daos {
		results[i] = &domain.PricePoint{
			ID:              d.ID,
			CreatedAt:       d.CreatedAt,
			Exchange:        d.Exchange,
			Symbol:          d.Symbol,
			Fiat:            d.Fiat,
			Side:            d.Side,
			TargetAmount:    d.TargetAmount,
			Rank:            d.Rank,
			Price:           d.Price,
			MerchantID:      d.MerchantID,
			PayMethods:      d.PayMethods,
			MinAmount:       d.MinAmount,
			MaxAmount:       d.MaxAmount,
			AvailableAmount: d.AvailableAmount,
			Merchant:        d.NickName, // Populate from joined field
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
	return r.db.WithContext(ctx).Create(dao).Error
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
	var daos []ForexRateDAO
	query := r.db.WithContext(ctx).Where("pair = ?", pair)
	
	if !start.IsZero() {
		query = query.Where("created_at >= ?", start)
	}
	if !end.IsZero() {
		query = query.Where("created_at <= ?", end)
	}
	
	query = query.Order("created_at ASC")
	
	if err := query.Find(&daos).Error; err != nil {
		return nil, err
	}

	results := make([]*domain.ForexRate, len(daos))
	for i, d := range daos {
		results[i] = &domain.ForexRate{
			ID:        d.ID,
			CreatedAt: d.CreatedAt,
			Source:    d.Source,
			Pair:      d.Pair,
			Rate:      d.Rate,
		}
	}
	return results, nil
}
