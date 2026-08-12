package repository

import (
	"context"
	"errors"
	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

type BillingRepository struct{ knowledge *KnowledgeRepository }

func NewBillingRepository(i InstallRepository) *BillingRepository {
	return &BillingRepository{NewKnowledgeRepository(i)}
}
func (r *BillingRepository) ConfigurePrices(c context.Context, pro, enterprise string) error {
	return r.knowledge.withDB(c, func(db *gorm.DB) error {
		if err := db.Model(&model.Plan{}).Where("code = ?", "pro").Update("stripe_price_id", pro).Error; err != nil {
			return err
		}
		return db.Model(&model.Plan{}).Where("code = ?", "enterprise").Update("stripe_price_id", enterprise).Error
	})
}
func (r *BillingRepository) Plans(c context.Context) ([]model.Plan, error) {
	out := []model.Plan{}
	e := r.knowledge.withDB(c, func(db *gorm.DB) error { return db.Where("status = ?", "active").Order("price_cents").Find(&out).Error })
	return out, e
}
func (r *BillingRepository) Overview(c context.Context, t string) (model.BillingOverview, error) {
	var out model.BillingOverview
	e := r.knowledge.withDB(c, func(db *gorm.DB) error {
		return db.Transaction(func(tx *gorm.DB) error {
			var sub model.Subscription
			e := tx.Where("tenant_id = ?", t).First(&sub).Error
			if errors.Is(e, gorm.ErrRecordNotFound) {
				var free model.Plan
				if e = tx.Where("code = ?", "free").First(&free).Error; e != nil {
					return e
				}
				now := time.Now().UTC()
				sub = model.Subscription{ID: newUUID(), TenantID: t, PlanID: free.ID, Provider: "local", Status: "active", PeriodStart: monthStart(now), PeriodEnd: monthStart(now).AddDate(0, 1, 0), CreatedAt: now, UpdatedAt: now}
				if e = tx.Create(&sub).Error; e != nil {
					return e
				}
			} else if e != nil {
				return e
			}
			if e = tx.First(&out.Plan, "id = ?", sub.PlanID).Error; e != nil {
				return e
			}
			out.Subscription = sub
			out.Usage = map[string]int64{}
			var rows []model.UsageCounter
			if e = tx.Where("tenant_id = ? AND period_start = ?", t, monthStart(time.Now().UTC())).Find(&rows).Error; e != nil {
				return e
			}
			for _, row := range rows {
				out.Usage[row.Metric] = row.Quantity
			}
			return nil
		})
	})
	return out, e
}
func (r *BillingRepository) RecordUsage(c context.Context, t, m string, q, limit int64) (bool, error) {
	allowed := true
	err := r.knowledge.withDB(c, func(db *gorm.DB) error {
		return db.Transaction(func(tx *gorm.DB) error {
			start := monthStart(time.Now().UTC())
			row := model.UsageCounter{TenantID: t, Metric: m, PeriodStart: start}
			if e := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; e != nil {
				return e
			}
			if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND metric = ? AND period_start = ?", t, m, start).First(&row).Error; e != nil {
				return e
			}
			if q > 0 && row.Quantity+q > limit {
				allowed = false
				return nil
			}
			next := row.Quantity + q
			if next < 0 {
				next = 0
			}
			return tx.Model(&row).Updates(map[string]any{"quantity": next, "updated_at": time.Now().UTC()}).Error
		})
	})
	return allowed, err
}
func (r *BillingRepository) ApplyEvent(c context.Context, id, typ, t, cus, subID, priceID string, start, end time.Time) error {
	return r.knowledge.withDB(c, func(db *gorm.DB) error {
		return db.Transaction(func(tx *gorm.DB) error {
			event := model.BillingEvent{ID: id, Type: typ, ProcessedAt: time.Now().UTC()}
			result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&event)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return nil
			}
			var plan model.Plan
			if e := tx.Where("stripe_price_id = ?", priceID).First(&plan).Error; e != nil {
				return e
			}
			status := "active"
			if typ == "customer.subscription.deleted" {
				status = "canceled"
			}
			now := time.Now().UTC()
			subscription := model.Subscription{ID: newUUID(), TenantID: t, PlanID: plan.ID, Provider: "stripe", ProviderCustomerID: cus, ProviderSubscriptionID: subID, Status: status, PeriodStart: start, PeriodEnd: end, CreatedAt: now, UpdatedAt: now}
			return tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "tenant_id"}},
				DoUpdates: clause.Assignments(map[string]any{"plan_id": plan.ID, "provider": "stripe", "provider_customer_id": cus, "provider_subscription_id": subID, "status": status, "period_start": start, "period_end": end, "updated_at": now}),
			}).Create(&subscription).Error
		})
	})
}
func monthStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}
