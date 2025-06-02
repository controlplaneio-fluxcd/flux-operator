// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package schedule

import (
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
)

const (
	// cronLookBack is the maximum time range a previous cron trigger
	// could have occurred in the past. It's 5 years because a cron
	// schedule can be defined for Feb 29, which only occurs every 4
	// years (0 0 29 2 *).
	cronLookBack = 5 * 365 * 24 * time.Hour
)

// GetPrevAndNextTriggers returns the previous and next triggers for the given schedule spec
// and the current time.
func GetPrevAndNextTriggers(spec, timeZone string, now time.Time) (time.Time, time.Time, error) {
	s, err := Parse(spec, timeZone)
	if err != nil {
		if timeZone != "" {
			return time.Time{}, time.Time{}, fmt.Errorf("failed to parse cron spec '%s' with timezone '%s': %w",
				spec, timeZone, err)
		}
		return time.Time{}, time.Time{}, fmt.Errorf("failed to parse cron spec '%s': %w", spec, err)
	}
	next := s.Next(now)
	start := now.Add(-cronLookBack)
	end := next.Add(-1)
	prev := binarySearchPreviousTrigger(s, start, end, next)
	return prev, next, nil
}

// Parse parses a cron schedule specification and returns a cron.Schedule.
func Parse(spec, timeZone string) (cron.Schedule, error) {
	if timeZone != "" {
		spec = fmt.Sprintf("CRON_TZ=%s %s", timeZone, spec)
	}
	return cron.
		NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor).
		Parse(spec)
}

// binarySearchPreviousTrigger finds the previous trigger for the schedule
// by performing a binary search. If no previous trigger is found, zero is
// returned.
func binarySearchPreviousTrigger(s cron.Schedule, start, end, next time.Time) time.Time {
	// Base case: The end time is before the start time, which means
	// that there are no points in the range to compute a trigger for.
	if end.Before(start) {
		return time.Time{}
	}

	// Compute middle of the range and the associated trigger.
	middle := start.Add(end.Sub(start) / 2)
	middleTrigger := s.Next(middle)

	if !middleTrigger.Before(next) {
		// If the trigger for the middle of the range is after or equals the next trigger,
		// we need to search in the left half of the range. It's very important to pass
		// the end as middle.Add(-1) as we know that the trigger at middle is not valid and
		// we need to effectively decrease the range to make the next subproblem smaller,
		// otherwise the base case above will never be reached.
		return binarySearchPreviousTrigger(s, start, middle.Add(-1), next)
	}

	// middleTrigger definitely comes before the next trigger, so it's definitely a candidate.
	// But we still need to check if there is a closer trigger in the right half of the range.
	// To effectively decrease the range to make the next subproblem smaller, we use the earliest
	// known time point after middle that could still yield a valid trigger. This time point is
	// middleTrigger, because s.Next(t) returns a time point that comes strictly after t, so
	// middleTrigger is guaranteed to come after middle, and s.Next(middleTrigger) is guaranteed
	// to come after middleTrigger. We could choose middle.Add(1) as well, but that could still
	// yield middleTrigger again, so we choose middleTrigger to ensure that we always decrease
	// the search space.
	closerTrigger := binarySearchPreviousTrigger(s, middleTrigger, end, next)
	if !closerTrigger.IsZero() {
		return closerTrigger
	}

	return middleTrigger
}
