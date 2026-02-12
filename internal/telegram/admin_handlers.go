package telegram

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"habit-tracker-bot/internal/domain"
	"habit-tracker-bot/internal/repository"
	"habit-tracker-bot/internal/service"
)

type AdminState struct {
	Action string
	Data   map[string]string
}

type AdminHandlers struct {
	bot          *tgbotapi.BotAPI
	repo         repository.Repository
	broadcastSvc *service.BroadcastService
	adSvc        *service.AdService
	adminStates  map[int64]*AdminState
}

func NewAdminHandlers(
	bot *tgbotapi.BotAPI,
	repo repository.Repository,
	broadcastSvc *service.BroadcastService,
	adSvc *service.AdService,
) *AdminHandlers {
	return &AdminHandlers{
		bot:          bot,
		repo:         repo,
		broadcastSvc: broadcastSvc,
		adSvc:        adSvc,
		adminStates:  make(map[int64]*AdminState),
	}
}

func (h *AdminHandlers) HandleAdminCommand(ctx context.Context, msg *tgbotapi.Message) bool {
	isAdmin, _ := h.repo.IsAdmin(ctx, msg.From.ID)

	// Автоматически назначаем админом пользователя, чей Telegram ID
	// совпадает с ADMIN_TELEGRAM_ID из окружения. Это удобно для
	// первого запуска, когда таблица admins ещё пустая.
	if !isAdmin {
		if mainIDStr := os.Getenv("ADMIN_TELEGRAM_ID"); mainIDStr != "" {
			if mainID, err := strconv.ParseInt(mainIDStr, 10, 64); err == nil && mainID == msg.From.ID {
				_ = h.repo.AddAdmin(ctx, msg.From.ID)
				isAdmin = true
			}
		}
	}
	if !isAdmin {
		return false
	}

	if state, ok := h.adminStates[msg.From.ID]; ok {
		return h.handleAdminState(ctx, msg, state)
	}

	switch {
	case msg.Text == "/admin":
		h.showAdminMenu(msg.Chat.ID)
		return true
	case msg.Text == "/stats":
		h.showStats(ctx, msg.Chat.ID)
		return true
	case msg.Text == "/ads":
		h.showAds(ctx, msg.Chat.ID)
		return true
	case msg.Text == "/addad":
		h.startAddAd(msg.From.ID, msg.Chat.ID)
		return true
	case strings.HasPrefix(msg.Text, "/deletead "):
		h.deleteAd(ctx, msg)
		return true
	case strings.HasPrefix(msg.Text, "/togglead "):
		h.toggleAd(ctx, msg)
		return true
	case msg.Text == "/broadcasts":
		h.showBroadcasts(ctx, msg.Chat.ID)
		return true
	case msg.Text == "/newbroadcast":
		h.startNewBroadcast(msg.From.ID, msg.Chat.ID)
		return true
	case strings.HasPrefix(msg.Text, "/startbroadcast "):
		h.startBroadcast(ctx, msg)
		return true
	case msg.Text == "/stopbroadcast":
		h.stopBroadcast(msg.Chat.ID)
		return true
	case msg.Text == "/resumebroadcast":
		h.resumeBroadcast(ctx, msg.Chat.ID)
		return true
	case msg.Text == "/promos":
		h.showPromos(ctx, msg.Chat.ID)
		return true
	case strings.HasPrefix(msg.Text, "/addpromo "):
		h.addPromo(ctx, msg)
		return true
	case strings.HasPrefix(msg.Text, "/delpromo "):
		h.deletePromo(ctx, msg)
		return true
	case strings.HasPrefix(msg.Text, "/togglepromo "):
		h.togglePromo(ctx, msg)
		return true
	}

	return false
}

func (h *AdminHandlers) showAdminMenu(chatID int64) {
	text := `🔐 *Админ-панель*

*Статистика:*
/stats - Общая статистика

*Промокоды:*
/promos - Список промокодов
/addpromo CODE СКИДКА [ЛИМИТ] - Создать
/delpromo CODE - Удалить
/togglepromo CODE - Вкл/Выкл

*Реклама:*
/ads - Список рекламы
/addad - Добавить рекламу
/deletead [id] - Удалить
/togglead [id] - Вкл/Выкл

*Рассылки:*
/broadcasts - Список рассылок
/newbroadcast - Новая рассылка
/startbroadcast [id] - Запустить
/stopbroadcast - Остановить
/resumebroadcast - Продолжить`

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	h.bot.Send(msg)
}

