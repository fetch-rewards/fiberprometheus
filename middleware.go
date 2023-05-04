//
// Copyright (c) 2021-present Ankur Srivastava and Contributors
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package fiberprometheus

import (
	"strconv"
	"time"

	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// FiberPrometheus ...
type FiberPrometheus struct {
	requestsTotal   *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
	requestInFlight *prometheus.GaugeVec
	defaultURL      string
	skipPaths       []string
	fullPaths       bool
}

type Config struct {
	registry    prometheus.Registerer
	serviceName string
	namespace   string
	subsystem   string
	labels      map[string]string
	skipPaths   []string
	fullPaths   bool
}

func (c *Config) fillDefaults() {
	if c.serviceName == "" {
		c.serviceName = "my-service"
	}
	if c.registry == nil {
		c.registry = prometheus.DefaultRegisterer
	}
	if c.namespace == "" {
		c.namespace = "http"
	}
}

func create(registry prometheus.Registerer, serviceName, namespace, subsystem string, labels map[string]string, skipPaths []string, fullPaths bool) *FiberPrometheus {
	constLabels := make(prometheus.Labels)
	if serviceName != "" {
		constLabels["service"] = serviceName
	}
	for label, value := range labels {
		constLabels[label] = value
	}

	counter := promauto.With(registry).NewCounterVec(
		prometheus.CounterOpts{
			Name:        prometheus.BuildFQName(namespace, subsystem, "requests_total"),
			Help:        "Count all http requests by status code, method and path.",
			ConstLabels: constLabels,
		},
		[]string{"status_code", "method", "path"},
	)
	histogram := promauto.With(registry).NewHistogramVec(prometheus.HistogramOpts{
		Name:        prometheus.BuildFQName(namespace, subsystem, "request_duration_seconds"),
		Help:        "Duration of all HTTP requests by status code, method and path.",
		ConstLabels: constLabels,
		Buckets: []float64{
			0.000000001, // 1ns
			0.000000002,
			0.000000005,
			0.00000001, // 10ns
			0.00000002,
			0.00000005,
			0.0000001, // 100ns
			0.0000002,
			0.0000005,
			0.000001, // 1µs
			0.000002,
			0.000005,
			0.00001, // 10µs
			0.00002,
			0.00005,
			0.0001, // 100µs
			0.0002,
			0.0005,
			0.001, // 1ms
			0.002,
			0.005,
			0.01, // 10ms
			0.02,
			0.05,
			0.1, // 100 ms
			0.2,
			0.5,
			1.0, // 1s
			2.0,
			5.0,
			10.0, // 10s
			15.0,
			20.0,
			30.0,
		},
	},
		[]string{"status_code", "method", "path"},
	)

	gauge := promauto.With(registry).NewGaugeVec(prometheus.GaugeOpts{
		Name:        prometheus.BuildFQName(namespace, subsystem, "requests_in_progress_total"),
		Help:        "All the requests in progress",
		ConstLabels: constLabels,
	}, []string{"method"})

	return &FiberPrometheus{
		requestsTotal:   counter,
		requestDuration: histogram,
		requestInFlight: gauge,
		defaultURL:      "/metrics",
		skipPaths:       skipPaths,
		fullPaths:       fullPaths,
	}
}

// New creates a new instance of FiberPrometheus middleware
// serviceName is available as a const label
func New(serviceName string) *FiberPrometheus {
	return create(prometheus.DefaultRegisterer, serviceName, "http", "", nil, nil, false)
}

func NewFromConfig(config Config) *FiberPrometheus {
	config.fillDefaults()
	return create(config.registry, config.serviceName, config.namespace, config.subsystem, config.labels, config.skipPaths, config.fullPaths)
}

// NewWith creates a new instance of FiberPrometheus middleware but with an ability
// to pass namespace and a custom subsystem
// Here serviceName is created as a constant-label for the metrics
// Namespace, subsystem get prefixed to the metrics.
//
// For e.g. namespace = "my_app", subsystem = "http" then metrics would be
// `my_app_http_requests_total{...,service= "serviceName"}`
func NewWith(serviceName, namespace, subsystem string) *FiberPrometheus {
	return create(prometheus.DefaultRegisterer, serviceName, namespace, subsystem, nil, nil, false)
}

// NewWithLabels creates a new instance of FiberPrometheus middleware but with an ability
// to pass namespace and a custom subsystem
// Here labels are created as a constant-labels for the metrics
// Namespace, subsystem get prefixed to the metrics.
//
// For e.g. namespace = "my_app", subsystem = "http" and labels = map[string]string{"key1": "value1", "key2":"value2"}
// then then metrics would become
// `my_app_http_requests_total{...,key1= "value1", key2= "value2" }`
func NewWithLabels(labels map[string]string, namespace, subsystem string) *FiberPrometheus {
	return create(prometheus.DefaultRegisterer, "", namespace, subsystem, labels, nil, false)
}

// NewWithRegistry creates a new instance of FiberPrometheus middleware but with an ability
// to pass a custom registry, serviceName, namespace, subsystem and labels
// Here labels are created as a constant-labels for the metrics
// Namespace, subsystem get prefixed to the metrics.
//
// For e.g. namespace = "my_app", subsystem = "http" and labels = map[string]string{"key1": "value1", "key2":"value2"}
// then then metrics would become
// `my_app_http_requests_total{...,key1= "value1", key2= "value2" }`
func NewWithRegistry(registry prometheus.Registerer, serviceName, namespace, subsystem string, labels map[string]string) *FiberPrometheus {
	return create(registry, serviceName, namespace, subsystem, labels, nil, false)
}

// RegisterAt will register the prometheus handler at a given URL
func (ps *FiberPrometheus) RegisterAt(app fiber.Router, url string, handlers ...fiber.Handler) {
	ps.defaultURL = url

	h := append(handlers, adaptor.HTTPHandler(promhttp.Handler()))
	app.Get(ps.defaultURL, h...)
}

// Middleware is the actual default middleware implementation
func (ps *FiberPrometheus) Middleware(ctx *fiber.Ctx) error {

	start := time.Now()
	method := ctx.Route().Method

	if ctx.Route().Path == ps.defaultURL {
		return ctx.Next()
	}

	ps.requestInFlight.WithLabelValues(method).Inc()
	defer func() {
		ps.requestInFlight.WithLabelValues(method).Dec()
	}()

	err := ctx.Next()
	// initialize with default error code
	// https://docs.gofiber.io/guide/error-handling
	status := fiber.StatusInternalServerError
	if err != nil {
		if e, ok := err.(*fiber.Error); ok {
			// Get correct error code from fiber.Error type
			status = e.Code
		}
	} else {
		status = ctx.Response().StatusCode()
	}

	var path string
	if ps.fullPaths {
		path = ctx.Path()
	} else {
		path = ctx.Route().Path
	}
	statusCode := strconv.Itoa(status)
	ps.requestsTotal.WithLabelValues(statusCode, method, path).Inc()

	elapsed := float64(time.Since(start).Nanoseconds()) / 1e9
	ps.requestDuration.WithLabelValues(statusCode, method, path).Observe(elapsed)

	return err
}
