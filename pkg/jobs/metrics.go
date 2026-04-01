package jobs

import "github.com/prometheus/client_golang/prometheus"

type metrics struct {
	enqueued       *prometheus.CounterVec
	processed      *prometheus.CounterVec
	stored         prometheus.Gauge
	runningWorkers prometheus.Gauge
}

func newMetrics(name string, reg prometheus.Registerer) (*metrics, error) {
	if reg == nil {
		return nil, nil
	}

	m := &metrics{
		enqueued: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "corelab",
				Subsystem: "jobs",
				Name:      "enqueued_total",
				Help:      "Total number of jobs enqueued.",
				ConstLabels: prometheus.Labels{
					"manager": name,
				},
			},
			[]string{"queue", "type"},
		),
		processed: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "corelab",
				Subsystem: "jobs",
				Name:      "processed_total",
				Help:      "Total number of processed jobs by outcome.",
				ConstLabels: prometheus.Labels{
					"manager": name,
				},
			},
			[]string{"queue", "type", "status"},
		),
		stored: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "corelab",
				Subsystem: "jobs",
				Name:      "jobs_stored",
				Help:      "Current number of stored jobs.",
				ConstLabels: prometheus.Labels{
					"manager": name,
				},
			},
		),
		runningWorkers: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "corelab",
				Subsystem: "jobs",
				Name:      "running_workers",
				Help:      "Current number of actively executing workers.",
				ConstLabels: prometheus.Labels{
					"manager": name,
				},
			},
		),
	}

	if err := reg.Register(m.enqueued); err != nil {
		return nil, err
	}
	if err := reg.Register(m.processed); err != nil {
		return nil, err
	}
	if err := reg.Register(m.stored); err != nil {
		return nil, err
	}
	if err := reg.Register(m.runningWorkers); err != nil {
		return nil, err
	}

	return m, nil
}
