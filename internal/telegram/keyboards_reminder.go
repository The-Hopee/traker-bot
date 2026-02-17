package telegram

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func ReminderModeKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⏰ Готовое время", "reminder_mode:preset"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✏️ Своё время", "reminder_mode:custom"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Без напоминания", "reminder_mode:none"),
		),
	)
}

func ReminderPresetTimeKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("06:00", "reminder_time:06:00"),
			tgbotapi.NewInlineKeyboardButtonData("07:00", "reminder_time:07:00"),
			tgbotapi.NewInlineKeyboardButtonData("08:00", "reminder_time:08:00"),
			tgbotapi.NewInlineKeyboardButtonData("09:00", "reminder_time:09:00"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("10:00", "reminder_time:10:00"),
			tgbotapi.NewInlineKeyboardButtonData("11:00", "reminder_time:11:00"),
			tgbotapi.NewInlineKeyboardButtonData("12:00", "reminder_time:12:00"),
			tgbotapi.NewInlineKeyboardButtonData("13:00", "reminder_time:12:00"),
			tgbotapi.NewInlineKeyboardButtonData("14:00", "reminder_time:14:00"),
			tgbotapi.NewInlineKeyboardButtonData("15:00", "reminder_time:15:00"),
			tgbotapi.NewInlineKeyboardButtonData("16:00", "reminder_time:16:00"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("17:00", "reminder_time:72:00"),
			tgbotapi.NewInlineKeyboardButtonData("18:00", "reminder_time:18:00"),
			tgbotapi.NewInlineKeyboardButtonData("19:00", "reminder_time:19:00"),
			tgbotapi.NewInlineKeyboardButtonData("20:00", "reminder_time:20:00"),
			tgbotapi.NewInlineKeyboardButtonData("21:00", "reminder_time:21:00"),
			tgbotapi.NewInlineKeyboardButtonData("22:00", "reminder_time:22:00"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("« Назад", "reminder_mode:back"),
		),
	)
}

func ReminderDaysKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📅 Каждый день", "reminder_days:all"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💼 Будни (пн-пт)", "reminder_days:weekdays"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🌴 Выходные (сб-вс)", "reminder_days:weekends"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✏️ Выбрать дни", "reminder_days:custom"),
		),
	)
}

func ReminderCustomDaysKeyboard(selected map[int]bool) tgbotapi.InlineKeyboardMarkup {
	days := []struct {
		num  int
		name string
	}{
		{1, "Пн"}, {2, "Вт"}, {3, "Ср"}, {4, "Чт"}, {5, "Пт"}, {6, "Сб"}, {7, "Вс"},
	}

	var row []tgbotapi.InlineKeyboardButton
	for _, d := range days {
		text := d.name
		if selected[d.num] {
			text = "✅" + d.name
		}
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(text, fmt.Sprintf("reminder_toggle_day:%d", d.num)))
	}

	return tgbotapi.NewInlineKeyboardMarkup(
		row,
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Готово", "reminder_days:done"),
		),
	)
}
