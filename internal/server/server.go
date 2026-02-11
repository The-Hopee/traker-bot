package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"habit-tracker-bot/internal/domain"
	"habit-tracker-bot/internal/repository"
	"habit-tracker-bot/internal/service"
	"habit-tracker-bot/internal/telegram"
)

type Server struct {
	repo       repository.Repository
	tinkoffSvc *service.TinkoffService
	handlers   *telegram.Handlers
	port       string
}

func NewServer(repo repository.Repository, tinkoffSvc *service.TinkoffService, handlers *telegram.Handlers, port string) *Server {
	return &Server{repo: repo, tinkoffSvc: tinkoffSvc, handlers: handlers, port: port}
}

func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.healthHandler)
	mux.HandleFunc("/tinkoff/webhook", s.tinkoffWebhookHandler)

	server := &http.Server{Addr: ":" + s.port, Handler: mux}

	go func() {
		<-ctx.Done()
		server.Shutdown(context.Background())
	}()

	log.Printf("HTTP server started on port %s", s.port)
	return server.ListenAndServe()
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (s *Server) tinkoffWebhookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var notification domain.TinkoffNotification
	if err := json.NewDecoder(r.Body).Decode(&notification); err != nil {
		log.Printf("Error decoding webhook: %v", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	log.Printf("Tinkoff webhook: OrderId=%s, Status=%s", notification.OrderId, notification.Status)

	ctx := context.Background()
	if err := s.tinkoffSvc.ProcessNotification(ctx, &notification); err != nil {
		log.Printf("Error processing notification: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	if notification.Status == "CONFIRMED" {
		payment, err := s.tinkoffSvc.GetPaymentByOrderID(ctx, notification.OrderId)
		if err == nil {
			user, err := s.repo.GetUserByID(ctx, payment.UserID)
			if err == nil {
				s.handlers.NotifyPaymentSuccess(user.TelegramID)
			}
		}
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "OK")
}
