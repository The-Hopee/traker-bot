package telegram

import (
	"context"
	"errors"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"habit-tracker-bot/internal/domain"
	"habit-tracker-bot/internal/repository"
	"habit-tracker-bot/internal/service"
)

const (
	StateNone                = "none"
	StateWaitingHabitName    = "waiting_habit_name"
	StateWaitingReminderMode = "waiting_reminder_mode"
	StateWaitingCustomTime   = "waiting_custom_time"
	StateWaitingReminderDays = "waiting_reminder_days"
	StateWaitingCustomDays   = "waiting_custom_days"
)

type UserState struct {
	State        string
	HabitName    string
	Frequency    string
	ReminderTime string
	SelectedDays map[int]bool
}

type Handlers struct {
	bot            *tgbotapi.BotAPI
	repo           repository.Repository
	habitSvc       *service.HabitService
	subSvc         *service.SubscriptionService
	referralSvc    *service.ReferralService
	achievementSvc *service.AchievementService
	tinkoffSvc     *service.TinkoffService
	adSvc          *service.AdService
	exportSvc      *service.ExportService
	adminHandlers  *AdminHandlers
	userStates     map[int64]*UserState
	botUsername    string
	subPrice       int64
}

func NewHandlers(
	bot *tgbotapi.BotAPI,
	repo repository.Repository,
	habitSvc *service.HabitService,
	subSvc *service.SubscriptionService,
	referralSvc *service.ReferralService,
	achievementSvc *service.AchievementService,
	tinkoffSvc *service.TinkoffService,
	adSvc *service.AdService,
	exportSvc *service.ExportService,
	botUsername string,
	subPrice int64,
) *Handlers {
	h := &Handlers{
		bot:            bot,
		repo:           repo,
		habitSvc:       habitSvc,
		subSvc:         subSvc,
		referralSvc:    referralSvc,
		achievementSvc: achievementSvc,
		tinkoffSvc:     tinkoffSvc,
		adSvc:          adSvc,
		exportSvc:      exportSvc,
		userStates:     make(map[int64]*UserState),
		botUsername:    botUsername,
		subPrice:       subPrice,
	}
	return h
}

func (h *Handlers) SetAdminHandlers(ah *AdminHandlers) {
	h.adminHandlers = ah
}

func (h *Handlers) HandleUpdate(update tgbotapi.Update) {
	ctx := context.Background()

	if update.Message != nil {
		h.handleMessage(ctx, update.Message)
	} else if update.CallbackQuery != nil {
		h.handleCallback(ctx, update.CallbackQuery)
	}
}

func (h *Handlers) handleMessage(ctx context.Context, msg *tgbotapi.Message) {
	// Проверяем админ-команды
	if h.adminHandlers != nil && h.adminHandlers.HandleAdminCommand(ctx, msg) {
		return
	}

	// Проверяем реферальный код
	var referralCode string
	if strings.HasPrefix(msg.Text, "/start ref_") {
		referralCode = strings.TrimPrefix(msg.Text, "/start ref_")
	}

	// Регистрируем пользователя
	user := &domain.User{
		TelegramID:   msg.From.ID,
		Username:     msg.From.UserName,
		FirstName:    msg.From.FirstName,
		Timezone:     "Europe/Moscow",
		ReferralCode: domain.GenerateReferralCode(),
	}

	existingUser, err := h.repo.GetUserByTelegramID(ctx, msg.From.ID)
	isNewUser := errors.Is(err, repository.ErrNotFound)

	if err := h.repo.CreateUser(ctx, user); err != nil {
		log.Printf("Error creating user: %v", err)
	}

	// Обрабатываем реферал для нового пользователя
	if isNewUser && referralCode != "" {
		user, _ = h.repo.GetUserByTelegramID(ctx, msg.From.ID)
		if user != nil {
			result, err := h.referralSvc.ProcessReferralStage1(ctx, referralCode, user)
			if err != nil {
				log.Printf("Error processing referral: %v", err)
			} else if result != nil {
				h.sendReferralWelcome(ctx, msg.Chat.ID, user, result)
				h.notifyReferrerStage1(ctx, result, user.FirstName)
				return
			}
		}
	}

	if existingUser != nil {
		user = existingUser
	}

	// Проверяем состояние пользователя
	if state, ok := h.userStates[msg.From.ID]; ok {
		h.handleUserState(ctx, msg, state)
		return
	}
	// Обработка команд
	switch {
	case msg.Text == "/start" || strings.HasPrefix(msg.Text, "/start "):
		h.handleStart(ctx, msg)
	case msg.Text == "📋 Мои привычки" || msg.Text == "/habits":
		h.handleHabits(ctx, msg)
	case msg.Text == "➕ Новая привычка" || msg.Text == "/new":
		h.handleNewHabit(ctx, msg)
	case msg.Text == "📊 Статистика" || msg.Text == "/stats":
		h.handleStats(ctx, msg)
	case msg.Text == "✅ Отметить сегодня" || msg.Text == "/today":
		h.handleToday(ctx, msg)
	case msg.Text == "🏆 Достижения" || msg.Text == "/achievements":
		h.handleAchievements(ctx, msg)
	case msg.Text == "👥 Рефералы" || msg.Text == "/referral":
		h.handleReferral(ctx, msg)
	case msg.Text == "⭐️ Premium" || msg.Text == "/premium":
		h.handlePremium(ctx, msg)
	case msg.Text == "❓ Помощь" || msg.Text == "/help":
		h.handleHelp(ctx, msg)
	case strings.HasPrefix(msg.Text, "/promo "):
		code := strings.ToUpper(strings.TrimSpace(strings.TrimPrefix(msg.Text, "/promo ")))
		h.applyPromocode(ctx, msg.Chat.ID, msg.From.ID, code)
	default:
		h.handleUnknown(ctx, msg)
	}
}

func (h *Handlers) sendReferralWelcome(ctx context.Context, chatID int64, user *domain.User, result *service.ReferralResult) {
	referrer, _ := h.repo.GetUserByID(ctx, result.ReferrerUserID)
	referrerName := "друга"
	if referrer != nil && referrer.FirstName != "" {
		referrerName = referrer.FirstName
	}

	text := fmt.Sprintf(`🎉 *Добро пожаловать!*

Ты пришёл по приглашению от *%s*!

🎁 *Этап 1 выполнен!*
+%d дня Premium тебе!

💡 *Этап 2:*
Отмечай привычки %d дней подряд и получи ещё +%d дней Premium!

Начни формировать полезные привычки прямо сейчас!`,
		referrerName, result.ReferredBonus, domain.ReferralStage2Streak, domain.ReferralStage2Bonus)

	reply := tgbotapi.NewMessage(chatID, text)
	reply.ParseMode = "Markdown"
	reply.ReplyMarkup = MainMenuKeyboard()
	h.bot.Send(reply)
}

func (h *Handlers) notifyReferrerStage1(ctx context.Context, result *service.ReferralResult, referredName string) {
	referrer, err := h.repo.GetUserByID(ctx, result.ReferrerUserID)
	if err != nil {
		return
	}

	var text string
	if result.IsDiscount {
		text = fmt.Sprintf(`🎉 *Новый реферал!*

По твоей ссылке пришёл *%s*!

🎁 Твоя скидка увеличена на *%d%%*!

💡 Когда %s достигнет %d дней серии — он получит ещё бонус!`,
			referredName, result.ReferrerBonus, referredName, domain.ReferralStage2Streak)
	} else {
		text = fmt.Sprintf(`🎉 *Новый реферал!*

По твоей ссылке пришёл *%s*!

🎁 *Этап 1:* +%d дня Premium!

💡 Когда %s достигнет %d дней серии — вы оба получите ещё +%d дней!`,
			referredName, result.ReferrerBonus, referredName, domain.ReferralStage2Streak, domain.ReferralStage2Bonus)
	}

	msg := tgbotapi.NewMessage(referrer.TelegramID, text)
	msg.ParseMode = "Markdown"
	h.bot.Send(msg)
}

