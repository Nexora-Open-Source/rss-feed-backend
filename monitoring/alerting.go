// Package monitoring provides alerting capabilities for the RSS feed backend
package monitoring

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// AlertSeverity represents the severity level of an alert
type AlertSeverity string

const (
	SeverityLow      AlertSeverity = "low"
	SeverityMedium   AlertSeverity = "medium"
	SeverityHigh     AlertSeverity = "high"
	SeverityCritical AlertSeverity = "critical"
)

// AlertType represents the type of alert
type AlertType string

const (
	AlertTypeFeedFailure    AlertType = "feed_failure"
	AlertTypeHighLatency    AlertType = "high_latency"
	AlertTypeQueueFull      AlertType = "queue_full"
	AlertTypeDatastoreError AlertType = "datastore_error"
	AlertTypeCacheFailure   AlertType = "cache_failure"
	AlertTypeWorkerDown     AlertType = "worker_down"
	AlertTypeHighErrorRate  AlertType = "high_error_rate"
)

// Alert represents an alert
type Alert struct {
	ID          string                 `json:"id"`
	Type        AlertType              `json:"type"`
	Severity    AlertSeverity          `json:"severity"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Timestamp   time.Time              `json:"timestamp"`
	Labels      map[string]string      `json:"labels"`
	Annotations map[string]interface{} `json:"annotations"`
	Resolved    bool                   `json:"resolved"`
	ResolvedAt  *time.Time             `json:"resolved_at,omitempty"`
}

// AlertManager manages alerts and notifications
type AlertManager struct {
	alerts    map[string]*Alert
	mutex     sync.RWMutex
	logger    *logrus.Logger
	rules     []AlertRule
	notifiers []Notifier
	ctx       context.Context
	cancel    context.CancelFunc
}

// AlertRule defines a rule for generating alerts
type AlertRule struct {
	Name        string
	Type        AlertType
	Severity    AlertSeverity
	Condition   func() bool
	Title       string
	Description string
	Labels      map[string]string
	Enabled     bool
	Interval    time.Duration
}

// Notifier interface for sending alert notifications
type Notifier interface {
	Send(alert *Alert) error
	Name() string
}

// LogNotifier sends alerts to the log
type LogNotifier struct {
	logger *logrus.Logger
}

func (n *LogNotifier) Name() string {
	return "log"
}

func (n *LogNotifier) Send(alert *Alert) error {
	level := logrus.InfoLevel
	switch alert.Severity {
	case SeverityHigh:
		level = logrus.WarnLevel
	case SeverityCritical:
		level = logrus.ErrorLevel
	}

	n.logger.WithFields(logrus.Fields{
		"alert_id":    alert.ID,
		"alert_type":  alert.Type,
		"severity":    alert.Severity,
		"labels":      alert.Labels,
		"annotations": alert.Annotations,
	}).Log(level, fmt.Sprintf("ALERT: %s - %s", alert.Title, alert.Description))

	return nil
}

// NewLogNotifier creates a new log notifier
func NewLogNotifier(logger *logrus.Logger) *LogNotifier {
	return &LogNotifier{logger: logger}
}

// NewAlertManager creates a new alert manager
func NewAlertManager(logger *logrus.Logger) *AlertManager {
	ctx, cancel := context.WithCancel(context.Background())

	am := &AlertManager{
		alerts:    make(map[string]*Alert),
		logger:    logger,
		rules:     getDefaultAlertRules(),
		notifiers: []Notifier{NewLogNotifier(logger)},
		ctx:       ctx,
		cancel:    cancel,
	}

	// Start alert evaluation loop
	go am.evaluateRules()

	return am
}

// getDefaultAlertRules returns default alert rules for RSS feed backend
func getDefaultAlertRules() []AlertRule {
	return []AlertRule{
		{
			Name:        "High Feed Failure Rate",
			Type:        AlertTypeHighErrorRate,
			Severity:    SeverityHigh,
			Condition:   func() bool { return false }, // Will be updated with actual metrics
			Title:       "High RSS feed failure rate detected",
			Description: "RSS feed failure rate has exceeded threshold",
			Labels:      map[string]string{"service": "rss-feed-backend"},
			Enabled:     true,
			Interval:    time.Minute * 5,
		},
		{
			Name:        "Async Queue Full",
			Type:        AlertTypeQueueFull,
			Severity:    SeverityMedium,
			Condition:   func() bool { return false }, // Will be updated with actual metrics
			Title:       "Async processing queue is full",
			Description: "The async job queue has reached capacity",
			Labels:      map[string]string{"service": "rss-feed-backend"},
			Enabled:     true,
			Interval:    time.Minute * 2,
		},
		{
			Name:        "Datastore Errors",
			Type:        AlertTypeDatastoreError,
			Severity:    SeverityHigh,
			Condition:   func() bool { return false }, // Will be updated with actual metrics
			Title:       "Datastore operation failures detected",
			Description: "Multiple datastore operations have failed",
			Labels:      map[string]string{"service": "rss-feed-backend"},
			Enabled:     true,
			Interval:    time.Minute * 3,
		},
	}
}

// evaluateRules runs the alert evaluation loop
func (am *AlertManager) evaluateRules() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-am.ctx.Done():
			return
		case <-ticker.C:
			am.evaluateAllRules()
		}
	}
}

// evaluateAllRules evaluates all enabled alert rules
func (am *AlertManager) evaluateAllRules() {
	am.mutex.RLock()
	rules := make([]AlertRule, len(am.rules))
	copy(rules, am.rules)
	am.mutex.RUnlock()

	for _, rule := range rules {
		if rule.Enabled && rule.Condition() {
			am.triggerAlert(rule)
		}
	}
}

// triggerAlert creates and sends an alert
func (am *AlertManager) triggerAlert(rule AlertRule) {
	alertID := fmt.Sprintf("%s-%d", rule.Type, time.Now().Unix())

	alert := &Alert{
		ID:          alertID,
		Type:        rule.Type,
		Severity:    rule.Severity,
		Title:       rule.Title,
		Description: rule.Description,
		Timestamp:   time.Now(),
		Labels:      rule.Labels,
		Annotations: make(map[string]interface{}),
		Resolved:    false,
	}

	am.mutex.Lock()
	// Check if we already have an active alert of this type
	for _, existingAlert := range am.alerts {
		if existingAlert.Type == rule.Type && !existingAlert.Resolved {
			am.mutex.Unlock()
			return // Alert already active
		}
	}
	am.alerts[alertID] = alert
	am.mutex.Unlock()

	am.sendNotifications(alert)
}

// sendNotifications sends the alert to all notifiers
func (am *AlertManager) sendNotifications(alert *Alert) {
	for _, notifier := range am.notifiers {
		if err := notifier.Send(alert); err != nil {
			am.logger.WithError(err).WithField("notifier", notifier.Name()).Error("Failed to send alert notification")
		}
	}
}

// TriggerManualAlert manually triggers an alert
func (am *AlertManager) TriggerManualAlert(alertType AlertType, severity AlertSeverity, title, description string, labels map[string]string) {
	alertID := fmt.Sprintf("%s-%d", alertType, time.Now().Unix())

	alert := &Alert{
		ID:          alertID,
		Type:        alertType,
		Severity:    severity,
		Title:       title,
		Description: description,
		Timestamp:   time.Now(),
		Labels:      labels,
		Annotations: make(map[string]interface{}),
		Resolved:    false,
	}

	am.mutex.Lock()
	am.alerts[alertID] = alert
	am.mutex.Unlock()

	am.sendNotifications(alert)
}

// ResolveAlert resolves an alert
func (am *AlertManager) ResolveAlert(alertID string) {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	if alert, exists := am.alerts[alertID]; exists {
		now := time.Now()
		alert.Resolved = true
		alert.ResolvedAt = &now

		am.logger.WithFields(logrus.Fields{
			"alert_id": alertID,
			"type":     alert.Type,
		}).Info("Alert resolved")
	}
}

// GetActiveAlerts returns all active (unresolved) alerts
func (am *AlertManager) GetActiveAlerts() []*Alert {
	am.mutex.RLock()
	defer am.mutex.RUnlock()

	var activeAlerts []*Alert
	for _, alert := range am.alerts {
		if !alert.Resolved {
			activeAlerts = append(activeAlerts, alert)
		}
	}

	return activeAlerts
}

// AddNotifier adds a new notifier
func (am *AlertManager) AddNotifier(notifier Notifier) {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	am.notifiers = append(am.notifiers, notifier)
}

// UpdateRuleCondition updates the condition function for a rule
func (am *AlertManager) UpdateRuleCondition(ruleName string, condition func() bool) {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	for i, rule := range am.rules {
		if rule.Name == ruleName {
			am.rules[i].Condition = condition
			break
		}
	}
}

// Stop stops the alert manager
func (am *AlertManager) Stop() {
	am.cancel()
}

// GetFeedFailureRate returns the current feed failure rate (placeholder)
func GetFeedFailureRate() float64 {
	// This would be calculated from actual metrics
	// For now, return 0
	return 0.0
}

// GetAsyncQueueSize returns the current async queue size (placeholder)
func GetAsyncQueueSize() int {
	// This would get the actual queue size
	// For now, return 0
	return 0
}

// GetDatastoreErrorRate returns the current datastore error rate (placeholder)
func GetDatastoreErrorRate() float64 {
	// This would be calculated from actual metrics
	// For now, return 0
	return 0.0
}
