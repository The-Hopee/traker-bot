package telegram

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"habit-tracker-bot/internal/domain"
)

func MainMenuKeyboard() tgbotapi.ReplyKeyboardMarkup {
	return tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("📋 Мои привычки"),
			tgbotapi.NewKeyboardButton("➕ Новая привычка"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("📊 Статистика"),
			tgbotapi.NewKeyboardButton("✅ Отметить сегодня"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("🏆 Достижения"),
			tgbotapi.NewKeyboardButton("👥 Рефералы"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("⭐️ Premium"),
			tgbotapi.NewKeyboardButton("❓ Помощь"),
		),
	)
}

func HabitsListKeyboard(habits []*domain.Habit, completedToday map[int64]bool) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

	for _, habit := range habits {
		status := "⬜️"
		if completedToday[habit.ID] {
			status = "✅"
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(status+" "+habit.Name, fmt.Sprintf("habit_%d", habit.ID)),
		))
	}

	if len(habits) == 0 {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("➕ Создать первую привычку", "create_habit"),
		))
	}

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func TodayChecklistKeyboard(habits []*domain.Habit, completedToday map[int64]bool) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

	for _, habit := range habits {
		if completedToday[habit.ID] {
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("✅ "+habit.Name, fmt.Sprintf("uncomplete_%d", habit.ID)),
			))
		} else {
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("⬜️ "+habit.Name, fmt.Sprintf("complete_%d", habit.ID)),
			))
		}
	}

	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔄 Обновить", "refresh_today"),
	))

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func HabitDetailKeyboard(habitID int64, isPremium bool) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("✅ Выполнено", fmt.Sprintf("complete_%d", habitID)),
		tgbotapi.NewInlineKeyboardButtonData("📊 Статистика", fmt.Sprintf("stats_%d", habitID)),
	))

	if isPremium {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⏰ Напоминание", fmt.Sprintf("reminder_%d", habitID)),
			tgbotapi.NewInlineKeyboardButtonData("🗑 Удалить", fmt.Sprintf("delete_%d", habitID)),
		))
	} else {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔒 Напоминание", "need_premium_reminder"),
			tgbotapi.NewInlineKeyboardButtonData("🗑 Удалить", fmt.Sprintf("delete_%d", habitID)),
		))
	}

	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("« Назад", "back_to_habits"),
	))

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func FrequencyKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📅 Ежедневно", "freq_daily"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📆 Еженедельно", "freq_weekly"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🗓 Ежемесячно", "freq_monthly"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Отмена", "cancel"),
		),
	)
}

func ReminderTimeKeyboard(habitID int64) tgbotapi.InlineKeyboardMarkup {
	times := []string{"02:00", "04:00", "06:00", "08:00", "10:00", "12:00", "14:00", "16:00", "18:00", "20:00", "22:00", "00:00"}
	var rows [][]tgbotapi.InlineKeyboardButton

	for i := 0; i < len(times); i += 3 {
		var row []tgbotapi.InlineKeyboardButton
		for j := i; j < i+3 && j < len(times); j++ {
			row = append(row, tgbotapi.NewInlineKeyboardButtonData(times[j], fmt.Sprintf("setreminder_%d_%s", habitID, times[j])))
		}
		rows = append(rows, row)
	}

	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🚫 Отключить", fmt.Sprintf("setreminder_%d_off", habitID)),
	))
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("« Назад", fmt.Sprintf("habit_%d", habitID)),
	))

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func ConfirmDeleteKeyboard(habitID int64) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Да, удалить", fmt.Sprintf("confirm_delete_%d", habitID)),
			tgbotapi.NewInlineKeyboardButtonData("❌ Отмена", fmt.Sprintf("habit_%d", habitID)),
		),
	)
}

func PremiumKeyboard(paymentURL string, discount int) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

	if paymentURL != "" {
		text := "💳 Оплатить"
		if discount > 0 {
			text = fmt.Sprintf("💳 Оплатить (скидка %d%%)", discount)
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL(text, paymentURL),
		))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔄 Проверить оплату", "check_payment"),
		))
	} else {
		text := "💳 Оформить подписку"
		if discount > 0 {
			text = fmt.Sprintf("💳 Оформить со скидкой %d%%", discount)
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(text, "subscribe"),
		))
	}

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func PremiumActiveKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📥 Экспорт данных", "export_data"),
		),
	)
}

func ReferralKeyboard(referralLink string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("📤 Поделиться", fmt.Sprintf("https://t.me/share/url?url=%s&text=Присоединяйся к трекеру привычек!", referralLink)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📋 Скопировать ссылку", "copy_referral"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📊 Мои приглашения", "my_referrals"),
		),
	)
}

func ReferralLockedKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Отметить сегодня", "go_today"),
		),
	)
}

func AdKeyboard(adID int64) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⭐️ Получить Premium", "subscribe"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Закрыть", fmt.Sprintf("close_ad_%d", adID)),
		),
	)
}

func BackKeyboard(callback string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("« Назад", callback),
		),
	)
}

func CancelKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Отмена", "cancel"),
		),
	)
}