func (h *Handlers) handleStart(ctx context.Context, msg *tgbotapi.Message) {
	text := fmt.Sprintf(`👋 Привет, *%s*!

Я помогу тебе сформировать полезные привычки и отслеживать прогресс.

🎯 *Что я умею:*
• Создавать и отслеживать привычки
• Напоминать о выполнении (Premium)
• Показывать статистику и серии

📌 Нажми "➕ Новая привычка" чтобы начать!

🆓 *Бесплатно:* до 3 привычек
⭐️ *Premium:* безлимит + напоминания + без рекламы

👥 *Реферальная программа:*
Отмечай привычки 7 дней подряд и приглашай друзей!`, msg.From.FirstName)

	reply := tgbotapi.NewMessage(msg.Chat.ID, text)
	reply.ParseMode = "Markdown"
	reply.ReplyMarkup = MainMenuKeyboard()
	h.bot.Send(reply)
}

func (h *Handlers) handleHabits(ctx context.Context, msg *tgbotapi.Message) {
	user, err := h.repo.GetUserByTelegramID(ctx, msg.From.ID)
	if err != nil {
		h.sendError(msg.Chat.ID, "Ошибка получения данных")
		return
	}

	habits, _ := h.habitSvc.GetUserHabits(ctx, user.ID)
	completedToday, _ := h.habitSvc.GetTodayStatus(ctx, user.ID)

	text := "📋 *Мои привычки*\n\n"
	if len(habits) == 0 {
		text += "У тебя пока нет привычек. Создай первую!"
	} else {
		text += "Выбери привычку для подробностей:"
	}

	reply := tgbotapi.NewMessage(msg.Chat.ID, text)
	reply.ParseMode = "Markdown"
	reply.ReplyMarkup = HabitsListKeyboard(habits, completedToday)
	h.bot.Send(reply)

	h.maybeShowAd(ctx, msg.Chat.ID, user.ID)
}

func (h *Handlers) handleNewHabit(ctx context.Context, msg *tgbotapi.Message) {
	user, err := h.repo.GetUserByTelegramID(ctx, msg.From.ID)
	if err != nil {
		h.sendError(msg.Chat.ID, "Ошибка получения данных")
		return
	}

	count, _ := h.repo.CountUserHabits(ctx, user.ID)
	limit := domain.FreeHabitsLimit
	if user.HasActiveSubscription() {
		limit = domain.PremiumHabitsLimit
	}

	if count >= limit {
		text := fmt.Sprintf(`⚠️ *Достигнут лимит привычек*
  
  У тебя уже %d из %d привычек.
  
  Оформи Premium или пригласи друзей!`, count, limit)

		reply := tgbotapi.NewMessage(msg.Chat.ID, text)
		reply.ParseMode = "Markdown"
		reply.ReplyMarkup = PremiumKeyboard("", user.DiscountPercent)
		h.bot.Send(reply)
		return
	}

	h.userStates[msg.From.ID] = &UserState{State: "awaiting_name"}

	text := "➕ *Новая привычка*\n\nВведи название привычки:"
	reply := tgbotapi.NewMessage(msg.Chat.ID, text)
	reply.ParseMode = "Markdown"
	reply.ReplyMarkup = CancelKeyboard()
	h.bot.Send(reply)
}

func (h *Handlers) handleUserState(ctx context.Context, msg *tgbotapi.Message, state *UserState) {
	switch state.State {
	case "awaiting_name":
		if len(msg.Text) > 100 {
			h.sendError(msg.Chat.ID, "Название слишком длинное (макс. 100 символов)")
			return
		}
		state.HabitName = msg.Text
		state.State = "awaiting_frequency"

		text := fmt.Sprintf("📝 Привычка: *%s*\n\nВыбери периодичность:", state.HabitName)
		reply := tgbotapi.NewMessage(msg.Chat.ID, text)
		reply.ParseMode = "Markdown"
		reply.ReplyMarkup = FrequencyKeyboard()
		h.bot.Send(reply)

	case StateWaitingCustomTime:
		matched, _ := regexp.MatchString(`^\d{1,2}:\d{2}$`, msg.Text)
		if !matched {
			h.sendMessage(msg.Chat.ID, "❌ Введи время в формате ЧЧ:ММ (например 08:30):")
			return
		}
		state.ReminderTime = msg.Text
		state.State = StateWaitingReminderDays

		keyboard := ReminderDaysKeyboard()
		reply := tgbotapi.NewMessage(msg.Chat.ID, "📅 В какие дни напоминать?")
		reply.ReplyMarkup = keyboard
		h.bot.Send(reply)
	}
}

func (h *Handlers) handleToday(ctx context.Context, msg *tgbotapi.Message) {
	user, err := h.repo.GetUserByTelegramID(ctx, msg.From.ID)
	if err != nil {
		h.sendError(msg.Chat.ID, "Ошибка получения данных")
		return
	}

	habits, _ := h.habitSvc.GetUserHabits(ctx, user.ID)
	if len(habits) == 0 {
		h.sendMessage(msg.Chat.ID, "У тебя пока нет привычек. Создай первую!")
		return
	}

	completedToday, _ := h.habitSvc.GetTodayStatus(ctx, user.ID)
	completed := 0
	for _, done := range completedToday {
		if done {
			completed++
		}
	}

	streak, _ := h.habitSvc.GetUserOverallStreak(ctx, user.ID)

	text := fmt.Sprintf("✅ *Сегодняшний прогресс*\n\nВыполнено: %d из %d\n🔥 Серия: %d дн.", completed, len(habits), streak)

	reply := tgbotapi.NewMessage(msg.Chat.ID, text)
	reply.ParseMode = "Markdown"
	reply.ReplyMarkup = TodayChecklistKeyboard(habits, completedToday)
	h.bot.Send(reply)
}

func (h *Handlers) handleStats(ctx context.Context, msg *tgbotapi.Message) {
	user, err := h.repo.GetUserByTelegramID(ctx, msg.From.ID)
	if err != nil {
		h.sendError(msg.Chat.ID, "Ошибка получения данных")
		return
	}

	stats, _ := h.habitSvc.GetUserStats(ctx, user.ID)
	if len(stats) == 0 {
		h.sendMessage(msg.Chat.ID, "📊 *Статистика*\n\nУ тебя пока нет привычек.")
		return
	}

	overallStreak, _ := h.habitSvc.GetUserOverallStreak(ctx, user.ID)

	var sb strings.Builder
	sb.WriteString("📊 *Твоя статистика*\n\n")
	sb.WriteString(fmt.Sprintf("🔥 *Общая серия:* %d дн.\n\n", overallStreak))

	for _, s := range stats {
		emoji := "🔥"
		if s.CurrentStreak == 0 {
			emoji = "💤"
		}
		sb.WriteString(fmt.Sprintf("*%s*\n", s.HabitName))
		sb.WriteString(fmt.Sprintf("  %s Серия: %d дн. | 🏆 Лучшая: %d дн.\n", emoji, s.CurrentStreak, s.BestStreak))
		sb.WriteString(fmt.Sprintf("  📈 Выполнено: %.0f%%\n\n", s.CompletionRate))
	}

	sb.WriteString("👇 *Выбери график:*")

	reply := tgbotapi.NewMessage(msg.Chat.ID, sb.String())
	reply.ParseMode = "Markdown"
	reply.ReplyMarkup = StatsKeyboard()
	h.bot.Send(reply)

	h.maybeShowAd(ctx, msg.Chat.ID, user.ID)
}

