package synthetic

import (
	"context"
	"infradesktop/internal/modules/foundation"
)

type Source struct{}

func (Source) Read(ctx context.Context, label string) (int, foundation.Code) {
	if ctx.Err() != nil {
		return 0, foundation.Deadline
	}
	n := 0
	for _, r := range label {
		n += int(r)
	}
	return n % 1000, foundation.OK
}
