package mysql

import (
	"context"
	"errors"
	"time"

	"c2c_monitor/internal/domain"
	"gorm.io/gorm"
)

// PricePointDAO represents the database schema for C2C prices
type PricePointDAO struct {
	ID           int64     `gorm:"primaryKey;autoIncrement"`
	CreatedAt    time.Time `gorm:"index:idx_query,priority:5"` // Part of composite index
	Exchange     string    `gorm:"type:varchar(32);index:idx_query,priority:1"`
	Symbol       string    `gorm:"type:varchar(10)"`
	Fiat         string    `gorm:"type:varchar(10)"`
	Side         string    `gorm:"type:varchar(10);index:idx_query,priority:2"`
	TargetAmount float64   `gorm:"index:idx_query,priority:3"`
	Rank         int       `gorm:"index:idx_query,priority:4"`
	Price        float64   `gorm:"type:decimal(18,8)"`
}

func (PricePointDAO) TableName() string {
	return "c2c_prices"
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
	return r.db.AutoMigrate(&PricePointDAO{}, &ForexRateDAO{})
}

// --- Price Operations ---

func (r *MySQLRepository) SavePricePoints(ctx context.Context, points []*domain.PricePoint) error {
	if len(points) == 0 {
		return nil
	}
	daos := make([]*PricePointDAO, len(points))
	for i, p := range points {
		daos[i] = &PricePointDAO{
			CreatedAt:    p.CreatedAt,
			Exchange:     p.Exchange,
			Symbol:       p.Symbol,
			Fiat:         p.Fiat,
			Side:         p.Side,
			TargetAmount: p.TargetAmount,
			Rank:         p.Rank,
			Price:        p.Price,
		}
	}
	return r.db.WithContext(ctx).Create(daos).Error
}

func (r *MySQLRepository) GetPriceHistory(ctx context.Context, filter domain.PriceQueryFilter) ([]*domain.PricePoint, error) {
	var daos []PricePointDAO
	query := r.db.WithContext(ctx).Model(&PricePointDAO{})

	if filter.Exchange != "" {
		query = query.Where("exchange = ?", filter.Exchange)
	}
	if filter.Symbol != "" {
		query = query.Where("symbol = ?", filter.Symbol)
	}
	if filter.Fiat != "" {
		query = query.Where("fiat = ?", filter.Fiat)
	}
	if filter.Side != "" {
		query = query.Where("side = ?", filter.Side)
	}
	if filter.TargetAmount > 0 {
		query = query.Where("target_amount = ?", filter.TargetAmount)
	}
	if filter.Rank > 0 {
		query = query.Where("`rank` = ?", filter.Rank)
	}
	if !filter.StartTime.IsZero() {
		query = query.Where("created_at >= ?", filter.StartTime)
	}
	if !filter.EndTime.IsZero() {
		query = query.Where("created_at <= ?", filter.EndTime)
	}

	query = query.Order("created_at ASC")

	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}

	if err := query.Find(&daos).Error; err != nil {
		return nil, err
	}

	results := make([]*domain.PricePoint, len(daos))
	for i, d := range daos {
		results[i] = &domain.PricePoint{
			ID:           d.ID,
			CreatedAt:    d.CreatedAt,
			Exchange:     d.Exchange,
			Symbol:       d.Symbol,
			Fiat:         d.Fiat,
			Side:         d.Side,
			TargetAmount: d.TargetAmount,
			Rank:         d.Rank,
			Price:        d.Price,
		}
	}
	return results, nil
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
