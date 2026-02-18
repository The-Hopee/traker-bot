package telegram

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

// ChartData — данные для графика
type ChartData struct {
	Labels []string
	Values []int
	Marks  []string // "✅" или "❌" для каждого дня
}

// GenerateWeeklyChart — генерирует URL графика за неделю
func GenerateWeeklyChart(data ChartData) string {
	// Формируем конфиг для QuickChart
	labelsJSON := `["` + strings.Join(data.Labels, `","`) + `"]`
	valuesJSON := intsToString(data.Values)

	chartConfig := fmt.Sprintf(`{
    "type": "bar",
    "data": {
      "labels": %s,
      "datasets": [{
        "label": "Выполнено",
        "data": [%s],
        "backgroundColor": "rgba(75, 192, 192, 0.8)",
        "borderColor": "rgba(75, 192, 192, 1)",
        "borderWidth": 1
      }]
    },
    "options": {
      "scales": {
        "y": {
          "beginAtZero": true,
          "ticks": {"stepSize": 1}
        }
      },
      "plugins": {
        "legend": {"display": false},
        "title": {
          "display": true,
          "text": "Привычки за неделю"
        }
      }
    }
  }`, labelsJSON, valuesJSON)

	return "https://quickchart.io/chart?c=" + url.QueryEscape(chartConfig)
}

// GenerateHabitCalendar — генерирует "календарь" привычки (30 дней)
func GenerateHabitCalendar(habitName string, completedDays map[string]bool) string {
	// Собираем данные за 30 дней
	var labels []string
	var colors []string

	now := time.Now()
	for i := 29; i >= 0; i-- {
		date := now.AddDate(0, 0, -i)
		dateStr := date.Format("2006-01-02")
		dayLabel := date.Format("02")

		labels = append(labels, dayLabel)

		if completedDays[dateStr] {
			colors = append(colors, "rgba(75, 192, 192, 0.9)") // зелёный
		} else {
			colors = append(colors, "rgba(255, 99, 132, 0.5)") // красный
		}
	}

	labelsJSON := `["` + strings.Join(labels, `","`) + `"]`
	colorsJSON := strings.Join(colors, ",")

	// Все значения = 1 для одинаковой высоты
	values := make([]string, 30)
	for i := range values {
		values[i] = "1"
	}
	valuesJSON := strings.Join(values, ",")

	chartConfig := fmt.Sprintf(`{
    "type": "bar",
    "data": {
      "labels": %s,
      "datasets": [{
        "data": [%s],
        "backgroundColor": [%s],
        "borderWidth": 0
      }]
    },
    "options": {
      "scales": {
        "y": {"display": false},
        "x": {"grid": {"display": false}}
      },
      "plugins": {
        "legend": {"display": false},
        "title": {
          "display": true,
          "text": "%s — последние 30 дней"
        }
      }
    }
  }`, labelsJSON, valuesJSON, colorsJSON, habitName)

	return "https://quickchart.io/chart?c=" + url.QueryEscape(chartConfig)
}

// GenerateStreakChart — график серий
func GenerateStreakChart(habits []HabitStreakData) string {
	if len(habits) == 0 {
		return ""
	}

	var labels []string
	var values []string

	for _, h := range habits {
		labels = append(labels, h.Name)
		values = append(values, fmt.Sprintf("%d", h.Streak))
	}

	labelsJSON := `["` + strings.Join(labels, `","`) + `"]`
	valuesJSON := strings.Join(values, ",")

	chartConfig := fmt.Sprintf(`{
    "type": "horizontalBar",
    "data": {
      "labels": %s,
      "datasets": [{
        "label": "Дней подряд",
        "data": [%s],
        "backgroundColor": "rgba(255, 159, 64, 0.8)"
      }]
    },
    "options": {
      "scales": {
        "x": {"beginAtZero": true}
      },
      "plugins": {
        "legend": {"display": false},
        "title": {
          "display": true,
          "text": "🔥 Текущие серии"
        }
      }
    }
  }`, labelsJSON, valuesJSON)

	return "https://quickchart.io/chart?c=" + url.QueryEscape(chartConfig)
}

type HabitStreakData struct {
	Name   string
	Streak int
}

func intsToString(data []int) string {
	strs := make([]string, len(data))
	for i, v := range data {
		strs[i] = fmt.Sprintf("%d", v)
	}
	return strings.Join(strs, ",")
}