func (h *Handlers) handleAchievements(ctx context.Context, msg *tgbotapi.Message) {
	user, err := h.repo.GetUserByTelegramID(ctx, msg.From.ID)
	if err != nil {
		h.sendError(msg.Chat.ID, "Ошибка получения данных")
		return
	}

	achievements, _ := h.achievementSvc.GetUserAchievements(ctx, user.ID)
	streak, _ := h.habitSvc.GetUserOverallStreak(ctx, user.ID)
	nextAch, daysLeft, _ := h.achievementSvc.GetNextAchievement(ctx, user.ID, streak)

	var sb strings.Builder
	sb.WriteString("🏆 *Твои достижения*\n\n")
	if len(achievements) == 0 {
		sb.WriteString("Пока нет достижений.\n\n")
	} else {
		for _, a := range achievements {
			cfg := domain.GetAchievementConfig(a.Type)
			if cfg != nil {
				bonus := ""
				if cfg.BonusDays > 0 {
					bonus = fmt.Sprintf(" (+%d дней)", cfg.BonusDays)
				}
				sb.WriteString(fmt.Sprintf("%s *%s*%s\n", cfg.Emoji, cfg.Title, bonus))
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("🔥 Текущая серия: *%d* дней\n\n", streak))

	if nextAch != nil {
		bonus := ""
		if nextAch.BonusDays > 0 {
			bonus = fmt.Sprintf(" (+%d дней Premium)", nextAch.BonusDays)
		}
		sb.WriteString(fmt.Sprintf("📍 *Следующее:* %s %s%s\n", nextAch.Emoji, nextAch.Title, bonus))
		sb.WriteString(fmt.Sprintf("   Осталось: %d дней\n", daysLeft))
	} else {
		sb.WriteString("🎊 *Все достижения получены!*\n")
	}

	sb.WriteString("\n📊 *Все достижения:*\n")
	for _, cfg := range domain.AchievementsConfig {
		has, _ := h.repo.HasAchievement(ctx, user.ID, cfg.Type)
		status := "⬜️"
		if has {
			status = "✅"
		}
		bonus := ""
		if cfg.BonusDays > 0 {
			bonus = fmt.Sprintf(" +%dд", cfg.BonusDays)
		}
		sb.WriteString(fmt.Sprintf("%s %s (%d дней)%s\n", status, cfg.Title, cfg.StreakDays, bonus))
	}

	reply := tgbotapi.NewMessage(msg.Chat.ID, sb.String())
	reply.ParseMode = "Markdown"
	h.bot.Send(reply)
}

func (h *Handlers) handleReferral(ctx context.Context, msg *tgbotapi.Message) {
	user, err := h.repo.GetUserByTelegramID(ctx, msg.From.ID)
	if err != nil {
		h.sendError(msg.Chat.ID, "Ошибка получения данных")
		return
	}

	stats, _ := h.referralSvc.GetReferralStats(ctx, user.ID)

	if !stats.CanInvite {
		text := fmt.Sprintf(`👥 *Реферальная программа*
	
	🔒 *Пока заблокировано*
	
	Выполняй все привычки %d дней подряд!
	
	📊 *Прогресс:* %d из %d дней
	
	🎁 *За первых %d друзей:*
	• Этап 1: +%d дня (регистрация)
	• Этап 2: +%d дня (7 дней серии)
	
	🎁 *После %d друзей:*
	• Скидка %d%% за каждого (до %d%%)`,
			domain.ReferralUnlockStreak, stats.CurrentStreak, domain.ReferralUnlockStreak,
			domain.ReferralBonusLimit, domain.ReferralStage1Bonus, domain.ReferralStage2Bonus,
			domain.ReferralBonusLimit, domain.ReferralDiscountPerRef, domain.MaxReferralDiscount)

		reply := tgbotapi.NewMessage(msg.Chat.ID, text)
		reply.ParseMode = "Markdown"
		reply.ReplyMarkup = ReferralLockedKeyboard()
		h.bot.Send(reply)
		return
	}

	referralLink, _ := h.referralSvc.GetReferralLink(ctx, user.ID, h.botUsername)
	remainingBonus := domain.ReferralBonusLimit - stats.BonusReferrals

	var bonusStatus string
	if remainingBonus > 0 {
		bonusStatus = fmt.Sprintf("📦 Осталось бонусных слотов: %d", remainingBonus)
	} else {
		bonusStatus = fmt.Sprintf("💰 За новых друзей — скидка %d%%", domain.ReferralDiscountPerRef)
	}

	discountInfo := ""
	if stats.AccumulatedDiscount > 0 {
		discountInfo = fmt.Sprintf("\n💳 Накопленная скидка: *%d%%*", stats.AccumulatedDiscount)
	}

	text := fmt.Sprintf(`👥 *Реферальная программа*
	
	🎉 *Разблокировано!*
	
	📊 *Статистика:*
	• Приглашено: %d
	• С бонусом: %d | Со скидкой: %d
	• Получено дней: %d%s
	
	%s
	
	🔗 *Твоя ссылка:*
	`+"`%s`",
		stats.TotalReferrals, stats.BonusReferrals, stats.DiscountReferrals,
		stats.TotalBonusDays, discountInfo, bonusStatus, referralLink)

	reply := tgbotapi.NewMessage(msg.Chat.ID, text)
	reply.ParseMode = "Markdown"
	reply.ReplyMarkup = ReferralKeyboard(referralLink)
	h.bot.Send(reply)
}

func (h *Handlers) handlePremium(ctx context.Context, msg *tgbotapi.Message) {
	user, err := h.repo.GetUserByTelegramID(ctx, msg.From.ID)
	if err != nil {
		h.sendError(msg.Chat.ID, "Ошибка получения данных")
		return
	}

	if user.HasActiveSubscription() {
		text := fmt.Sprintf(`⭐️ *Premium активен*
	
	Подписка до: *%s*
	
	✅ Безлимитные привычки
	✅ Напоминания о привычках
	✅ Статистика за год
	✅ Экспорт данных
	✅ Без рекламы`, user.SubscriptionEnd.Format("02.01.2006"))
		reply := tgbotapi.NewMessage(msg.Chat.ID, text)
		reply.ParseMode = "Markdown"
		reply.ReplyMarkup = PremiumActiveKeyboard()
		h.bot.Send(reply)
		return
	}

	// Проверяем промокод
	promo, _ := h.repo.GetUserActivePromocode(ctx, msg.From.ID)

	// Берём максимальную скидку: реферальную или промокод
	discount := user.DiscountPercent
	promoText := ""

	if promo != nil && promo.DiscountPercent > discount {
		discount = promo.DiscountPercent
		promoText = fmt.Sprintf("\n🎟 Промокод %s применён!", promo.Code)
	}

	originalPrice := float64(h.subPrice) / 100
	finalPrice := originalPrice * (1 - float64(discount)/100)

	discountText := ""
	if discount > 0 {
		discountText = fmt.Sprintf("\n\n🎁 *Твоя скидка:* %d%%%s\n💰 Цена для тебя: *%.0f₽* ~%.0f₽~",
			discount, promoText, finalPrice, originalPrice)
	}

	text := fmt.Sprintf(`⭐️ *Premium подписка*

✨ *Что входит:*
• ♾️ Безлимитные привычки
• ⏰ Напоминания о привычках
• 📊 Статистика за год
• 📥 Экспорт данных
• 🚫 Без рекламы

💰 *Стоимость:* %.0f₽/месяц%s

💡 *Или бесплатно:* приглашай друзей!`, originalPrice, discountText)

	var paymentURL string
	if h.tinkoffSvc != nil && h.tinkoffSvc.IsConfigured() {
		pending, _ := h.repo.GetUserPendingPayment(ctx, user.ID)
		if pending != nil && pending.DiscountPercent == discount {
			paymentURL = pending.PaymentURL
		}
	}

	reply := tgbotapi.NewMessage(msg.Chat.ID, text)
	reply.ParseMode = "Markdown"
	reply.ReplyMarkup = PremiumKeyboard(paymentURL, discount)
	h.bot.Send(reply)
}

func (h *Handlers) handleHelp(ctx context.Context, msg *tgbotapi.Message) {
	text := `📖 *Справка*

*Команды:*
/start - Начать
/habits - Мои привычки
/new - Создать привычку
/today - Отметить сегодня
/stats - Статистика
/achievements - Достижения
/referral - Рефералы
/premium - Подписка
/promo - использовать промокод

*🆓 Бесплатно:*
• До 3 привычек
• Статистика за 7 дней

*⭐️ Premium:*
• Безлимитные привычки
• ⏰ Напоминания
• Статистика за год
• Экспорт данных
• Без рекламы

*👥 Реферальная программа:*
1. Отмечай привычки 7 дней подряд
2. Приглашай до 5 друзей = до 25 дней
3. Больше 5 = скидка до 50%`

	reply := tgbotapi.NewMessage(msg.Chat.ID, text)
	reply.ParseMode = "Markdown"
	h.bot.Send(reply)
}

func (h *Handlers) handleUnknown(ctx context.Context, msg *tgbotapi.Message) {
	reply := tgbotapi.NewMessage(msg.Chat.ID, "Используй кнопки меню или /help")
	reply.ReplyMarkup = MainMenuKeyboard()
	h.bot.Send(reply)
}

// ==================== CALLBACKS ====================

func (h *Handlers) handleCallback(ctx context.Context, callback *tgbotapi.CallbackQuery) {
	h.bot.Send(tgbotapi.NewCallback(callback.ID, ""))

	data := callback.Data

	switch {
	case data == "cancel":
		delete(h.userStates, callback.From.ID)
		h.editMessage(callback.Message.Chat.ID, callback.Message.MessageID, "❌ Отменено", nil)

	case strings.HasPrefix(data, "freq_"):
		h.handleFrequencyCallback(ctx, callback)

	case strings.HasPrefix(data, "complete_"):
		h.handleCompleteCallback(ctx, callback)

	case strings.HasPrefix(data, "uncomplete_"):
		h.handleUncompleteCallback(ctx, callback)

	case data == "refresh_today" || data == "go_today":
		h.refreshToday(ctx, callback)

	case strings.HasPrefix(data, "habit_"):
		h.handleHabitDetailCallback(ctx, callback)

	case strings.HasPrefix(data, "stats_"):
		h.handleStatsCallback(ctx, callback)

	case strings.HasPrefix(data, "reminder_mode:"):
		h.handleReminderModeCallback(ctx, callback)

	case strings.HasPrefix(data, "reminder_time:"):
		h.handleReminderTimeCallback(ctx, callback)

	case strings.HasPrefix(data, "reminder_days:"):
		h.handleReminderDaysCallback(ctx, callback)

	case strings.HasPrefix(data, "reminder_toggle_day:"):
		h.handleReminderToggleDayCallback(ctx, callback)

	case strings.HasPrefix(data, "reminder_"):
		h.handleReminderCallback(ctx, callback)

	case strings.HasPrefix(data, "setreminder_"):
		h.handleSetReminderCallback(ctx, callback)

	case strings.HasPrefix(data, "delete_"):
		h.handleDeleteCallback(ctx, callback)

	case strings.HasPrefix(data, "confirm_delete_"):
		h.handleConfirmDeleteCallback(ctx, callback)

	case data == "back_to_habits":
		h.handleBackToHabits(ctx, callback)

	case data == "create_habit":
		h.handleCreateHabitCallback(ctx, callback)

	case data == "subscribe":
		h.handleSubscribeCallback(ctx, callback)

	case data == "check_payment":
		h.handleCheckPaymentCallback(ctx, callback)

	case data == "export_data":
		h.handleExportDataCallback(ctx, callback)

	case data == "need_premium_reminder":
		h.handleNeedPremiumReminder(ctx, callback)

	case data == "copy_referral":
		h.handleCopyReferralCallback(ctx, callback)

	case data == "my_referrals":
		h.handleMyReferralsCallback(ctx, callback)

	case data == "chart_weekly":
		h.handleChartWeeklyCallback(ctx, callback)

	case data == "chart_streaks":
		h.handleChartStreaksCallback(ctx, callback)

	case data == "chart_calendar":
		h.handleChartCalendarCallback(ctx, callback)

	case strings.HasPrefix(data, "chart_habit_"):
		h.handleChartHabitCallback(ctx, callback)

	case data == "back_to_stats" || data == "back_to_stats_text":
		h.handleBackToStatsCallback(ctx, callback)

	case strings.HasPrefix(data, "close_ad_"):
		h.bot.Send(tgbotapi.NewDeleteMessage(callback.Message.Chat.ID, callback.Message.MessageID))
	}
}

func (h *Handlers) handleFrequencyCallback(ctx context.Context, callback *tgbotapi.CallbackQuery) {
	state, ok := h.userStates[callback.From.ID]
	if !ok || state.State != "awaiting_frequency" {
		return
	}

	freq := strings.TrimPrefix(callback.Data, "freq_")
	state.Frequency = freq
	state.State = StateWaitingReminderMode

	user, _ := h.repo.GetUserByTelegramID(ctx, callback.From.ID)

	// Если не Premium — сразу создаём без напоминания
	if !user.HasActiveSubscription() {
		h.createHabitFinal(ctx, callback.Message.Chat.ID, callback.From.ID, state)
		delete(h.userStates, callback.From.ID)
		return
	}

	// Premium — спрашиваем про напоминание
	keyboard := ReminderModeKeyboard()
	h.editMessage(callback.Message.Chat.ID, callback.Message.MessageID, "⏰ Настроить напоминание?", &keyboard)
}

func (h *Handlers) handleCompleteCallback(ctx context.Context, callback *tgbotapi.CallbackQuery) {
	habitID, _ := strconv.ParseInt(strings.TrimPrefix(callback.Data, "complete_"), 10, 64)

	user, _ := h.repo.GetUserByTelegramID(ctx, callback.From.ID)
	h.habitSvc.CompleteHabit(ctx, habitID, user.ID)

	streak, _ := h.habitSvc.GetUserOverallStreak(ctx, user.ID)

	// Проверяем достижения
	achievementResult, _ := h.achievementSvc.CheckAndUnlockAchievements(ctx, user.ID, streak)
	if achievementResult != nil && achievementResult.IsNew {
		h.notifyAchievement(callback.From.ID, achievementResult.Achievement)
	}

	// Проверяем реферальный этап 2
	referralResult, _ := h.referralSvc.ProcessReferralStage2(ctx, user.ID, streak)
	if referralResult != nil {
		h.notifyReferralStage2(ctx, referralResult, user)
	}

	// Проверяем разблокировку рефералки
	if streak == domain.ReferralUnlockStreak {
		h.notifyReferralUnlock(callback.From.ID)
	}

	h.refreshToday(ctx, callback)
	h.maybeShowAd(ctx, callback.Message.Chat.ID, user.ID)
}

func (h *Handlers) handleUncompleteCallback(ctx context.Context, callback *tgbotapi.CallbackQuery) {
	habitID, _ := strconv.ParseInt(strings.TrimPrefix(callback.Data, "uncomplete_"), 10, 64)
	user, _ := h.repo.GetUserByTelegramID(ctx, callback.From.ID)
	h.habitSvc.UncompleteHabit(ctx, habitID, user.ID)
	h.refreshToday(ctx, callback)
}

func (h *Handlers) refreshToday(ctx context.Context, callback *tgbotapi.CallbackQuery) {
	user, _ := h.repo.GetUserByTelegramID(ctx, callback.From.ID)
	habits, _ := h.habitSvc.GetUserHabits(ctx, user.ID)
	completedToday, _ := h.habitSvc.GetTodayStatus(ctx, user.ID)

	completed := 0
	for _, done := range completedToday {
		if done {
			completed++
		}
	}

	streak, _ := h.habitSvc.GetUserOverallStreak(ctx, user.ID)
	text := fmt.Sprintf("✅ *Сегодняшний прогресс*\n\nВыполнено: %d из %d\n🔥 Серия: %d дн.", completed, len(habits), streak)

	keyboard := TodayChecklistKeyboard(habits, completedToday)
	h.editMessage(callback.Message.Chat.ID, callback.Message.MessageID, text, &keyboard)
}

func (h *Handlers) handleHabitDetailCallback(ctx context.Context, callback *tgbotapi.CallbackQuery) {
	habitID, _ := strconv.ParseInt(strings.TrimPrefix(callback.Data, "habit_"), 10, 64)
	user, _ := h.repo.GetUserByTelegramID(ctx, callback.From.ID)
	habit, _ := h.habitSvc.GetHabit(ctx, habitID)
	stats, _ := h.habitSvc.GetHabitStats(ctx, habitID)

	var freq string
	switch habit.Frequency {
	case domain.FrequencyDaily:
		freq = "Ежедневно"
	case domain.FrequencyWeekly:
		freq = "Еженедельно"
	case domain.FrequencyMonthly:
		freq = "Ежемесячно"
	}

	reminder := "Не установлено"
	if habit.ReminderTime != nil {
		reminder = *habit.ReminderTime
	}
	if !user.HasActiveSubscription() {
		reminder = "🔒 Только в Premium"
	}

	text := fmt.Sprintf(`📌 *%s*

📅 Периодичность: %s
⏰ Напоминание: %s
📊 *Статистика:*
🔥 Серия: %d дн. | 🏆 Лучшая: %d дн.
📈 Выполнено: %.0f%%`, habit.Name, freq, reminder, stats.CurrentStreak, stats.BestStreak, stats.CompletionRate)

	keyboard := HabitDetailKeyboard(habitID, user.HasActiveSubscription())
	h.editMessage(callback.Message.Chat.ID, callback.Message.MessageID, text, &keyboard)
}

func (h *Handlers) handleStatsCallback(ctx context.Context, callback *tgbotapi.CallbackQuery) {
	habitID, _ := strconv.ParseInt(strings.TrimPrefix(callback.Data, "stats_"), 10, 64)
	stats, _ := h.habitSvc.GetHabitStats(ctx, habitID)

	text := fmt.Sprintf(`📊 *%s*

🔥 Текущая серия: *%d* дн.
🏆 Лучшая серия: *%d* дн.
📅 Дней отслеживания: %d
✅ Выполнено: %d
📈 Процент: *%.0f%%*`,
		stats.HabitName, stats.CurrentStreak, stats.BestStreak,
		stats.TotalDays, stats.CompletedDays, stats.CompletionRate)

	keyboard := BackKeyboard(fmt.Sprintf("habit_%d", habitID))
	h.editMessage(callback.Message.Chat.ID, callback.Message.MessageID, text, &keyboard)
}

func (h *Handlers) handleReminderCallback(ctx context.Context, callback *tgbotapi.CallbackQuery) {
	habitID, _ := strconv.ParseInt(strings.TrimPrefix(callback.Data, "reminder_"), 10, 64)
	keyboard := ReminderTimeKeyboard(habitID)
	h.editMessage(callback.Message.Chat.ID, callback.Message.MessageID, "⏰ Выбери время напоминания:", &keyboard)
}

func (h *Handlers) handleSetReminderCallback(ctx context.Context, callback *tgbotapi.CallbackQuery) {
	parts := strings.Split(strings.TrimPrefix(callback.Data, "setreminder_"), "_")
	if len(parts) != 2 {
		return
	}

	habitID, _ := strconv.ParseInt(parts[0], 10, 64)
	timeStr := parts[1]
	user, _ := h.repo.GetUserByTelegramID(ctx, callback.From.ID)

	var reminder *string
	var text string

	if timeStr == "off" {
		text = "🔕 Напоминание отключено"
	} else {
		reminder = &timeStr
		text = fmt.Sprintf("⏰ Напоминание: %s", timeStr)
	}

	h.habitSvc.UpdateHabitReminder(ctx, habitID, user.ID, reminder)
	keyboard := BackKeyboard(fmt.Sprintf("habit_%d", habitID))
	h.editMessage(callback.Message.Chat.ID, callback.Message.MessageID, text, &keyboard)
}

func (h *Handlers) handleDeleteCallback(ctx context.Context, callback *tgbotapi.CallbackQuery) {
	habitID, _ := strconv.ParseInt(strings.TrimPrefix(callback.Data, "delete_"), 10, 64)
	habit, _ := h.habitSvc.GetHabit(ctx, habitID)

	text := fmt.Sprintf("🗑 Удалить *%s*?\n\nСтатистика будет потеряна!", habit.Name)
	keyboard := ConfirmDeleteKeyboard(habitID)
	h.editMessage(callback.Message.Chat.ID, callback.Message.MessageID, text, &keyboard)
}

func (h *Handlers) handleConfirmDeleteCallback(ctx context.Context, callback *tgbotapi.CallbackQuery) {
	habitID, _ := strconv.ParseInt(strings.TrimPrefix(callback.Data, "confirm_delete_"), 10, 64)
	user, _ := h.repo.GetUserByTelegramID(ctx, callback.From.ID)
	h.habitSvc.DeleteHabit(ctx, habitID, user.ID)

	keyboard := BackKeyboard("back_to_habits")
	h.editMessage(callback.Message.Chat.ID, callback.Message.MessageID, "✅ Привычка удалена", &keyboard)
}

func (h *Handlers) handleBackToHabits(ctx context.Context, callback *tgbotapi.CallbackQuery) {
	user, _ := h.repo.GetUserByTelegramID(ctx, callback.From.ID)
	habits, _ := h.habitSvc.GetUserHabits(ctx, user.ID)
	completedToday, _ := h.habitSvc.GetTodayStatus(ctx, user.ID)

	text := "📋 *Мои привычки*\n\n"
	if len(habits) == 0 {
		text += "У тебя пока нет привычек."
	} else {
		text += "Выбери привычку:"
	}

	keyboard := HabitsListKeyboard(habits, completedToday)
	h.editMessage(callback.Message.Chat.ID, callback.Message.MessageID, text, &keyboard)
}

func (h *Handlers) handleCreateHabitCallback(ctx context.Context, callback *tgbotapi.CallbackQuery) {
	h.userStates[callback.From.ID] = &UserState{State: "awaiting_name"}
	keyboard := CancelKeyboard()
	h.editMessage(callback.Message.Chat.ID, callback.Message.MessageID, "➕ Введи название привычки:", &keyboard)
}

func (h *Handlers) handleSubscribeCallback(ctx context.Context, callback *tgbotapi.CallbackQuery) {
	if h.tinkoffSvc == nil || !h.tinkoffSvc.IsConfigured() {
		h.sendMessage(callback.Message.Chat.ID, "💡 Оплата временно недоступна. Используй реферальную программу!")
		return
	}

	payment, err := h.tinkoffSvc.CreatePayment(ctx, callback.From.ID, h.subPrice, "Premium подписка на 1 месяц")
	if err != nil {
		log.Printf("Error creating payment: %v", err)
		h.sendError(callback.Message.Chat.ID, "Ошибка создания платежа")
		return
	}

	priceText := fmt.Sprintf("%.0f₽", float64(payment.Amount)/100)
	if payment.DiscountPercent > 0 {
		priceText = fmt.Sprintf("%.0f₽ (скидка %d%%)", float64(payment.Amount)/100, payment.DiscountPercent)
	}

	text := fmt.Sprintf(`💳 *Оплата подписки*

Сумма: *%s*

Нажми кнопку для оплаты.
После оплаты нажми "Проверить оплату".`, priceText)

	keyboard := PremiumKeyboard(payment.PaymentURL, payment.DiscountPercent)
	h.editMessage(callback.Message.Chat.ID, callback.Message.MessageID, text, &keyboard)
}

func (h *Handlers) handleCheckPaymentCallback(ctx context.Context, callback *tgbotapi.CallbackQuery) {
	user, err := h.repo.GetUserByTelegramID(ctx, callback.From.ID)
	if err != nil {
		h.sendError(callback.Message.Chat.ID, "Пользователь не найден")
		return
	}

	// Если уже Premium — показываем статус
	if user.HasActiveSubscription() {
		text := fmt.Sprintf(`🎉 *Оплата прошла успешно!*

Premium активен до: *%s*

Теперь тебе доступны:
✅ Безлимитные привычки
✅ Напоминания о привычках
✅ Статистика за год
✅ Экспорт / импорт данных
✅ Отсутствие рекламы`, user.SubscriptionEnd.Format("02.01.2006"))
		h.editMessage(callback.Message.Chat.ID, callback.Message.MessageID, text, nil)
		return
	}

	// Ищем последний pending платёж
	payment, err := h.repo.GetUserPendingPayment(ctx, user.ID)
	if err != nil || payment == nil {
		h.bot.Send(tgbotapi.NewCallback(callback.ID, "Нет активного платежа"))
		return
	}

	// Запрашиваем актуальный статус у Tinkoff
	tinkoffResp, err := h.tinkoffSvc.GetPaymentStatus(ctx, payment.OrderID)
	if err != nil {
		log.Printf("Ошибка GetState для OrderID=%s: %v", payment.OrderID, err)
		h.bot.Send(tgbotapi.NewCallback(callback.ID, "Не удалось проверить платёж"))
		return
	}

	if tinkoffResp.Status == "CONFIRMED" {
		// Активируем подписку напрямую
		if err := h.tinkoffSvc.ProcessConfirmedPayment(ctx, payment.OrderID); err != nil {
			log.Printf("Ошибка активации подписки: %v", err)
			h.bot.Send(tgbotapi.NewCallback(callback.ID, "Ошибка при активации"))
			return
		}

		// Обновляем данные пользователя
		updatedUser, _ := h.repo.GetUserByTelegramID(ctx, callback.From.ID)
		text := fmt.Sprintf(`🎉 *Оплата прошла успешно!*

Premium активен до: *%s*

Теперь тебе доступны:
✅ Безлимитные привычки
✅ Напоминания о привычках
✅ Статистика за год
✅ Экспорт / импорт данных
✅ Отсутствие рекламы`, updatedUser.SubscriptionEnd.Format("02.01.2006"))
		h.editMessage(callback.Message.Chat.ID, callback.Message.MessageID, text, nil)

		// Уведомление
		h.NotifyPaymentSuccess(callback.From.ID)
	} else {
		h.bot.Send(tgbotapi.NewCallback(callback.ID, "Оплата ещё не поступила"))
	}
}

func (h *Handlers) handleExportDataCallback(ctx context.Context, callback *tgbotapi.CallbackQuery) {
	user, _ := h.repo.GetUserByTelegramID(ctx, callback.From.ID)

	if !user.HasActiveSubscription() {
		text := "🔒 *Экспорт данных — Premium функция*"
		keyboard := PremiumKeyboard("", user.DiscountPercent)
		h.editMessage(callback.Message.Chat.ID, callback.Message.MessageID, text, &keyboard)
		return
	}

	csvData, err := h.exportSvc.ExportToCSV(ctx, user.ID)
	if err != nil {
		h.sendError(callback.Message.Chat.ID, "Ошибка экспорта")
		return
	}

	doc := tgbotapi.NewDocument(callback.Message.Chat.ID, tgbotapi.FileBytes{
		Name:  fmt.Sprintf("habits_export_%s.csv", time.Now().Format("2006-01-02")),
		Bytes: csvData,
	})
	doc.Caption = "📥 Твои данные экспортированы!"
	h.bot.Send(doc)
}

func (h *Handlers) handleNeedPremiumReminder(ctx context.Context, callback *tgbotapi.CallbackQuery) {
	user, _ := h.repo.GetUserByTelegramID(ctx, callback.From.ID)

	text := `🔒 *Напоминания — Premium функция*

С напоминаниями ты не пропустишь ни одного дня!

⏰ Бот напомнит тебе в нужное время.

Оформи Premium!`

	keyboard := PremiumKeyboard("", user.DiscountPercent)
	h.editMessage(callback.Message.Chat.ID, callback.Message.MessageID, text, &keyboard)
}

func (h *Handlers) handleCopyReferralCallback(ctx context.Context, callback *tgbotapi.CallbackQuery) {
	user, _ := h.repo.GetUserByTelegramID(ctx, callback.From.ID)
	referralLink, _ := h.referralSvc.GetReferralLink(ctx, user.ID, h.botUsername)

	msg := tgbotapi.NewMessage(callback.Message.Chat.ID, referralLink)
	h.bot.Send(msg)
	h.bot.Send(tgbotapi.NewCallback(callback.ID, "Ссылка отправлена!"))
}

func (h *Handlers) handleMyReferralsCallback(ctx context.Context, callback *tgbotapi.CallbackQuery) {
	user, _ := h.repo.GetUserByTelegramID(ctx, callback.From.ID)
	referrals, _ := h.referralSvc.GetUserReferrals(ctx, user.ID)

	var sb strings.Builder
	sb.WriteString("📋 *Мои приглашения*\n\n")

	if len(referrals) == 0 {
		sb.WriteString("Ты ещё никого не пригласил.")
	} else {
		for i, ref := range referrals {
			referred, _ := h.repo.GetUserByID(ctx, ref.ReferredID)
			name := "Пользователь"
			if referred != nil && referred.FirstName != "" {
				name = referred.FirstName
			}

			stage1 := "✅"
			stage2 := "⬜️"
			if ref.Stage2Applied {
				stage2 = "✅"
			}

			bonus := ref.Stage1BonusDays + ref.Stage2BonusDays
			bonusText := fmt.Sprintf("+%d дн.", bonus)
			if ref.GaveDiscount {
				bonusText = "скидка"
			}
			sb.WriteString(fmt.Sprintf("%d. *%s* [%s|%s] %s\n", i+1, name, stage1, stage2, bonusText))
		}
	}

	keyboard := BackKeyboard("back_to_referral")
	h.editMessage(callback.Message.Chat.ID, callback.Message.MessageID, sb.String(), &keyboard)
}

// ==================== NOTIFICATIONS ====================

func (h *Handlers) notifyAchievement(telegramID int64, achievement *domain.AchievementConfig) {
	bonus := ""
	if achievement.BonusDays > 0 {
		bonus = fmt.Sprintf("\n\n🎁 Бонус: *+%d дней* Premium!", achievement.BonusDays)
	}

	text := fmt.Sprintf(`%s *Новое достижение!*

*%s*
%s%s`, achievement.Emoji, achievement.Title, achievement.Description, bonus)

	msg := tgbotapi.NewMessage(telegramID, text)
	msg.ParseMode = "Markdown"
	h.bot.Send(msg)
}

func (h *Handlers) notifyReferralStage2(ctx context.Context, result *service.ReferralResult, referredUser *domain.User) {
	text := fmt.Sprintf(`🎉 *Этап 2 выполнен!*

Ты отмечал привычки %d дней подряд!

🎁 +%d дней Premium тебе и твоему пригласившему!`, domain.ReferralStage2Streak, result.ReferredBonus)

	msg := tgbotapi.NewMessage(referredUser.TelegramID, text)
	msg.ParseMode = "Markdown"
	h.bot.Send(msg)

	referrer, _ := h.repo.GetUserByID(ctx, result.ReferrerUserID)
	if referrer != nil {
		text := fmt.Sprintf(`🎉 *Реферал завершён!*

*%s* достиг %d дней серии!

🎁 *Этап 2:* +%d дней Premium!`, referredUser.FirstName, domain.ReferralStage2Streak, result.ReferrerBonus)

		msg := tgbotapi.NewMessage(referrer.TelegramID, text)
		msg.ParseMode = "Markdown"
		h.bot.Send(msg)
	}
}

func (h *Handlers) notifyReferralUnlock(telegramID int64) {
	text := `🔓 *Реферальная программа разблокирована!*

Ты выполнял привычки 7 дней подряд!

Теперь можешь приглашать друзей:
• *Этап 1:* +2 дня при регистрации
• *Этап 2:* +3 дня при достижении серии

Нажми "👥 Рефералы"!`

	msg := tgbotapi.NewMessage(telegramID, text)
	msg.ParseMode = "Markdown"
	h.bot.Send(msg)
}

// ==================== ADS ====================

func (h *Handlers) maybeShowAd(ctx context.Context, chatID int64, userID int64) {
	shouldShow, _ := h.adSvc.ShouldShowAd(ctx, userID)
	if !shouldShow {
		return
	}

	ad := h.adSvc.GetRandomAd(ctx)
	if ad == nil {
		return
	}

	h.adSvc.TrackView(ctx, ad.ID)

	msg := tgbotapi.NewMessage(chatID, ad.Text)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = AdKeyboard(ad.ID)
	h.bot.Send(msg)
}

// ==================== HELPERS ====================

func (h *Handlers) sendMessage(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	h.bot.Send(msg)
}

func (h *Handlers) sendError(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, "❌ "+text)
	h.bot.Send(msg)
}

func (h *Handlers) editMessage(chatID int64, messageID int, text string, keyboard *tgbotapi.InlineKeyboardMarkup) {
	edit := tgbotapi.NewEditMessageText(chatID, messageID, text)
	edit.ParseMode = "Markdown"
	if keyboard != nil {
		edit.ReplyMarkup = keyboard
	}
	h.bot.Send(edit)
}

func (h *Handlers) SendReminder(telegramID int64, habitName string) error {
	text := fmt.Sprintf("⏰ *Напоминание!*\n\nПора выполнить: *%s*", habitName)

	msg := tgbotapi.NewMessage(telegramID, text)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Отметить", "go_today"),
		),
	)

	_, err := h.bot.Send(msg)
	return err
}

