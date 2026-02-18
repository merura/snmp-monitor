package snmp

import (
	"fmt"
	"time"

	"github.com/gosnmp/gosnmp"
)

// OID константы для IF-MIB
const (
	OIDIfNumber     = "1.3.6.1.2.1.2.1.0"    // Количество интерфейсов
	OIDIfDescr      = "1.3.6.1.2.1.2.2.1.2"  // Имя интерфейса
	OIDIfType       = "1.3.6.1.2.1.2.2.1.3"  // Тип интерфейса
	OIDIfSpeed      = "1.3.6.1.2.1.2.2.1.5"  // Скорость интерфейса
	OIDIfOperStatus = "1.3.6.1.2.1.2.2.1.8"  // Статус (1=up, 2=down, 3=testing)
	OIDIfInOctets   = "1.3.6.1.2.1.2.2.1.10" // Входящие байты
	OIDIfOutOctets  = "1.3.6.1.2.1.2.2.1.16" // Исходящие байты

	// 64-битные счётчики (для высокоскоростных интерфейсов)
	OIDIfHCInOctets  = "1.3.6.1.2.1.31.1.1.1.6"  // Входящие байты (64-bit)
	OIDIfHCOutOctets = "1.3.6.1.2.1.31.1.1.1.10" // Исходящие байты (64-bit)
	OIDIfName        = "1.3.6.1.2.1.31.1.1.1.1"  // Имя интерфейса (ifXTable)
)

type OperStatus int

const (
	StatusUp      OperStatus = 1
	StatusDown    OperStatus = 2
	StatusTesting OperStatus = 3
	StatusUnknown OperStatus = 4
	StatusDormant OperStatus = 5
)

func (s OperStatus) String() string {
	switch s {
	case StatusUp:
		return "UP"
	case StatusDown:
		return "DOWN"
	case StatusTesting:
		return "TESTING"
	case StatusDormant:
		return "DORMANT"
	default:
		return "UNKNOWN"
	}
}

// Client обёртка над gosnmp для работы с IF-MIB
type Client struct {
	snmp *gosnmp.GoSNMP
}

type Config struct {
	Host      string
	Port      uint16
	Community string
	Version   gosnmp.SnmpVersion
	Timeout   time.Duration
	Retries   int
}

func NewClient(cfg Config) (*Client, error) {
	client := &gosnmp.GoSNMP{
		Target:    cfg.Host,
		Port:      cfg.Port,
		Community: cfg.Community,
		Version:   cfg.Version,
		Timeout:   cfg.Timeout,
		Retries:   cfg.Retries,
	}

	if err := client.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	return &Client{snmp: client}, nil
}

func (c *Client) Close() error {
	return c.snmp.Conn.Close()
}

type InterfaceStats struct {
	Index     int
	Name      string
	Status    OperStatus
	Speed     uint64 // бит/сек
	InOctets  uint64
	OutOctets uint64
	Use64Bit  bool // флаг использования 64-битных счётчиков
}

func (c *Client) GetInterfaces() ([]InterfaceStats, error) {
	names, err := c.walkString(OIDIfDescr)
	if err != nil {
		return nil, fmt.Errorf("failed to get interface names: %w", err)
	}

	interfaces := make([]InterfaceStats, 0, len(names))

	for idx, name := range names {
		ifIndex := idx + 1 // SNMP индексы начинаются с 1

		stats := InterfaceStats{
			Index: ifIndex,
			Name:  name,
		}

		if status, err := c.getInt(fmt.Sprintf("%s.%d", OIDIfOperStatus, ifIndex)); err == nil {
			stats.Status = OperStatus(status)
		}

		if speed, err := c.getUint(fmt.Sprintf("%s.%d", OIDIfSpeed, ifIndex)); err == nil {
			stats.Speed = speed
		}

		// Пробуем 64-битные счётчики сначала
		inOctets, errIn := c.getUint64(fmt.Sprintf("%s.%d", OIDIfHCInOctets, ifIndex))
		outOctets, errOut := c.getUint64(fmt.Sprintf("%s.%d", OIDIfHCOutOctets, ifIndex))

		if errIn == nil && errOut == nil {
			stats.InOctets = inOctets
			stats.OutOctets = outOctets
			stats.Use64Bit = true
		} else {
			if in, err := c.getUint(fmt.Sprintf("%s.%d", OIDIfInOctets, ifIndex)); err == nil {
				stats.InOctets = in
			}
			if out, err := c.getUint(fmt.Sprintf("%s.%d", OIDIfOutOctets, ifIndex)); err == nil {
				stats.OutOctets = out
			}
		}

		interfaces = append(interfaces, stats)
	}

	return interfaces, nil
}

func (c *Client) walkString(oid string) ([]string, error) {
	results, err := c.snmp.WalkAll(oid)
	if err != nil {
		return nil, err
	}

	values := make([]string, 0, len(results))
	for _, result := range results {
		switch v := result.Value.(type) {
		case []byte:
			values = append(values, string(v))
		case string:
			values = append(values, v)
		default:
			values = append(values, fmt.Sprintf("%v", v))
		}
	}

	return values, nil
}

func (c *Client) getInt(oid string) (int, error) {
	result, err := c.snmp.Get([]string{oid})
	if err != nil {
		return 0, err
	}

	if len(result.Variables) == 0 {
		return 0, fmt.Errorf("no result for OID %s", oid)
	}

	return int(gosnmp.ToBigInt(result.Variables[0].Value).Int64()), nil
}

func (c *Client) getUint(oid string) (uint64, error) {
	result, err := c.snmp.Get([]string{oid})
	if err != nil {
		return 0, err
	}

	if len(result.Variables) == 0 {
		return 0, fmt.Errorf("no result for OID %s", oid)
	}

	return gosnmp.ToBigInt(result.Variables[0].Value).Uint64(), nil
}

func (c *Client) getUint64(oid string) (uint64, error) {
	result, err := c.snmp.Get([]string{oid})
	if err != nil {
		return 0, err
	}

	if len(result.Variables) == 0 {
		return 0, fmt.Errorf("no result for OID %s", oid)
	}

	pdu := result.Variables[0]
	if pdu.Type == gosnmp.NoSuchObject || pdu.Type == gosnmp.NoSuchInstance {
		return 0, fmt.Errorf("OID %s not supported", oid)
	}

	return gosnmp.ToBigInt(pdu.Value).Uint64(), nil
}
