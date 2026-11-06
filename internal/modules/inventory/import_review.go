package inventory

import (
	"net/netip"
	"strconv"
	"strings"

	"infradesktop/internal/modules/equipment"
)

const (
	importReady     = "ready"
	importPending   = "pending"
	importDuplicate = "duplicate"
	importConflict  = "conflict"
	importIgnored   = "ignored"
	importRowsLimit = 500
)

func importRole(value string) equipment.Role {
	names := map[string]equipment.Role{
		"concentrador": equipment.RoleConcentrator, "roteador de borda": equipment.RoleEdge,
		"switch": equipment.RoleSwitch, "ap": equipment.RoleAP, "st": equipment.RoleStation,
		"ptp": equipment.RolePTP, "cpe": equipment.RoleCPE, "monitor de energia": equipment.RolePowerMonitor,
		"nobreak": equipment.RoleUPS, "retificadora ou fonte": equipment.RoleRectifier,
		"controlador solar": equipment.RoleSolarController, "monitor de bateria": equipment.RoleBatteryMonitor,
		"outro": equipment.RoleOther,
	}
	v := strings.ToLower(strings.TrimSpace(value))
	if role, ok := names[v]; ok {
		return role
	}
	return equipment.Role(v)
}

func credentialForImport(st *State, name string) (string, string) {
	if name == "" {
		return "", ""
	}
	id, matches := "", 0
	for _, cr := range st.Credentials {
		if strings.EqualFold(cr.Metadata.Alias, name) {
			id, matches = cr.Metadata.ID, matches+1
		}
	}
	if matches == 0 {
		return "", "Credencial não encontrada"
	}
	if matches > 1 {
		return "", "Nome de credencial repetido"
	}
	return id, ""
}

func importDevice(row equipment.ImportRow, id, credential string) (Device, string) {
	role := importRole(row.Role)
	if !equipment.ValidInventoryRole(role) {
		return Device{}, "Papel desconhecido"
	}
	portText := strings.TrimSpace(row.Port)
	if portText == "" {
		portText = "22"
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return Device{}, "Porta inválida"
	}
	address := strings.TrimSpace(row.Address)
	if ip, err := netip.ParseAddr(address); err == nil {
		address = ip.String()
	}
	d := Device{ID: id, Name: strings.TrimSpace(row.Name), Address: address, Port: port, Transport: TransportSSH,
		Role: role, Block: row.Block, CredentialID: credential, Revision: 1, Active: true}
	if code := ValidateDevice(d); code != OK {
		return Device{}, string(code)
	}
	return d, ""
}

func reviewImportRow(st *State, row equipment.ImportRow, existing, seen map[string]bool) equipment.ImportItem {
	item := equipment.ImportItem{Row: row}
	if row.Line < 1 {
		item.Status, item.Reason = importPending, "Número de linha inválido"
		return item
	}
	if row.Ignored {
		item.Status = importIgnored
		return item
	}
	credential, reason := credentialForImport(st, strings.TrimSpace(row.Credential))
	if reason != "" {
		item.Status, item.Reason = importPending, reason
		return item
	}
	if credential == "" {
		credential = st.CredentialDefaults[string(importRole(row.Role))]
		if credential != "" && CredentialByID(st, credential) == nil {
			item.Status, item.Reason = importPending, "Credencial padrão não encontrada"
			return item
		}
	}
	d, reason := importDevice(row, "00000000-0000-4000-8000-000000000000", credential)
	if reason != "" {
		item.Status, item.Reason = importPending, reason
		return item
	}
	item.CredentialID = credential
	if existing[d.Address] {
		item.Status, item.Reason = importDuplicate, "IP já cadastrado"
		return item
	}
	if seen[d.Address] {
		item.Status, item.Reason = importConflict, "IP repetido no arquivo"
		return item
	}
	item.Status = importReady
	return item
}

func reviewImport(st *State, rows []equipment.ImportRow) []equipment.ImportItem {
	existing, counts := map[string]bool{}, map[string]int{}
	for _, d := range st.Devices {
		existing[d.Address] = true
	}
	for _, row := range rows {
		if !row.Ignored {
			if ip, err := netip.ParseAddr(strings.TrimSpace(row.Address)); err == nil {
				counts[ip.String()]++
			}
		}
	}
	duplicates := map[string]bool{}
	for address, count := range counts {
		duplicates[address] = count > 1
	}
	out := make([]equipment.ImportItem, 0, len(rows))
	for _, row := range rows {
		out = append(out, reviewImportRow(st, row, existing, duplicates))
	}
	return out
}