func (h *AdminHandlers) showStats(ctx context.Context, chatID int64) {
	totalUsers, _ := h.repo.GetTotalUsersCount(ctx)

	text := fmt.Sprintf(`📊 *Статистика*

👥 Всего пользователей: *%d*`, totalUsers)

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	h.bot.Send(msg)
}

func (h *AdminHandlers) showAds(ctx context.Context, chatID int64) {
	ads, _ := h.repo.GetAllAds(ctx)

	if len(ads) == 0 {
		h.bot.Send(tgbotapi.NewMessage(chatID, "Нет рекламных объявлений"))
		return
	}

	var sb strings.Builder
	sb.WriteString("📢 *Рекламные объявления:*\n\n")

	for _, ad := range ads {
		status := "✅"
		if !ad.IsActive {
			status = "❌"
		}
		ctr := float64(0)
		if ad.ViewsCount > 0 {
			ctr = float64(ad.ClicksCount) / float64(ad.ViewsCount) * 100
		}
		sb.WriteString(fmt.Sprintf("%s *#%d* %s\n", status, ad.ID, ad.Name))
		sb.WriteString(fmt.Sprintf("   👁 %d | 👆 %d | CTR: %.1f%%\n\n", ad.ViewsCount, ad.ClicksCount, ctr))
	}

	msg := tgbotapi.NewMessage(chatID, sb.String())
	msg.ParseMode = "Markdown"
	h.bot.Send(msg)
}

func (h *AdminHandlers) startAddAd(userID int64, chatID int64) {
	h.adminStates[userID] = &AdminState{Action: "add_ad_name", Data: make(map[string]string)}
	h.bot.Send(tgbotapi.NewMessage(chatID, "📝 Введи название рекламы:"))
}
func (h *AdminHandlers) handleAdminState(ctx context.Context, msg *tgbotapi.Message, state *AdminState) bool {
	switch state.Action {
	case "add_ad_name":
		state.Data["name"] = msg.Text
		state.Action = "add_ad_text"
		h.bot.Send(tgbotapi.NewMessage(msg.Chat.ID, "📝 Введи текст рекламы (Markdown):"))
		return true

	case "add_ad_text":
		state.Data["text"] = msg.Text
		state.Action = "add_ad_button"
		h.bot.Send(tgbotapi.NewMessage(msg.Chat.ID, "📝 Введи кнопку (текст|url) или 'нет':"))
		return true

	case "add_ad_button":
		if msg.Text != "нет" && msg.Text != "-" {
			parts := strings.SplitN(msg.Text, "|", 2)
			if len(parts) == 2 {
				state.Data["button_text"] = strings.TrimSpace(parts[0])
				state.Data["button_url"] = strings.TrimSpace(parts[1])
			}
		}

		ad := &domain.Ad{
			Name:     state.Data["name"],
			Text:     state.Data["text"],
			IsActive: true,
			Priority: 1,
		}
		if bt, ok := state.Data["button_text"]; ok {
			ad.ButtonText = &bt
			bu := state.Data["button_url"]
			ad.ButtonURL = &bu
		}

		err := h.repo.CreateAd(ctx, ad)
		delete(h.adminStates, msg.From.ID)

		if err != nil {
			h.bot.Send(tgbotapi.NewMessage(msg.Chat.ID, "❌ Ошибка создания"))
		} else {
			h.adSvc.RefreshCache(ctx)
			h.bot.Send(tgbotapi.NewMessage(msg.Chat.ID, fmt.Sprintf("✅ Реклама #%d создана!", ad.ID)))
		}
		return true

	case "broadcast_name":
		state.Data["name"] = msg.Text
		state.Action = "broadcast_text"
		h.bot.Send(tgbotapi.NewMessage(msg.Chat.ID, "📝 Введи текст рассылки:"))
		return true

	case "broadcast_text":
		state.Data["text"] = msg.Text
		state.Action = "broadcast_button"
		h.bot.Send(tgbotapi.NewMessage(msg.Chat.ID, "📝 Введи кнопку (текст|url) или 'нет':"))
		return true

	case "broadcast_button":
		if msg.Text != "нет" && msg.Text != "-" {
			parts := strings.SplitN(msg.Text, "|", 2)
			if len(parts) == 2 {
				state.Data["button_text"] = strings.TrimSpace(parts[0])
				state.Data["button_url"] = strings.TrimSpace(parts[1])
			}
		}

		b := &domain.Broadcast{
			Name:   state.Data["name"],
			Text:   state.Data["text"],
			Status: domain.BroadcastDraft,
		}
		if bt, ok := state.Data["button_text"]; ok {
			b.ButtonText = &bt
			bu := state.Data["button_url"]
			b.ButtonURL = &bu
		}

		err := h.repo.CreateBroadcast(ctx, b)
		delete(h.adminStates, msg.From.ID)

		if err != nil {
			h.bot.Send(tgbotapi.NewMessage(msg.Chat.ID, "❌ Ошибка создания"))
		} else {
			h.bot.Send(tgbotapi.NewMessage(msg.Chat.ID, fmt.Sprintf("✅ Рассылка #%d создана!\n\nЗапустить: /startbroadcast %d", b.ID, b.ID)))
		}
		return true
	}

	return false
}

