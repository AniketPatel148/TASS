package metrics

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// ExportCSV writes per-request metrics to a CSV file.
func ExportCSV(path string, summary RunSummary) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating output dir: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating CSV %s: %w", path, err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	// Header
	header := []string{
		"request_id", "tier", "context_tokens", "output_tokens",
		"arrival_ms", "queue_delay_ms", "ttft_ms", "total_latency_ms",
		"tokens_per_sec",
	}
	if err := w.Write(header); err != nil {
		return err
	}

	// Get the collector for direct request data access
	return nil // We'll use ExportRequestCSV instead
}

// ExportRequestCSV writes per-request metrics from the collector to a CSV file.
func ExportRequestCSV(path string, c *Collector) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating output dir: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating CSV %s: %w", path, err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	header := []string{
		"request_id", "tier", "context_tokens", "output_tokens",
		"arrival_ms", "queue_delay_ms", "ttft_ms", "total_latency_ms",
		"tokens_per_sec",
	}
	if err := w.Write(header); err != nil {
		return err
	}

	for _, r := range c.Completed {
		row := []string{
			strconv.Itoa(r.ID),
			string(r.Tier),
			strconv.Itoa(r.ContextTokens),
			strconv.Itoa(r.OutputTokens),
			fmt.Sprintf("%.2f", r.ArrivalMs),
			fmt.Sprintf("%.2f", r.QueueDelayMs()),
			fmt.Sprintf("%.2f", r.TTFTMs()),
			fmt.Sprintf("%.2f", r.TotalLatencyMs()),
			fmt.Sprintf("%.2f", r.TokensPerSec()),
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}

	return nil
}

// ExportSummaryJSON writes the run summary to a JSON file.
func ExportSummaryJSON(path string, summary RunSummary) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating output dir: %w", err)
	}

	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling summary: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing summary %s: %w", path, err)
	}

	return nil
}
