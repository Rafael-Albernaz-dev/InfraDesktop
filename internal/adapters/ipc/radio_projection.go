package ipc

import (
	"encoding/json"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/equipment"
)

func encodeRadioProjection(v equipment.RadioView) ([]byte, equipment.Code) {
	for _, reading := range []*equipment.RadioReading{v.Reading, v.CPEReading} {
		if reading == nil {
			continue
		}
		for _, client := range reading.Clients {
			if client.CPE != "" && !access.ValidCompanyID(client.CPE) {
				return nil, equipment.Invalid
			}
		}
	}
	body, err := json.Marshal(v)
	if err != nil || len(body) > 48*1024 {
		return nil, equipment.Limit
	}
	return body, equipment.OK
}