func (h *AdminHandlers) deleteAd(ctx context.Context, msg *tgbotapi.Message) {
	id, _ := strconv.ParseInt(strings.TrimPrefix(msg.Text, "/deletead "), 10, 64)
	h.repo.DeleteAd(ctx, id)
	h.adSvc.RefreshCache(ctx)
	h.bot.Send(tgbotapi.NewMessage(msg.Chat.ID, fmt.Sprintf("✅ Реклама #%d удалена", id)))
}

func (h *AdminHandlers) toggleAd(ctx context.Context, msg *tgbotapi.Message) {
	id, _ := strconv.ParseInt(strings.TrimPrefix(msg.Text, "/togglead "), 10, 64)
	ad, err := h.repo.GetAdByID(ctx, id)
	if err != nil {
		h.bot.Send(tgbotapi.NewMessage(msg.Chat.ID, "❌ Не найдено"))
		return
	}
	ad.IsActive = !ad.IsActive
	h.repo.UpdateAd(ctx, ad)
	h.adSvc.RefreshCache(ctx)

	status := "включена ✅"
	if !ad.IsActive {
		status = "выключена ❌"
	}
	h.bot.Send(tgbotapi.NewMessage(msg.Chat.ID, fmt.Sprintf("Реклама #%d %s", id, status)))
}

func (h *AdminHandlers) showBroadcasts(ctx context.Context, chatID int64) {
	broadcasts, _ := h.repo.GetAllBroadcasts(ctx)

	if len(broadcasts) == 0 {
		h.bot.Send(tgbotapi.NewMessage(chatID, "Нет рассылок"))
		return
	}

	var sb strings.Builder
	sb.WriteString("📬 *Рассылки:*\n\n")
	for _, b := range broadcasts {
		status := "📝"
		switch b.Status {
		case domain.BroadcastRunning:
			status = "▶️"
		case domain.BroadcastPaused:
			status = "⏸️"
		case domain.BroadcastCompleted:
			status = "✅"
		}

		progress := ""
		if b.TotalUsers > 0 {
			progress = fmt.Sprintf(" (%d/%d)", b.SentCount, b.TotalUsers)
		}
		sb.WriteString(fmt.Sprintf("%s *#%d* %s%s\n", status, b.ID, b.Name, progress))
	}

	msg := tgbotapi.NewMessage(chatID, sb.String())
	msg.ParseMode = "Markdown"
	h.bot.Send(msg)
}

func (h *AdminHandlers) startNewBroadcast(userID int64, chatID int64) {
	h.adminStates[userID] = &AdminState{Action: "broadcast_name", Data: make(map[string]string)}
	h.bot.Send(tgbotapi.NewMessage(chatID, "📝 Введи название рассылки:"))
}

func (h *AdminHandlers) startBroadcast(ctx context.Context, msg *tgbotapi.Message) {
	id, _ := strconv.ParseInt(strings.TrimPrefix(msg.Text, "/startbroadcast "), 10, 64)

	if h.broadcastSvc.IsRunning() {
		h.bot.Send(tgbotapi.NewMessage(msg.Chat.ID, "❌ Уже есть запущенная рассылка"))
		return
	}

	if err := h.broadcastSvc.StartBroadcast(ctx, id); err != nil {
		h.bot.Send(tgbotapi.NewMessage(msg.Chat.ID, "❌ "+err.Error()))
		return
	}

	h.bot.Send(tgbotapi.NewMessage(msg.Chat.ID, fmt.Sprintf("▶️ Рассылка #%d запущена!", id)))
}