func (h *Handlers) NotifyPaymentSuccess(telegramID int64) {
	text := `🎉 *Оплата прошла успешно!*

Твоя Premium подписка активирована на 30 дней!

✅ Безлимитные привычки
✅ Напоминания
✅ Статистика за год
✅ Экспорт данных
✅ Без рекламы`

	msg := tgbotapi.NewMessage(telegramID, text)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = MainMenuKeyboard()
	h.bot.Send(msg)
}

func (h *Handlers) applyPromocode(ctx context.Context, chatID int64, userID int64, code string) {
	promo, err := h.repo.GetPromocodeByCode(ctx, code)
	if err != nil || promo == nil || !promo.IsActive {
		h.bot.Send(tgbotapi.NewMessage(chatID, "❌ Промокод не найден"))
		return
	}

	// Проверяем лимит
	if promo.MaxUses != nil && promo.UsedCount >= *promo.MaxUses {
		h.bot.Send(tgbotapi.NewMessage(chatID, "❌ Промокод больше не действует"))
		return
	}

	// Проверяем использовал ли
	used, _ := h.repo.HasUserUsedPromocode(ctx, userID, promo.ID)
	if used {
		h.bot.Send(tgbotapi.NewMessage(chatID, "❌ Вы уже использовали этот промокод"))
		return
	}

	// Сохраняем
	h.repo.SetUserActivePromocode(ctx, userID, promo.ID)

	h.bot.Send(tgbotapi.NewMessage(chatID,
		fmt.Sprintf("✅ Промокод применён!\n\nСкидка: %d%%\n\nПерейдите к оплате — скидка применится автоматически.",
			promo.DiscountPercent)))
}

