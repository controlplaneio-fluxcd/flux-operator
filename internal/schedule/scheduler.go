// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package schedule

import (
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

const (
	// cronLookBack is the maximum time range a previous cron trigger
	// could have occurred in the past. It's 5 years because a cron
	// schedule can be defined for Feb 29, which only occurs every 4
	// years (0 0 29 2 *).
	cronLookBack = 5 * 365 * 24 * time.Hour
)

// Scheduler computes when reconciliations should happen according
// to a list of schedule configurations.
type Scheduler struct {
	schedules []parsedSchedule
	timeout   time.Duration
}

type parsedSchedule struct {
	cron.Schedule
	spec fluxcdv1.Schedule
}

// NewScheduler creates a new Schedule instance from the provided schedules.
func NewScheduler(schedules []fluxcdv1.Schedule, timeout time.Duration) (*Scheduler, error) {
	if len(schedules) == 0 {
		return nil, nil
	}

	s := &Scheduler{
		schedules: make([]parsedSchedule, len(schedules)),
		timeout:   timeout,
	}

	for i := range schedules {
		// Validate window and timeout.
		window := schedules[i].Window.Duration
		switch {
		case 0 < window && window < 2*timeout:
			return nil, fmt.Errorf("failed to validate schedule[%d]: a non-zero window (%s) must always be at least twice the timeout (%s)",
				i, window, timeout)
		case window < 0:
			return nil, fmt.Errorf("failed to validate schedule[%d]: negative window: %s", i, window)
		}

		// Parse cron spec.
		cron, err := Parse(schedules[i].Cron, schedules[i].TimeZone)
		if err != nil {
			return nil, fmt.Errorf("failed to validate schedule[%d]: %w", i, err)
		}
		s.schedules[i] = parsedSchedule{
			Schedule: cron,
			spec:     schedules[i],
		}
	}

	return s, nil
}

// ShouldReconcile checks if any of the provided schedules
// allows a reconciliation at the given time.
func (s *Scheduler) ShouldReconcile(now time.Time) bool {
	if s == nil {
		return true
	}

	deadline := now.Add(s.timeout)

	for _, parsedSchedule := range s.schedules {
		// When there is at least one schedule without a window,
		// to make the best effort we must always allow the
		// reconciliation.
		window := parsedSchedule.spec.Window.Duration
		if window == 0 {
			return true
		}

		// Check if the reconciliation is within the window.
		prev, _ := GetPrevAndNextTriggers(parsedSchedule, now)
		if deadline.Before(prev.Add(window)) {
			return true
		}
	}

	return false
}

// ShouldScheduleInterval checks if any of the provided schedules
// allows a reconciliation to be scheduled at the given time.
func (s *Scheduler) ShouldScheduleInterval(nextInterval time.Time) bool {
	if s == nil {
		return true
	}

	deadline := nextInterval.Add(s.timeout)

	for _, parsedSchedule := range s.schedules {
		// Schedules without a window do not contribute to
		// deciding if a reconciliation can be scheduled.
		window := parsedSchedule.spec.Window.Duration
		if window == 0 {
			continue
		}

		// Check if the reconciliation is within the window.
		prev, _ := GetPrevAndNextTriggers(parsedSchedule, nextInterval)
		if deadline.Before(prev.Add(window)) {
			return true
		}
	}

	return false
}

// Next returns the next reconciliation schedule and time for the given time.
func (s *Scheduler) Next(now time.Time) *fluxcdv1.NextSchedule {
	if s == nil {
		return nil
	}

	var nextSchedule *fluxcdv1.NextSchedule

	for _, parsedSchedule := range s.schedules {
		next := parsedSchedule.Next(now)
		if nextSchedule == nil || next.Before(nextSchedule.When.Time) {
			nextSchedule = &fluxcdv1.NextSchedule{
				Schedule: parsedSchedule.spec,
				When:     metav1.Time{Time: next},
			}
		}
	}

	return nextSchedule
}

// Parse parses a cron schedule specification and returns a cron.Schedule.
func Parse(spec, timeZone string) (cron.Schedule, error) {
	cronSpec := spec
	if timeZone != "" {
		cronSpec = fmt.Sprintf("CRON_TZ=%s %s", timeZone, spec)
	}
	s, err := cron.
		NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor).
		Parse(cronSpec)
	if err == nil {
		return s, nil
	}
	if timeZone != "" {
		return nil, fmt.Errorf("failed to parse cron spec '%s' with timezone '%s': %w", spec, timeZone, err)
	}
	return nil, fmt.Errorf("failed to parse cron spec '%s': %w", spec, err)
}

// GetPrevAndNextTriggers returns the previous and next triggers for the given schedule spec
// and the current time.
func GetPrevAndNextTriggers(schedule cron.Schedule, now time.Time) (time.Time, time.Time) {
	next := schedule.Next(now)
	start := now.Add(-cronLookBack)
	end := next.Add(-1)
	prev := binarySearchPreviousTrigger(schedule, start, end, next)
	return prev, next
}

// binarySearchPreviousTrigger finds the previous trigger for the schedule
// by performing a binary search. If no previous trigger is found, zero is
// returned.
func binarySearchPreviousTrigger(schedule cron.Schedule, start, end, next time.Time) time.Time {
	// Base case: The end time is before the start time, which means
	// that there are no points in the range to compute a trigger for.
	if end.Before(start) {
		return time.Time{}
	}

	// Compute middle of the range and the associated trigger.
	middle := start.Add(end.Sub(start) / 2)
	middleTrigger := schedule.Next(middle)

	if !middleTrigger.Before(next) { // if next <= middleTrigger
		// If the trigger for the middle of the range is after or equals the next trigger,
		// we need to search in the left half of the range. It's very important to pass
		// the end as middle.Add(-1) as we know that the trigger at middle is not valid and
		// we need to effectively decrease the range to make the next subproblem smaller,
		// otherwise the base case above will never be reached.
		return binarySearchPreviousTrigger(schedule, start, middle.Add(-1), next)
	}

	// middleTrigger definitely comes before the next trigger, so it's definitely a candidate.
	// But we still need to check if there is a closer trigger in the right half of the range.
	// To effectively decrease the range to make the next subproblem smaller, we use the earliest
	// known time point after middle that could still yield a valid trigger. This time point is
	// middleTrigger, because schedule.Next(t) returns a time point that comes strictly after t,
	// so middleTrigger is guaranteed to come after middle, and schedule.Next(middleTrigger) is
	// guaranteed to come after middleTrigger. We could choose middle.Add(1) as well, but that
	// could still yield middleTrigger again, so we choose middleTrigger for more efficiency.
	closerTrigger := binarySearchPreviousTrigger(schedule, middleTrigger, end, next)
	if !closerTrigger.IsZero() {
		return closerTrigger
	}

	return middleTrigger
}
