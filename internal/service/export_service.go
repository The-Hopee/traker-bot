package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"time"

	"habit-tracker-bot/internal/domain"
	"habit-tracker-bot/internal/repository"
)

type ExportService struct {
	repo repository.Repository
}

func NewExportService(repo repository.Repository) *ExportService {
	return &ExportService{repo: repo}
}

func (s *ExportService) ExportToCSV(ctx context.Context, userID int64) ([]byte, error) {
	data, err := s.repo.GetAllUserData(ctx, userID)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	writer.Write([]string{"HABIT TRACKER EXPORT"})
	writer.Write([]string{fmt.Sprintf("User: %s", data.User.FirstName)})
	writer.Write([]string{fmt.Sprintf("Exported: %s", time.Now().Format("2006-01-02 15:04:05"))})
	writer.Write([]string{""})

	writer.Write([]string{"=== HABITS ==="})
	writer.Write([]string{"ID", "Name", "Frequency", "Created"})
	for _, h := range data.Habits {
		writer.Write([]string{
			fmt.Sprintf("%d", h.ID),
			h.Name,
			string(h.Frequency),
			h.CreatedAt.Format("2006-01-02"),
		})
	}
	writer.Write([]string{""})

	writer.Write([]string{"=== HABIT LOGS ==="})
	writer.Write([]string{"Date", "Habit ID", "Completed"})
	for _, l := range data.Logs {
		completed := "No"
		if l.Completed {
			completed = "Yes"
		}
		writer.Write([]string{
			l.Date.Format("2006-01-02"),
			fmt.Sprintf("%d", l.HabitID),
			completed,
		})
	}
	writer.Write([]string{""})

	writer.Write([]string{"=== STATISTICS ==="})
	writer.Write([]string{"Habit", "Current Streak", "Best Streak", "Completion %"})
	for _, st := range data.Stats {
		writer.Write([]string{
			st.HabitName,
			fmt.Sprintf("%d days", st.CurrentStreak),
			fmt.Sprintf("%d days", st.BestStreak),
			fmt.Sprintf("%.1f%%", st.CompletionRate),
		})
	}
	writer.Write([]string{""})

	writer.Write([]string{"=== ACHIEVEMENTS ==="})
	writer.Write([]string{"Achievement", "Unlocked"})
	for _, a := range data.Achievements {
		cfg := domain.GetAchievementConfig(a.Type)
		if cfg != nil {
			writer.Write([]string{
				cfg.Title,
				a.UnlockedAt.Format("2006-01-02"),
			})
		}
	}

	writer.Flush()
	return buf.Bytes(), nil
}
