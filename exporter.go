package main

import (
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/robfig/cron/v3"
)

// Exporter collects SQL metrics. It implements prometheus.Collector.
type Exporter struct {
	jobs          []*Job
	logger        log.Logger
	cronScheduler *cron.Cron
}

// NewExporter returns a new SQL Exporter for the provided config.
func NewExporter(logger log.Logger, configFile string) (*Exporter, error) {
	if configFile == "" {
		configFile = "config.yml"
	}

	// read config
	cfg, err := Read(configFile)
	if err != nil {
		return nil, err
	}

	exp := &Exporter{
		jobs:          make([]*Job, 0, len(cfg.Jobs)),
		logger:        logger,
		cronScheduler: cron.New(),
	}

	// dispatch all jobs
	for _, job := range cfg.Jobs {
		if job == nil {
			continue
		}

		if err := job.Init(logger, cfg.Queries); err != nil {
			level.Warn(logger).Log("msg", "Skipping job. Failed to initialize", "err", err, "job", job.Name)
			continue
		}
		exp.jobs = append(exp.jobs, job)
		if job.CronSchedule.schedule != nil {
			exp.cronScheduler.Schedule(job.CronSchedule.schedule, job)
			level.Info(logger).Log("msg", "Scheduled CRON job", "name", job.Name, "cron_schedule", job.CronSchedule.definition)
		} else {
			go job.ExecutePeriodically()
			level.Info(logger).Log("msg", "Started periodically execution of job", "name", job.Name, "interval", job.Interval)
		}
	}
	exp.cronScheduler.Start()
	return exp, nil
}

// Describe implements prometheus.Collector
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	for _, job := range e.jobs {
		if job == nil {
			continue
		}
		for _, query := range job.Queries {
			if query == nil {
				continue
			}
			if query.desc == nil {
				level.Error(e.logger).Log("msg", "Query has no descriptor", "query", query.Name)
				continue
			}
			ch <- query.desc
		}
	}
}

// Collect implements prometheus.Collector
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	for _, job := range e.jobs {
		if job == nil {
			continue
		}
		for _, query := range job.Queries {
			if query == nil {
				continue
			}
			for _, metrics := range query.metrics {
				for _, metric := range metrics {
					ch <- metric
				}
			}
		}
	}
}
