package services_test

import (
	"context"
	"errors"
	"testing"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/policies"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

type recordingChannel struct {
	delivered []entities.AlertNotice
	err       error
}

func (c *recordingChannel) DeliverNotice(_ context.Context, n entities.AlertNotice) error {
	c.delivered = append(c.delivered, n)
	return c.err
}

func testLogger() *logger.Logger { return logger.New("NotificationsTest", "Error") }

func pushable() entities.AlertNotice {
	return entities.AlertNotice{MessageID: 7, AlertCode: "ALR-000142", Code: "OPEN", Level: entities.LevelAmarillo}
}

func TestNoticeDispatcher_DeliversWhatThePolicyAllows(t *testing.T) {
	target := &recordingChannel{}
	services.NewNoticeDispatcher("push", policies.MobilePushPolicy{}, target, testLogger()).
		Dispatch(context.Background(), pushable())
	if len(target.delivered) != 1 || target.delivered[0].MessageID != 7 {
		t.Fatalf("entregados = %+v", target.delivered)
	}
}

func TestNoticeDispatcher_SkipsWhatThePolicyRejects(t *testing.T) {
	target := &recordingChannel{}
	late := pushable()
	late.Late = true
	services.NewNoticeDispatcher("push", policies.MobilePushPolicy{}, target, testLogger()).
		Dispatch(context.Background(), late)
	if len(target.delivered) != 0 {
		t.Fatalf("entregados = %+v: el push omite las horas atrasadas", target.delivered)
	}
}

func TestNoticeDispatcher_DeliveryErrorDoesNotPanic(t *testing.T) {
	target := &recordingChannel{err: errors.New("canal caído")}
	services.NewNoticeDispatcher("web", policies.WebPolicy{}, target, testLogger()).
		Dispatch(context.Background(), pushable())
	if len(target.delivered) != 1 {
		t.Fatalf("entregados = %+v", target.delivered)
	}
}
