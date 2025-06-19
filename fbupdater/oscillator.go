package fbupdater

import (
	"time"
)

type OscillatorFBUpdater struct {
	CBHolder
	period time.Duration
	doneCh chan any
}

func OscillatorFBUpdaterFactory(period time.Duration) FBUpdaterFactory {
	return func(updatable FBUpdatable) FBUpdater {
		return NewOscillatorFBUpdater(updatable, period)
	}
}

func NewOscillatorFBUpdater(updatable FBUpdatable, period time.Duration) *OscillatorFBUpdater {
	updater := &OscillatorFBUpdater{
		CBHolder: CBHolder{updatable},
		period:   period,
	}

	return updater
}

func (updater *OscillatorFBUpdater) OnFrameReceived() {}

func (updater *OscillatorFBUpdater) Start() {
	if updater.doneCh != nil {
		panic("OscillatorFBUpdater already running")
	}

	updater.updatable.UpdateFB(true)

	updater.doneCh = make(chan any, 1)
	go func() {
		for {
			select {
			case <-updater.doneCh:
				goto done

			case <-time.After(updater.period):
				updater.updatable.UpdateFB(false)
			}
		}
	done:
	}()
}

func (updater *OscillatorFBUpdater) Stop() {
	if updater.doneCh == nil {
		panic("OscillatorFBUpdater not running")
	}

	updater.doneCh <- struct{}{}
	updater.doneCh = nil
}
