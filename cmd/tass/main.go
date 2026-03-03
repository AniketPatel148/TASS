// TASS - Token-Aware Scheduling Simulator
// CLI entry point for running LLM inference scheduling simulations.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/aniketpatel/tass/internal/config"
	"github.com/aniketpatel/tass/internal/engine"
	"github.com/aniketpatel/tass/internal/metrics"
	"github.com/aniketpatel/tass/internal/scheduler"
	"github.com/aniketpatel/tass/internal/workload"
)

func main() {
	configPath := flag.String("config", "examples/config.json", "Path to config JSON file")
	outDir := flag.String("out", "out/", "Output directory for results")
	compare := flag.Bool("compare", false, "Run all schedulers and print comparison table")
	verbose := flag.Bool("verbose", false, "Enable verbose event logging")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	if *compare {
		runComparison(cfg, *outDir, *verbose)
	} else {
		runSingle(cfg, *outDir, *verbose)
	}
}

func runSingle(cfg *config.Config, outDir string, verbose bool) {
	sched := createScheduler(cfg.Scheduler, cfg)
	summary, collector := runSimulation(cfg, sched, verbose)

	fmt.Println(summary.FormatTable())

	// Export results
	if err := exportResults(outDir, cfg.Scheduler, summary, collector); err != nil {
		log.Fatalf("Export error: %v", err)
	}
	fmt.Printf("Results written to %s\n", outDir)
}

func runComparison(cfg *config.Config, outDir string, verbose bool) {
	policies := []string{"fifo", "priority", "wfq", "srtf", "dynbatch"}
	var summaries []metrics.RunSummary

	for _, policy := range policies {
		sched := createScheduler(policy, cfg)
		summary, collector := runSimulation(cfg, sched, verbose)
		summaries = append(summaries, summary)

		// Export each policy's results
		policyDir := filepath.Join(outDir, policy)
		if err := exportResults(policyDir, policy, summary, collector); err != nil {
			log.Printf("Export error for %s: %v", policy, err)
		}
	}

	// Print comparison table
	printComparisonTable(summaries)
	fmt.Printf("\nResults written to %s\n", outDir)
}

func runSimulation(cfg *config.Config, sched scheduler.Scheduler, verbose bool) (metrics.RunSummary, *metrics.Collector) {
	collector := metrics.NewCollector(cfg.Tiers)
	eng := engine.NewEngine(cfg, sched, collector)
	eng.SetVerbose(verbose)

	// Generate workload
	requests, err := workload.Generate(cfg)
	if err != nil {
		log.Fatalf("Workload generation error: %v", err)
	}

	// Schedule all arrivals
	for _, r := range requests {
		eng.ScheduleArrival(r)
	}

	fmt.Printf("Running %s scheduler with %d requests...\n", sched.Name(), len(requests))
	eng.Run()
	fmt.Printf("Completed: %d/%d requests\n", eng.CompletedCount(), len(requests))

	// Compute summary using the engine's cluster (has actual busy-time stats)
	summary := collector.ComputeSummary(sched.Name(), eng.Clock(), eng.Cluster.Workers)

	return summary, collector
}

func createScheduler(name string, cfg *config.Config) scheduler.Scheduler {
	switch name {
	case "fifo":
		return scheduler.NewFIFO()
	case "priority":
		return scheduler.NewPriority(cfg.Tiers)
	case "wfq":
		return scheduler.NewWFQ(cfg.Tiers)
	case "srtf":
		return scheduler.NewSRTF()
	case "dynbatch":
		return scheduler.NewDynBatch(cfg.Tiers, cfg.Cluster.MaxBatchSize)
	default:
		log.Fatalf("Unknown scheduler: %s", name)
		return nil
	}
}

func exportResults(dir, schedulerName string, summary metrics.RunSummary, collector *metrics.Collector) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Export summary JSON
	summaryPath := filepath.Join(dir, "summary.json")
	if err := metrics.ExportSummaryJSON(summaryPath, summary); err != nil {
		return err
	}

	// Export per-request CSV
	if collector != nil {
		csvPath := filepath.Join(dir, "requests.csv")
		if err := metrics.ExportRequestCSV(csvPath, collector); err != nil {
			return err
		}
	}

	return nil
}

func printComparisonTable(summaries []metrics.RunSummary) {
	fmt.Println()
	fmt.Println("╔═══════════════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                         SCHEDULING POLICY COMPARISON                             ║")
	fmt.Println("╠════════════╦═════════╦═══════════╦══════════╦══════════╦══════════╦═══════════════╣")
	fmt.Printf("║ %-10s ║ %7s ║ %9s ║ %8s ║ %8s ║ %8s ║ %13s ║\n",
		"Policy", "Req", "Tok/s", "P50ms", "P95ms", "P99ms", "Fairness")
	fmt.Println("╠════════════╬═════════╬═══════════╬══════════╬══════════╬══════════╬═══════════════╣")

	for _, s := range summaries {
		fmt.Printf("║ %-10s ║ %7d ║ %9.1f ║ %8.1f ║ %8.1f ║ %8.1f ║ %13.3f ║\n",
			s.SchedulerName, s.TotalRequests, s.ThroughputTokSec,
			s.OverallP50Ms, s.OverallP95Ms, s.OverallP99Ms, s.FairnessIndex)
	}

	fmt.Println("╚════════════╩═════════╩═══════════╩══════════╩══════════╩══════════╩═══════════════╝")
}