func (h *Handlers) handleReminderModeCallback(ctx context.Context, callback *tgbotapi.CallbackQuery) {
	state, ok := h.userStates[callback.From.ID]
	if !ok {
		return
	}

	mode := strings.TrimPrefix(callback.Data, "reminder_mode:")

	switch mode {
	case "preset":
		keyboard := ReminderPresetTimeKeyboard()
		h.editMessage(callback.Message.Chat.ID, callback.Message.MessageID, "⏰ Выбери время:", &keyboard)

	case "custom":
		state.State = StateWaitingCustomTime
		h.editMessage(callback.Message.Chat.ID, callback.Message.MessageID, "✏️ Введи время в формате ЧЧ:ММ (например 14:30):", nil)

	case "none":
		h.createHabitFinal(ctx, callback.Message.Chat.ID, callback.From.ID, state)
		delete(h.userStates, callback.From.ID)

	case "back":
		state.State = "awaiting_frequency"
		keyboard := FrequencyKeyboard()
		h.editMessage(callback.Message.Chat.ID, callback.Message.MessageID, "📅 Выбери периодичность:", &keyboard)
	}
}

func (h *Handlers) handleReminderTimeCallback(ctx context.Context, callback *tgbotapi.CallbackQuery) {
	state, ok := h.userStates[callback.From.ID]
	if !ok {
		return
	}

	timeVal := strings.TrimPrefix(callback.Data, "reminder_time:")
	state.ReminderTime = timeVal
	state.State = StateWaitingReminderDays

	keyboard := ReminderDaysKeyboard()
	h.editMessage(callback.Message.Chat.ID, callback.Message.MessageID, "📅 В какие дни напоминать?", &keyboard)
}

