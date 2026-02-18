package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	termui "github.com/gizak/termui/v3"
	"github.com/gosnmp/gosnmp"

	"snmp-monitor/internal/monitor"
	"snmp-monitor/internal/snmp"
	"snmp-monitor/internal/ui"
)

const version = "1.0.0"

var (
	host      = flag.String("host", "127.0.0.1", "SNMP agent host")
	port      = flag.Uint("port", 161, "SNMP agent port")
	community = flag.String("community", "public", "SNMP community string")
	interval  = flag.Duration("interval", 1*time.Second, "Polling interval")
	timeout   = flag.Duration("timeout", 2*time.Second, "SNMP timeout")
	retries   = flag.Int("retries", 1, "SNMP retries")
	snmpV1    = flag.Bool("v1", false, "Use SNMP v1 (default: v2c)")
	showVer   = flag.Bool("version", false, "Show version")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `SNMP Interface Monitor v%s

Usage: snmp-monitor [options]

Options:
`, version)
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Examples:
  snmp-monitor                         # Monitor localhost with defaults
  snmp-monitor -host 192.168.1.1       # Monitor remote host
  snmp-monitor -interval 2s            # Poll every 2 seconds
  snmp-monitor -community private      # Use different community string

`)
	}
	flag.Parse()

	if *showVer {
		fmt.Printf("snmp-monitor version %s\n", version)
		os.Exit(0)
	}

	cfg := snmp.Config{
		Host:      *host,
		Port:      uint16(*port),
		Community: *community,
		Timeout:   *timeout,
		Retries:   *retries,
		Version:   gosnmp.Version2c,
	}

	if *snmpV1 {
		cfg.Version = gosnmp.Version1
	}

	client, err := snmp.NewClient(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to connect to SNMP agent: %v\n", err)
		fmt.Fprintf(os.Stderr, "\nMake sure snmpd is running on %s:%d\n", *host, *port)
		os.Exit(1)
	}
	defer client.Close()

	mon := monitor.NewMonitor(client, *interval)
	display := ui.NewDisplay(false)

	if err := display.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to initialize UI: %v\n", err)
		os.Exit(1)
	}
	defer display.Close()

	uiEvents := display.PollEvents()
	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	hostStr := fmt.Sprintf("%s:%d", *host, *port)

	metrics, err := mon.Collect()
	if err != nil {
		display.RenderError(err)
	} else {
		display.Render(metrics, hostStr, *interval)
	}

	// Main loop
	for {
		select {
		case e := <-uiEvents:
			switch e.Type {
			case termui.KeyboardEvent:
				if e.ID == "q" || e.ID == "<C-c>" {
					return
				}
				// Any other key - just continue
			case termui.ResizeEvent:
				display.HandleResize()
				if metrics != nil {
					display.Render(metrics, hostStr, *interval)
				}
			}
		case <-ticker.C:
			newMetrics, collectErr := mon.Collect()
			if collectErr != nil {
				// Keep showing last good data if available
				if metrics != nil {
					display.Render(metrics, hostStr, *interval)
				}
			} else {
				metrics = newMetrics
				display.Render(metrics, hostStr, *interval)
			}
		}
	}
}
