package equipment

import "context"

// RadioDeviceLinkStore projects confirmed local associations for an inventory device.
type RadioDeviceLinkStore interface {
	RadioDeviceLinks(context.Context, string, string) ([]RadioLink, Code)
}