func (h *Handlers) handleReminderDaysCallback(ctx context.Context, callback *tgbotapi.CallbackQuery) {
	state, ok := h.userStates[callback.From.ID]
	if !ok {
		return
	}

	daysVal := strings.TrimPrefix(callback.Data, "reminder_days:")

	switch daysVal {
	case "all":
		state.SelectedDays = map[int]bool{1: true, 2: true, 3: true, 4: true, 5: true, 6: true, 7: true}
		h.createHabitFinal(ctx, callback.Message.Chat.ID, callback.From.ID, state)
		delete(h.userStates, callback.From.ID)

	case "weekdays":
		state.SelectedDays = map[int]bool{1: true, 2: true, 3: true, 4: true, 5: true}
		h.createHabitFinal(ctx, callback.Message.Chat.ID, callback.From.ID, state)
		delete(h.userStates, callback.From.ID)

	case "weekends":
		state.SelectedDays = map[int]bool{6: true, 7: true}
		h.createHabitFinal(ctx, callback.Message.Chat.ID, callback.From.ID, state)
		delete(h.userStates, callback.From.ID)

	case "custom":
		state.State = StateWaitingCustomDays
		if state.SelectedDays == nil {
			state.SelectedDays = make(map[int]bool)
		}
		keyboard := ReminderCustomDaysKeyboard(state.SelectedDays)
		h.editMessage(callback.Message.Chat.ID, callback.Message.MessageID, "📅 Выбери дни:", &keyboard)

	case "done":
		if len(state.SelectedDays) == 0 {
			state.SelectedDays = map[int]bool{1: true, 2: true, 3: true, 4: true, 5: true, 6: true, 7: true}
		}
		h.createHabitFinal(ctx, callback.Message.Chat.ID, callback.From.ID, state)
		delete(h.userStates, callback.From.ID)
	}
}