func (h *AdminHandlers) stopBroadcast(chatID int64) {
	if !h.broadcastSvc.IsRunning() {
		h.bot.Send(tgbotapi.NewMessage(chatID, "❌ Нет активной рассылки"))
		return
	}
	h.broadcastSvc.StopBroadcast()
	h.bot.Send(tgbotapi.NewMessage(chatID, "⏸️ Рассылка остановлена"))
}

func (h *AdminHandlers) resumeBroadcast(ctx context.Context, chatID int64) {
	if err := h.broadcastSvc.ResumeBroadcast(ctx); err != nil {
		h.bot.Send(tgbotapi.NewMessage(chatID, "❌ "+err.Error()))
		return
	}
	h.bot.Send(tgbotapi.NewMessage(chatID, "▶️ Рассылка продолжена"))
}

func (h *AdminHandlers) showPromos(ctx context.Context, chatID int64) {
	promos, _ := h.repo.GetAllPromocodes(ctx)

	if len(promos) == 0 {
		h.bot.Send(tgbotapi.NewMessage(chatID, "Нет промокодов"))
		return
	}

	var sb strings.Builder
	sb.WriteString("🎟 *Промокоды:*\n\n")
	for _, p := range promos {
		status := "✅"
		if !p.IsActive {
			status = "❌"
		}
		sb.WriteString(fmt.Sprintf("%s %s — %d%%", status, p.Code, p.DiscountPercent))
		sb.WriteString(fmt.Sprintf(" (исп: %d", p.UsedCount))
		if p.MaxUses != nil {
			sb.WriteString(fmt.Sprintf("/%d", *p.MaxUses))
		}
		sb.WriteString(")\n")
	}

	msg := tgbotapi.NewMessage(chatID, sb.String())
	msg.ParseMode = "Markdown"
	h.bot.Send(msg)
}

func (h *AdminHandlers) addPromo(ctx context.Context, msg *tgbotapi.Message) {
	// /addpromo CODE 50 20
	parts := strings.Fields(msg.Text)
	if len(parts) < 3 {
		h.bot.Send(tgbotapi.NewMessage(msg.Chat.ID,
			"Формат: /addpromo CODE СКИДКА [ЛИМИТ]\nПример: /addpromo EARLYBIRD 50 20"))
		return
	}

	code := strings.ToUpper(parts[1])
	discount, err := strconv.Atoi(parts[2])
	if err != nil || discount < 1 || discount > 100 {
		h.bot.Send(tgbotapi.NewMessage(msg.Chat.ID, "❌ Скидка от 1 до 100"))
		return
	}

	maxUses := 0
	if len(parts) >= 4 {
		maxUses, _ = strconv.Atoi(parts[3])
	}

	err = h.repo.CreatePromocode(ctx, code, discount, maxUses)
	if err != nil {
		h.bot.Send(tgbotapi.NewMessage(msg.Chat.ID, "❌ Ошибка: "+err.Error()))
		return
	}

	text := fmt.Sprintf("✅ Промокод создан\n\nКод: `%s`\nСкидка: %d%%", code, discount)
	if maxUses > 0 {
		text += fmt.Sprintf("\nЛимит: %d", maxUses)
	}

	m := tgbotapi.NewMessage(msg.Chat.ID, text)
	m.ParseMode = "Markdown"
	h.bot.Send(m)
}

func (h *AdminHandlers) deletePromo(ctx context.Context, msg *tgbotapi.Message) {
	code := strings.ToUpper(strings.TrimPrefix(msg.Text, "/delpromo "))
	h.repo.DeletePromocode(ctx, code)
	h.bot.Send(tgbotapi.NewMessage(msg.Chat.ID, fmt.Sprintf("✅ Промокод %s удалён", code)))
}

func (h *AdminHandlers) togglePromo(ctx context.Context, msg *tgbotapi.Message) {
	code := strings.ToUpper(strings.TrimPrefix(msg.Text, "/togglepromo "))
	h.repo.TogglePromocode(ctx, code)
	h.bot.Send(tgbotapi.NewMessage(msg.Chat.ID, fmt.Sprintf("✅ Промокод %s переключён", code)))
}
