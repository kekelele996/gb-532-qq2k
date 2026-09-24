package service

import (
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"

	"sonar-survey-coverage-planner/backend/internal/model"
	"sonar-survey-coverage-planner/backend/internal/repository"
)

type Actor struct {
	RequestID string
	UserID    uint
	Username  string
	Role      string
}

type AuditService struct{ repository *repository.SupportRepository }

func NewAuditService(repository *repository.SupportRepository) *AuditService {
	return &AuditService{repository: repository}
}

func (s *AuditService) Record(actor Actor, action, entityType string, entityID uint, before, after, metadata any) error {
	event, err := s.buildEvent(actor, action, entityType, entityID, before, after, metadata)
	if err != nil {
		return err
	}
	return s.repository.CreateAudit(&event)
}

// RecordTx 让审计事件与业务状态迁移共用同一事务。
func (s *AuditService) RecordTx(tx *gorm.DB, actor Actor, action, entityType string, entityID uint, before, after, metadata any) error {
	event, err := s.buildEvent(actor, action, entityType, entityID, before, after, metadata)
	if err != nil {
		return err
	}
	if err := tx.Create(&event).Error; err != nil {
		return fmt.Errorf("create transactional audit event: %w", err)
	}
	return nil
}

func (s *AuditService) buildEvent(actor Actor, action, entityType string, entityID uint, before, after, metadata any) (model.AuditEvent, error) {
	beforeJSON, err := encodeSnapshot(before)
	if err != nil {
		return model.AuditEvent{}, fmt.Errorf("encode audit before snapshot: %w", err)
	}
	afterJSON, err := encodeSnapshot(after)
	if err != nil {
		return model.AuditEvent{}, fmt.Errorf("encode audit after snapshot: %w", err)
	}
	metadataJSON, err := encodeSnapshot(metadata)
	if err != nil {
		return model.AuditEvent{}, fmt.Errorf("encode audit metadata: %w", err)
	}
	return model.AuditEvent{RequestID: actor.RequestID, UserID: actor.UserID, Actor: actor.Username, Role: actor.Role, Action: action, EntityType: entityType, EntityID: entityID, BeforeJSON: beforeJSON, AfterJSON: afterJSON, Metadata: metadataJSON, CreatedAt: time.Now().UTC()}, nil
}

func (s *AuditService) List(filter repository.AuditFilter) ([]model.AuditEvent, int64, error) {
	return s.repository.ListAudits(filter)
}

func (s *AuditService) Ready() error { return s.repository.Ping() }

func encodeSnapshot(value any) (string, error) {
	if value == nil {
		return "{}", nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}
