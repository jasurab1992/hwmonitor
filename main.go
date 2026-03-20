package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"hwmonitor/internal/collectors"
	"hwmonitor/internal/config"
	"hwmonitor/internal/exporter"
	"hwmonitor/internal/ui"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	mode := flag.String("mode", "tui", "run mode: tui, exporter, both")
	flag.Parse()

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	var colls []collectors.Collector

	if cfg.Collectors.CPU {
		colls = append(colls, collectors.NewCPUCollector())
	}
	if cfg.Collectors.Memory {
		colls = append(colls, collectors.NewMemoryCollector())
	}
	if cfg.Collectors.Disk {
		colls = append(colls, collectors.NewDiskCollector())
	}
	if cfg.Collectors.NVMe {
		colls = append(colls, collectors.NewNVMeCollector())
	}
	if cfg.Collectors.CPUTemp {
		colls = append(colls, collectors.NewCPUTempCollector())
	}
	if cfg.Collectors.SMART {
		colls = append(colls, collectors.NewSMARTCollector())
	}
	if cfg.Collectors.Network {
		colls = append(colls, collectors.NewNetworkCollector())
	}
	if cfg.Collectors.SysInfo {
		colls = append(colls, collectors.NewSysInfoCollector())
	}

	if len(colls) == 0 {
		log.Fatal("no collectors enabled in config")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle Ctrl+C
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Shutting down...")
		cancel()
	}()

	switch *mode {
	case "tui":
		t := ui.NewTUI(colls, cfg.CollectInterval)
		if err := t.Run(ctx); err != nil {
			log.Fatalf("TUI error: %v", err)
		}

	case "exporter":
		exp := exporter.NewExporter(colls, cfg.PrometheusPort)
		if err := exp.Start(ctx); err != nil {
			log.Fatalf("exporter error: %v", err)
		}

	case "both":
		exp := exporter.NewExporter(colls, cfg.PrometheusPort)
		go func() {
			if err := exp.Start(ctx); err != nil {
				log.Printf("exporter error: %v", err)
				cancel()
			}
		}()

		t := ui.NewTUI(colls, cfg.CollectInterval)
		if err := t.Run(ctx); err != nil {
			log.Fatalf("TUI error: %v", err)
		}

	default:
		fmt.Fprintf(os.Stderr, "unknown mode: %s (use tui, exporter, or both)\n", *mode)
		os.Exit(1)
	}
}
