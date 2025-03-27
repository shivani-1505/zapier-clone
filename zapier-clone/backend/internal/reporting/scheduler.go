// backend/internal/reporting/scheduler.go
package reporting

import (
	"log"
	"time"

	"github.com/shivani-1505/zapier-clone/backend/internal/integrations/servicenow"
	"github.com/shivani-1505/zapier-clone/backend/internal/integrations/slack"
)

// ReportScheduler schedules and runs periodic reports
type ReportScheduler struct {
	ReportingHandler *servicenow.ReportingHandler
	running          bool
	stopChan         chan struct{}
}

// NewReportScheduler creates a new report scheduler
func NewReportScheduler(serviceNowClient *servicenow.Client, slackClient *slack.Client) *ReportScheduler {
	return &ReportScheduler{
		ReportingHandler: servicenow.NewReportingHandler(serviceNowClient, slackClient),
		running:          false,
		stopChan:         make(chan struct{}),
	}
}

// Start begins running the scheduled reports
func (s *ReportScheduler) Start() {
	if s.running {
		return
	}

	s.running = true

	// Run the weekly summary report every Monday at 9:00 AM
	go s.scheduleWeeklySummary()

	// Run the risk category report every Wednesday at 9:00 AM
	go s.scheduleRiskCategorySummary()
}

// Stop stops the scheduler
func (s *ReportScheduler) Stop() {
	if !s.running {
		return
	}

	s.running = false
	close(s.stopChan)
}

// scheduleWeeklySummary schedules the weekly summary report
func (s *ReportScheduler) scheduleWeeklySummary() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			return
		case t := <-ticker.C:
			// Check if it's Monday at 9:00 AM
			if t.Weekday() == time.Monday && t.Hour() == 9 && t.Minute() < 5 {
				log.Println("Running weekly GRC summary report")
				err := s.ReportingHandler.SendWeeklySummary()
				if err != nil {
					log.Printf("Error sending weekly summary: %v", err)
				}
			}
		}
	}
}

// scheduleRiskCategorySummary schedules the risk category summary report
func (s *ReportScheduler) scheduleRiskCategorySummary() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			return
		case t := <-ticker.C:
			// Check if it's Wednesday at 9:00 AM
			if t.Weekday() == time.Wednesday && t.Hour() == 9 && t.Minute() < 5 {
				log.Println("Running risk category summary report")
				err := s.ReportingHandler.SendRiskCategorySummary()
				if err != nil {
					log.Printf("Error sending risk category summary: %v", err)
				}
			}
		}
	}
}

// RunManualReport runs a report manually
func (s *ReportScheduler) RunManualReport(reportType string) error {
	switch reportType {
	case "weekly":
		return s.ReportingHandler.SendWeeklySummary()
	case "risk-category":
		return s.ReportingHandler.SendRiskCategorySummary()
	default:
		return nil
	}
}
