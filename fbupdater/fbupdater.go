package fbupdater

import (
	"sync/atomic"
)

type FBUpdater interface {
	OnFrameReceived()
	Start()
	Stop()
}
type FBUpdatable interface {
	UpdateFB(incremental bool)
}

type FBUpdaterFactory = func(FBUpdatable) FBUpdater

type CBHolder struct {
	updatable FBUpdatable
}

type OnFrameReceivedFBUpdater struct {
	CBHolder
	isRunning atomic.Bool
}

func NewOnFrameReceivedFBUpdater(updatable FBUpdatable) FBUpdater {
	return &OnFrameReceivedFBUpdater{CBHolder: CBHolder{updatable}}
}

func (updater *OnFrameReceivedFBUpdater) Start() {
	if !updater.isRunning.CompareAndSwap(false, true) {
		panic("OnFrameReceivedFBUpdater already running")
	}

	updater.updatable.UpdateFB(false)
}

func (updater *OnFrameReceivedFBUpdater) Stop() {
	if !updater.isRunning.CompareAndSwap(true, false) {
		panic("OnFrameReceivedFBUpdater not running")
	}
}

func (updater *OnFrameReceivedFBUpdater) OnFrameReceived() {
	if updater.isRunning.Load() {
		updater.updatable.UpdateFB(true)
	}
}
