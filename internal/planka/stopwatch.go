package planka

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"
)

// StartCardStopwatch starts the stopwatch on a card, preserving any existing
// accumulated total. PROPAGATE errors.
func (c *Client) StartCardStopwatch(ctx context.Context, id string) (*Card, error) {
	card, err := c.GetCard(ctx, id)
	if err != nil {
		return nil, err
	}

	total := float64(0)
	if card.Stopwatch != nil {
		total = card.Stopwatch.Total
	}

	nowISO := time.Now().UTC().Format(time.RFC3339)
	sw := map[string]any{
		"startedAt": nowISO,
		"total":     total,
	}

	body := map[string]any{"stopwatch": sw}
	var env itemEnvelope[Card]
	if err := c.Patch(ctx, "/api/cards/"+id, body, &env); err != nil {
		return nil, err
	}
	return &env.Item, nil
}

// StopCardStopwatch stops the running stopwatch and accumulates elapsed time.
// If the stopwatch is not running, returns the card unchanged (no PATCH).
// PROPAGATE errors.
func (c *Client) StopCardStopwatch(ctx context.Context, id string) (*Card, error) {
	card, err := c.GetCard(ctx, id)
	if err != nil {
		return nil, err
	}

	if card.Stopwatch == nil || card.Stopwatch.StartedAt == nil {
		return card, nil
	}

	parsed, err := time.Parse(time.RFC3339, *card.Stopwatch.StartedAt)
	if err != nil {
		return nil, fmt.Errorf("parse stopwatch startedAt: %w", err)
	}
	elapsed := int(time.Since(parsed).Seconds())
	total := card.Stopwatch.Total + float64(elapsed)

	sw := map[string]any{
		"startedAt": nil,
		"total":     total,
	}
	body := map[string]any{"stopwatch": sw}
	var env itemEnvelope[Card]
	if err := c.Patch(ctx, "/api/cards/"+id, body, &env); err != nil {
		return nil, err
	}
	return &env.Item, nil
}

// GetCardStopwatch returns a rich stopwatch status for a card, including
// formatted duration strings. PROPAGATE errors.
func (c *Client) GetCardStopwatch(ctx context.Context, id string) (any, error) {
	card, err := c.GetCard(ctx, id)
	if err != nil {
		return nil, err
	}

	if card.Stopwatch == nil {
		return map[string]any{
			"isRunning":        false,
			"total":            float64(0),
			"current":          float64(0),
			"formattedTotal":   formatDuration(0),
			"formattedCurrent": formatDuration(0),
		}, nil
	}

	sw := card.Stopwatch
	isRunning := sw.StartedAt != nil
	current := float64(0)
	if isRunning && sw.StartedAt != nil {
		parsed, err := time.Parse(time.RFC3339, *sw.StartedAt)
		if err == nil {
			current = float64(int(time.Since(parsed).Seconds()))
		}
	}

	result := map[string]any{
		"isRunning":        isRunning,
		"total":            sw.Total,
		"current":          current,
		"formattedTotal":   formatDuration(sw.Total),
		"formattedCurrent": formatDuration(current),
	}
	if sw.StartedAt != nil {
		result["startedAt"] = *sw.StartedAt
	}
	return result, nil
}

// ResetCardStopwatch clears the stopwatch on a card (sets it to null).
// PROPAGATE errors.
func (c *Client) ResetCardStopwatch(ctx context.Context, id string) (*Card, error) {
	body := map[string]any{"stopwatch": nil}
	var env itemEnvelope[Card]
	if err := c.Patch(ctx, "/api/cards/"+id, body, &env); err != nil {
		return nil, err
	}
	return &env.Item, nil
}

// formatDuration formats a total-seconds value into "Xh Ym Zs" (ports
// formatDuration from cards.ts). Hours are omitted when zero; minutes are
// omitted when zero and no hours present; seconds always shown.
func formatDuration(totalSeconds float64) string {
	s := int(math.Floor(totalSeconds))
	h := s / 3600
	m := (s % 3600) / 60
	rem := s % 60

	var parts []string
	if h > 0 {
		parts = append(parts, fmt.Sprintf("%dh", h))
	}
	if m > 0 || h > 0 {
		parts = append(parts, fmt.Sprintf("%dm", m))
	}
	parts = append(parts, fmt.Sprintf("%ds", rem))
	return strings.TrimSpace(strings.Join(parts, " "))
}
