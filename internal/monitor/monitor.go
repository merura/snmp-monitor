package monitor

import (
	"sync"
	"time"

	"snmp-monitor/internal/snmp"
)

type InterfaceMetrics struct {
	Index    int
	Name     string
	Status   snmp.OperStatus
	Speed    uint64  // максимальная скорость интерфейса (бит/сек)
	InRate   float64 // текущая входящая скорость (байт/сек)
	OutRate  float64 // текущая исходящая скорость (байт/сек)
	InTotal  uint64  // всего получено байт
	OutTotal uint64  // всего отправлено байт
	Use64Bit bool    // используются 64-битные счётчики
}

// interfaceState хранит состояние для расчёта скорости
type interfaceState struct {
	lastInOctets  uint64
	lastOutOctets uint64
	lastTime      time.Time
	use64Bit      bool
}

type Monitor struct {
	client   *snmp.Client
	interval time.Duration
	states   map[int]*interfaceState // состояние по индексу интерфейса
	mu       sync.RWMutex
}

func NewMonitor(client *snmp.Client, interval time.Duration) *Monitor {
	return &Monitor{
		client:   client,
		interval: interval,
		states:   make(map[int]*interfaceState),
	}
}

func (m *Monitor) Collect() ([]InterfaceMetrics, error) {
	interfaces, err := m.client.GetInterfaces()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	metrics := make([]InterfaceMetrics, 0, len(interfaces))

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, iface := range interfaces {
		metric := InterfaceMetrics{
			Index:    iface.Index,
			Name:     iface.Name,
			Status:   iface.Status,
			Speed:    iface.Speed,
			InTotal:  iface.InOctets,
			OutTotal: iface.OutOctets,
			Use64Bit: iface.Use64Bit,
		}

		// Рассчитываем скорость если есть предыдущее состояние
		if state, exists := m.states[iface.Index]; exists {
			elapsed := now.Sub(state.lastTime).Seconds()
			if elapsed > 0 {
				// Рассчитываем дельту с учётом возможного переполнения счётчика
				inDelta := calculateDelta(state.lastInOctets, iface.InOctets, iface.Use64Bit)
				outDelta := calculateDelta(state.lastOutOctets, iface.OutOctets, iface.Use64Bit)

				metric.InRate = float64(inDelta) / elapsed
				metric.OutRate = float64(outDelta) / elapsed
			}
		}

		// Обновляем состояние
		m.states[iface.Index] = &interfaceState{
			lastInOctets:  iface.InOctets,
			lastOutOctets: iface.OutOctets,
			lastTime:      now,
			use64Bit:      iface.Use64Bit,
		}

		metrics = append(metrics, metric)
	}

	return metrics, nil
}

// calculateDelta рассчитывает дельту с учётом переполнения счётчика
func calculateDelta(prev, curr uint64, use64Bit bool) uint64 {
	if curr >= prev {
		return curr - prev
	}

	// Счётчик переполнился (counter wrap)
	if use64Bit {
		// 64-битный счётчик: маловероятно, но обработаем
		return (^uint64(0) - prev) + curr + 1
	}
	// 32-битный счётчик
	return (uint64(^uint32(0)) - prev) + curr + 1
}
