package equipment

import "context"

// RadioReadSession owns a bounded, authorized read window. Transport details
// stay in the adapter. The rich read is separate from the ten measured ticks.
type RadioReadSession interface {
	Initial() (*RadioReading, Code)
	Tick() (*RadioReading, Code)
	Probe() (RadioLoad, Code)
	Close()
}

type RadioSessionRemote interface {
	OpenRadioSession(context.Context, Device, string, string, RadioFamily, *RadioLink, func() Code) (RadioReadSession, Code)
}