func (h *Handlers) handleReminderToggleDayCallback(ctx context.Context, callback *tgbotapi.CallbackQuery) {
	state, ok := h.userStates[callback.From.ID]
	if !ok {
		return
	}

	day, _ := strconv.Atoi(strings.TrimPrefix(callback.Data, "reminder_toggle_day:"))

	if state.SelectedDays == nil {
		state.SelectedDays = make(map[int]bool)
	}
	state.SelectedDays[day] = !state.SelectedDays[day]

	keyboard := ReminderCustomDaysKeyboard(state.SelectedDays)
	h.editMessage(callback.Message.Chat.ID, callback.Message.MessageID, "📅 Выбери дни:", &keyboard)
}

func (h *Handlers) createHabitFinal(ctx context.Context, chatID int64, telegramID int64, state *UserState) {
	user, err := h.repo.GetUserByTelegramID(ctx, telegramID)
	if err != nil {
		h.sendError(chatID, "Ошибка")
		return
	}

	freq := domain.Frequency(state.Frequency)

	// Подготовка напоминания
	var reminderTime *string
	var reminderDays []int

	if state.ReminderTime != "" {
		reminderTime = &state.ReminderTime
		for d, selected := range state.SelectedDays {
			if selected {
				reminderDays = append(reminderDays, d)
			}
		}
		sort.Ints(reminderDays)
	}

	habit, err := h.habitSvc.CreateHabit(ctx, user, state.HabitName, "", freq)
	if err != nil {
		h.sendError(chatID, "Ошибка создания привычки")
		return
	}

	// Если есть напоминание — обновляем
	if reminderTime != nil && len(reminderDays) > 0 {
		h.repo.UpdateHabitReminder(ctx, habit.ID, reminderTime, reminderDays)
	}

	text := fmt.Sprintf("✅ Привычка *%s* создана!", habit.Name)
	if reminderTime != nil {
		daysText := formatDays(reminderDays)
		text += fmt.Sprintf("\n⏰ Напоминание: *%s* (%s)", *reminderTime, daysText)
	}

	keyboard := BackKeyboard("back_to_habits")
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard
	h.bot.Send(msg)
}

func formatDays(days []int) string {
	if len(days) == 0 || len(days) == 7 {
		return "каждый день"
	}
	if len(days) == 5 && days[0] == 1 && days[4] == 5 {
		return "по будням"
	}
	if len(days) == 2 && days[0] == 6 && days[1] == 7 {
		return "по выходным"
	}

	names := map[int]string{1: "пн", 2: "вт", 3: "ср", 4: "чт", 5: "пт", 6: "сб", 7: "вс"}
	var result []string
	for _, d := range days {
		result = append(result, names[d])
	}
	return strings.Join(result, ", ")
}

// ==================== CHARTS ====================

