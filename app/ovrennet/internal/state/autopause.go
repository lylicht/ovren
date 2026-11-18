package state

import "time"

const (
	autoPauseIdleDelay   = 10 * time.Second
	autoPauseBusyDelay   = 2 * time.Minute
	autoPauseResumeDelay = 3 * time.Second
	autoPauseSwitchGap   = 30 * time.Second
)

type AutoPause struct {
	seenTwoLinks  bool
	paused        bool
	lastSwitch    time.Time
	stableTwoLink time.Time
	idleSince     time.Time
	busySince     time.Time
}

func (a *AutoPause) ManualResume(now time.Time, usableLinks int) {
	a.paused = false
	a.seenTwoLinks = usableLinks >= 2
	a.lastSwitch = now
	a.stableTwoLink = time.Time{}
	a.idleSince = time.Time{}
	a.busySince = time.Time{}
}

func (a *AutoPause) Paused() bool {
	return a.paused
}

func (a *AutoPause) ResumeFailed(now time.Time) {
	a.paused = true
	a.lastSwitch = now
}

func (a *AutoPause) Step(now time.Time, enabled, manualPaused, busy bool, usableLinks int) (pause, resume bool, reason string) {
	if usableLinks >= 2 {
		a.seenTwoLinks = true
	}
	if manualPaused {
		return false, false, "manual"
	}
	if !enabled {
		if a.paused && now.Sub(a.lastSwitch) >= autoPauseSwitchGap {
			a.paused = false
			a.lastSwitch = now
			return false, true, "none"
		}
		if a.paused {
			return false, false, "automatic"
		}
		return false, false, "none"
	}
	if usableLinks >= 2 {
		if a.stableTwoLink.IsZero() {
			a.stableTwoLink = now
		}
		a.seenTwoLinks = true
		a.idleSince = time.Time{}
		a.busySince = time.Time{}
		if a.paused && now.Sub(a.stableTwoLink) >= autoPauseResumeDelay && now.Sub(a.lastSwitch) >= autoPauseSwitchGap {
			a.paused = false
			a.lastSwitch = now
			return false, true, "none"
		}
		if a.paused {
			return false, false, "automatic"
		}
		return false, false, "none"
	}
	a.stableTwoLink = time.Time{}
	if !a.seenTwoLinks {
		return false, false, "none"
	}
	if a.paused {
		return false, false, "automatic"
	}
	if busy {
		a.idleSince = time.Time{}
		if a.busySince.IsZero() {
			a.busySince = now
		}
		if now.Sub(a.busySince) < autoPauseBusyDelay {
			return false, false, "none"
		}
	} else {
		a.busySince = time.Time{}
		if a.idleSince.IsZero() {
			a.idleSince = now
		}
		if now.Sub(a.idleSince) < autoPauseIdleDelay {
			return false, false, "none"
		}
	}
	if !a.lastSwitch.IsZero() && now.Sub(a.lastSwitch) < autoPauseSwitchGap {
		return false, false, "none"
	}
	a.paused = true
	a.lastSwitch = now
	return true, false, "automatic"
}
