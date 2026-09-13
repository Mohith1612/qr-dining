//go:build integration

package services_test

import (
	"github.com/Mohith1612/qr-dining/internal/events"
	"github.com/Mohith1612/qr-dining/internal/observability"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/rs/zerolog"
)

func testMetrics() *observability.Metrics {
	return observability.NewMetrics()
}

func newTestSessionService(repos *repository.Repos, pub *events.Publisher) *services.SessionService {
	return services.NewSessionService(repos, pub, testMetrics(), nil)
}

func newTestOrderService(repos *repository.Repos, pub *events.Publisher) *services.OrderService {
	promoSvc := services.NewPromoService(repos)
	svc := services.NewOrderService(repos, pub, testMetrics(), promoSvc)
	svc.SetHostAuthority(newTestSessionService(repos, pub))
	return svc
}

func newTestAssistanceService(repos *repository.Repos, pub *events.Publisher) *services.AssistanceService {
	return services.NewAssistanceService(repos, pub)
}

func newTestPaymentService(repos *repository.Repos, pub *events.Publisher, sessionSvc *services.SessionService) *services.PaymentService {
	svc := services.NewPaymentService(repos, pub, testMetrics(), sessionSvc, zerolog.Nop())
	svc.SetHostAuthority(sessionSvc)
	svc.SetPromoService(services.NewPromoService(repos))
	return svc
}
