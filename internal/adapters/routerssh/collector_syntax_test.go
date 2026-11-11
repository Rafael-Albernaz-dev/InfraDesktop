package routerssh

import (
	"infradesktop/internal/adapters/routerssh/deviceparse"
	"infradesktop/internal/modules/equipment"
	"testing"
)

func TestCollectorSyntaxGuard(t *testing.T) {
	for name, script := range map[string]string{
		"operations": deviceparse.DeviceOperationalCommand,
		"auth":       deviceparse.DeviceAuthFailuresCommand,
	} {
		t.Run(name, func(t *testing.T) {
			health, dollar := collectorSyntaxHazards(script)
			if health || dollar {
				t.Fatalf("version-specific health outside parse=%v; unescaped string dollar=%v", health, dollar)
			}
		})
	}
	assertCollectorSyntaxExamples(t)
}

// Keep the import contract anchored while subsequent focal cases are added.
var _ = equipment.Partial
