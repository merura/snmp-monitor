# SNMP Interface Monitor

Real-time network interface monitoring via SNMP (IF-MIB).

## Build

```bash
go build -o snmp-monitor .
```

## Usage

```bash
./snmp-monitor                      # monitor localhost
./snmp-monitor -host 192.168.1.1    # monitor remote host
./snmp-monitor -interval 2s         # poll every 2 seconds
```

### Flags

```
-host        SNMP agent host (default: 127.0.0.1)
-port        SNMP port (default: 161)
-community   community string (default: public)
-interval    polling interval (default: 1s)
-timeout     SNMP timeout (default: 2s)
-retries     retry count (default: 1)
-v1          use SNMP v1 instead of v2c
```

## Requirements

SNMP agent must be running on target host. On Windows, use Net-SNMP. On Linux, install `snmpd`.

## Testing

Run `speedtest-cli` while monitoring to see live traffic rates.
