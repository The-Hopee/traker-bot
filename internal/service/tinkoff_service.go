package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"

	"habit-tracker-bot/internal/domain"
	"habit-tracker-bot/internal/repository"
)

const (
	TinkoffAPIURL     = "https://securepay.tinkoff.ru/v2"
	TinkoffTestAPIURL = "https://rest-api-test.tinkoff.ru/v2"
)

type TinkoffService struct {
	repo        repository.Repository
	terminalKey string
	password    string
	testMode    bool
	httpClient  *http.Client
}

func NewTinkoffService(repo repository.Repository, terminalKey, password string, testMode bool) *TinkoffService {
	return &TinkoffService{
		repo:        repo,
		terminalKey: terminalKey,
		password:    password,
		testMode:    testMode,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (s *TinkoffService) getAPIURL() string {
	if s.testMode {
		return TinkoffTestAPIURL
	}
	return TinkoffAPIURL
}

func (s *TinkoffService) IsConfigured() bool {
	return s.terminalKey != "" && s.password != ""
}

func (s *TinkoffService) CreatePayment(ctx context.Context, telegramID int64, baseAmount int64, originalAmount int64, discount int, description string) (*domain.Payment, error) {
	// Проверяем промокод
	promo, _ := s.repo.GetUserActivePromocode(ctx, telegramID)

	finalAmount := baseAmount
	discountPercent := 0

	if promo != nil {
		discountPercent = promo.DiscountPercent
		finalAmount = baseAmount * int64(100-discountPercent) / 100
	}
	orderID := uuid.New().String()

	params := map[string]string{
		"TerminalKey": s.terminalKey,
		"Amount":      fmt.Sprintf("%d", finalAmount), // уже со скидкой
		"OrderId":     orderID,
		"Description": description,
	}
	token := domain.GenerateTinkoffToken(params, s.password)

	req := domain.TinkoffInitRequest{
		TerminalKey: s.terminalKey,
		Amount:      finalAmount,
		OrderId:     orderID,
		Description: description,
		Token:       token,
		DATA:        &domain.TinkoffPaymentData{TelegramUserID: fmt.Sprintf("%d", telegramID)},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// 👇 ДОБАВЬ ЭТО: логируем что отправляем
	log.Printf("Tinkoff request URL: %s/Init", s.getAPIURL())
	log.Printf("Tinkoff request body: %s", string(body))
	log.Printf("Token input: TerminalKey=%s, Amount=%d, OrderId=%s, Description=%s, Password=%s",
		s.terminalKey, baseAmount, orderID, description, s.password)

	resp, err := s.httpClient.Post(s.getAPIURL()+"/Init", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	// 👇 ДОБАВЬ ЭТО: логируем статус ответа
	log.Printf("Tinkoff response status: %d", resp.StatusCode)

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// 👇 ДОБАВЬ ЭТО: логируем сырой ответ
	log.Printf("Tinkoff raw response: %s", string(respBody))

	// 👇 ДОБАВЬ ЭТО: проверяем пустой ответ
	if len(respBody) == 0 {
		return nil, fmt.Errorf("empty response from Tinkoff API")
	}

	var tinkoffResp domain.TinkoffInitResponse
	if err := json.Unmarshal(respBody, &tinkoffResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w (body: %s)", err, string(respBody))
	}

	if !tinkoffResp.Success {
		return nil, fmt.Errorf("tinkoff error: %s - %s", tinkoffResp.ErrorCode, tinkoffResp.Message)
	}

	user, err := s.repo.GetUserByTelegramID(ctx, telegramID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}

	payment := &domain.Payment{
		UserID:          user.ID,
		TinkoffID:       tinkoffResp.PaymentId,
		OrderID:         orderID,
		Amount:          finalAmount,
		OriginalAmount:  baseAmount,
		DiscountPercent: discountPercent,
		Status:          domain.PaymentStatus(tinkoffResp.Status),
		PaymentURL:      tinkoffResp.PaymentURL,
		Description:     description,
	}

	log.Printf("Tinkoff Init request params: %+v", params)
	log.Printf("Generated token: %s", token)

	if err := s.repo.CreatePayment(ctx, payment); err != nil {
		return nil, fmt.Errorf("save payment: %w", err)
	}

	// После успешного создания — отмечаем промокод как использованный
	if promo != nil {
		s.repo.IncrementPromocodeUsage(ctx, promo.ID, telegramID)
		s.repo.ClearUserActivePromocode(ctx, telegramID)
	}

	return payment, nil
}

func (s *TinkoffService) ProcessNotification(ctx context.Context, notification *domain.TinkoffNotification) error {
	if !domain.VerifyTinkoffToken(notification, s.password) {
		return fmt.Errorf("invalid token")
	}

	payment, err := s.repo.GetPaymentByOrderID(ctx, notification.OrderId)
	if err != nil {
		return fmt.Errorf("get payment: %w", err)
	}

	status := domain.PaymentStatus(notification.Status)
	tinkoffID := fmt.Sprintf("%d", notification.PaymentId)

	if err := s.repo.UpdatePaymentStatus(ctx, notification.OrderId, status, tinkoffID); err != nil {
		return fmt.Errorf("update payment status: %w", err)
	}

	if status == domain.PaymentStatusConfirmed {
		if err := s.repo.UpdatePaymentPaid(ctx, notification.OrderId, time.Now()); err != nil {
			return fmt.Errorf("update payment paid: %w", err)
		}
		if err := s.repo.AddSubscriptionDays(ctx, payment.UserID, domain.SubscriptionDays); err != nil {
			return fmt.Errorf("add subscription: %w", err)
		}
	}

	return nil
}

func (s *TinkoffService) GetPaymentByOrderID(ctx context.Context, orderID string) (*domain.Payment, error) {
	return s.repo.GetPaymentByOrderID(ctx, orderID)
}

func (s *TinkoffService) GetPaymentStatus(ctx context.Context, orderID string) (*domain.TinkoffInitResponse, error) {
	// 1. Получаем платёж, чтобы взять TinkoffID (PaymentId)
	payment, err := s.repo.GetPaymentByOrderID(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("get payment for GetState: %w", err)
	}

	if payment.TinkoffID == "" {
		return nil, fmt.Errorf("PaymentId is empty for OrderID=%s", orderID)
	}

	// 2. Генерируем токен БЕЗ пароля (для GetState)
	token := domain.GenerateTinkoffTokenForGetState(payment.TinkoffID, s.terminalKey, s.password)

	// 3. Формируем запрос
	reqBody := struct {
		TerminalKey string `json:"TerminalKey"`
		PaymentId   string `json:"PaymentId"` // ← обязательно
		Token       string `json:"Token"`
	}{
		TerminalKey: s.terminalKey,
		PaymentId:   payment.TinkoffID,
		Token:       token,
	}

	log.Printf("🔍 GetState request body (raw): %+v", reqBody)
	log.Printf("🔐 Token generated from: PaymentId=%s, Password=%s, TerminalKey=%s",
		payment.TinkoffID, s.password, s.terminalKey)
	log.Printf("📡 Sending to %s/GetState", s.getAPIURL())

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal GetState request: %w", err)
	}

	log.Printf("📤 Final JSON payload: %s", string(body))

	resp, err := s.httpClient.Post(s.getAPIURL()+"/GetState", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("send GetState request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read GetState response: %w", err)
	}

	log.Printf("Tinkoff GetState response status: %d", resp.StatusCode)
	log.Printf("Tinkoff GetState raw response: %s", string(respBody))

	var tinkoffResp domain.TinkoffInitResponse
	if err := json.Unmarshal(respBody, &tinkoffResp); err != nil {
		return nil, fmt.Errorf("unmarshal GetState response: %w (body: %s)", err, string(respBody))
	}

	if !tinkoffResp.Success {
		return nil, fmt.Errorf("tinkoff GetState error: %s - %s", tinkoffResp.ErrorCode, tinkoffResp.Message)
	}

	return &tinkoffResp, nil
}

// ProcessConfirmedPayment активирует подписку без проверки токена.
// Используется только при ручной проверке через GetState.
func (s *TinkoffService) ProcessConfirmedPayment(ctx context.Context, orderID string) error {
	payment, err := s.repo.GetPaymentByOrderID(ctx, orderID)
	if err != nil {
		return fmt.Errorf("get payment: %w", err)
	}

	now := time.Now()
	if err := s.repo.UpdatePaymentPaid(ctx, orderID, now); err != nil {
		return fmt.Errorf("update payment paid: %w", err)
	}
	if err := s.repo.AddSubscriptionDays(ctx, payment.UserID, domain.SubscriptionDays); err != nil {
		return fmt.Errorf("add subscription: %w", err)
	}

	return nil
}