func (h *Handlers) handleChartWeeklyCallback(ctx context.Context, callback *tgbotapi.CallbackQuery) {
	user, err := h.repo.GetUserByTelegramID(ctx, callback.From.ID)
	if err != nil {
		log.Printf("Chart weekly: ошибка получения юзера: %v", err)
		h.sendError(callback.Message.Chat.ID, "Ошибка загрузки данных")
		return
	}

	// Получаем данные за неделю
	weeklyStats, err := h.repo.GetWeeklyCompletionStats(ctx, user.ID)
	if err != nil {
		log.Printf("Chart weekly: ошибка GetWeeklyCompletionStats для user.ID=%d: %v", user.ID, err)
		// Если ошибка — просто создаём пустой график
		weeklyStats = make(map[string]int)
	}

	log.Printf("Chart weekly: weeklyStats=%v", weeklyStats)

	// Формируем данные для графика
	var labels []string
	var values []int

	now := time.Now()
	dayNames := []string{"Вс", "Пн", "Вт", "Ср", "Чт", "Пт", "Сб"}

	for i := 6; i >= 0; i-- {
		date := now.AddDate(0, 0, -i)
		dateStr := date.Format("2006-01-02")
		dayName := dayNames[int(date.Weekday())]

		labels = append(labels, dayName)
		values = append(values, weeklyStats[dateStr])
	}

	chartData := ChartData{
		Labels: labels,
		Values: values,
	}

	chartURL := GenerateWeeklyChart(chartData)
	log.Printf("Chart weekly URL: %s", chartURL[:100]) // первые 100 символов

	// Отправляем картинку
	photo := tgbotapi.NewPhoto(callback.Message.Chat.ID, tgbotapi.FileURL(chartURL))
	photo.Caption = "📊 *Выполнено привычек за неделю*"
	photo.ParseMode = "Markdown"

	// Удаляем старое сообщение
	h.bot.Request(tgbotapi.NewDeleteMessage(callback.Message.Chat.ID, callback.Message.MessageID))
	h.bot.Send(photo)

	// Кнопка "Назад"
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("« Назад к статистике", "back_to_stats_text"),
		),
	)
	msg := tgbotapi.NewMessage(callback.Message.Chat.ID, "👆 График выше")
	msg.ReplyMarkup = keyboard
	h.bot.Send(msg)
}

func (h *Handlers) handleChartStreaksCallback(ctx context.Context, callback *tgbotapi.CallbackQuery) {
	user, _ := h.repo.GetUserByTelegramID(ctx, callback.From.ID)

	streaks, err := h.repo.GetHabitsStreaks(ctx, user.ID)
	if err != nil {
		log.Printf("Chart streaks: ошибка GetHabitsStreaks: %v", err)
		h.sendError(callback.Message.Chat.ID, "Нет данных для графика")
		return
	}

	log.Printf("Chart streaks: получено %d привычек", len(streaks))

	if len(streaks) == 0 {
		h.sendError(callback.Message.Chat.ID, "У тебя нет привычек")
		return
	}

	// Конвертируем в формат для графика
	var chartData []HabitStreakData
	for _, s := range streaks {
		log.Printf("Chart streaks: %s = %d дней", s.Name, s.Streak)
		chartData = append(chartData, HabitStreakData{
			Name:   s.Name,
			Streak: s.Streak,
		})
	}

	chartURL := GenerateStreakChart(chartData)
	log.Printf("Chart streaks URL length: %d", len(chartURL))

	// Отправляем картинку
	photo := tgbotapi.NewPhoto(callback.Message.Chat.ID, tgbotapi.FileURL(chartURL))
	photo.Caption = "🔥 *Текущие серии привычек*\n\nЧем длиннее полоска — тем дольше серия!"
	photo.ParseMode = "Markdown"

	h.bot.Request(tgbotapi.NewDeleteMessage(callback.Message.Chat.ID, callback.Message.MessageID))
	h.bot.Send(photo)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("« Назад к статистике", "back_to_stats_text"),
		),
	)
	msg := tgbotapi.NewMessage(callback.Message.Chat.ID, "👆 График серий")
	msg.ReplyMarkup = keyboard
	h.bot.Send(msg)
}

func (h *Handlers) handleChartCalendarCallback(ctx context.Context, callback *tgbotapi.CallbackQuery) {
	user, err := h.repo.GetUserByTelegramID(ctx, callback.From.ID)
	if err != nil {
		log.Printf("Chart calendar: ошибка получения юзера: %v", err)
		h.sendError(callback.Message.Chat.ID, "Ошибка")
		return
	}

	habits, err := h.habitSvc.GetUserHabits(ctx, user.ID)
	if err != nil {
		log.Printf("Chart calendar: ошибка получения привычек: %v", err)
		h.sendError(callback.Message.Chat.ID, "Ошибка загрузки привычек")
		return
	}

	log.Printf("Chart calendar: найдено %d привычек для user.ID=%d", len(habits), user.ID)

	if len(habits) == 0 {
		h.answerCallback(callback.ID, "У тебя нет привычек")
		return
	}

	keyboard := HabitSelectForChartKeyboard(habits)
	h.editMessage(callback.Message.Chat.ID, callback.Message.MessageID, "📅 Выбери привычку для календаря:", &keyboard)
}

func (h *Handlers) handleChartHabitCallback(ctx context.Context, callback *tgbotapi.CallbackQuery) {
	habitIDStr := strings.TrimPrefix(callback.Data, "chart_habit_")
	habitID, err := strconv.ParseInt(habitIDStr, 10, 64)
	if err != nil {
		log.Printf("Chart habit: ошибка парсинга habitID из '%s': %v", habitIDStr, err)
		h.sendError(callback.Message.Chat.ID, "Ошибка")
		return
	}

	log.Printf("Chart habit: запрос для habitID=%d", habitID)

	habit, err := h.habitSvc.GetHabit(ctx, habitID)
	if err != nil {
		log.Printf("Chart habit: ошибка получения привычки %d: %v", habitID, err)
		h.sendError(callback.Message.Chat.ID, "Привычка не найдена")
		return
	}

	log.Printf("Chart habit: привычка найдена: %s", habit.Name)

	// Получаем дни выполнения за 30 дней
	completedDays, err := h.repo.GetHabitCompletionDays(ctx, habitID, 30)
	if err != nil {
		log.Printf("Chart habit: ошибка GetHabitCompletionDays: %v", err)
		completedDays = make(map[string]bool)
	}

	log.Printf("Chart habit: найдено %d дней выполнения", len(completedDays))

	chartURL := GenerateHabitCalendar(habit.Name, completedDays)
	log.Printf("Chart habit: URL длина=%d", len(chartURL))

	// Удаляем старое сообщение
	h.bot.Request(tgbotapi.NewDeleteMessage(callback.Message.Chat.ID, callback.Message.MessageID))

	// Отправляем картинку
	photo := tgbotapi.NewPhoto(callback.Message.Chat.ID, tgbotapi.FileURL(chartURL))
	photo.Caption = fmt.Sprintf("📅 *%s* — последние 30 дней\n\n🟢 — выполнено\n🔴 — пропущено", habit.Name)
	photo.ParseMode = "Markdown"

	_, err = h.bot.Send(photo)
	if err != nil {
		log.Printf("Chart habit: ошибка отправки фото: %v", err)
		h.sendMessage(callback.Message.Chat.ID, "❌ Не удалось загрузить график")
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("« Назад к статистике", "back_to_stats_text"),
		),
	)
	msg := tgbotapi.NewMessage(callback.Message.Chat.ID, "👆 Календарь привычки")
	msg.ReplyMarkup = keyboard
	h.bot.Send(msg)
}

func (h *Handlers) handleBackToStatsCallback(ctx context.Context, callback *tgbotapi.CallbackQuery) {
	user, _ := h.repo.GetUserByTelegramID(ctx, callback.From.ID)
	stats, _ := h.habitSvc.GetUserStats(ctx, user.ID)
	overallStreak, _ := h.habitSvc.GetUserOverallStreak(ctx, user.ID)

	var sb strings.Builder
	sb.WriteString("📊 *Твоя статистика*\n\n")
	sb.WriteString(fmt.Sprintf("🔥 *Общая серия:* %d дн.\n\n", overallStreak))

	for _, s := range stats {
		emoji := "🔥"
		if s.CurrentStreak == 0 {
			emoji = "💤"
		}
		sb.WriteString(fmt.Sprintf("*%s*\n", s.HabitName))
		sb.WriteString(fmt.Sprintf("  %s Серия: %d дн. | 🏆 Лучшая: %d дн.\n", emoji, s.CurrentStreak, s.BestStreak))
		sb.WriteString(fmt.Sprintf("  📈 Выполнено: %.0f%%\n\n", s.CompletionRate))
	}

	sb.WriteString("👇 *Выбери график:*")

	keyboard := StatsKeyboard()
	h.editMessage(callback.Message.Chat.ID, callback.Message.MessageID, sb.String(), &keyboard)
}

// -------------------- HELPERS --------------------------

func (h *Handlers) answerCallback(callbackID string, text string) {
	h.bot.Send(tgbotapi.NewCallback(callbackID, text))
}
