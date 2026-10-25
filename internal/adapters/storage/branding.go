package storage

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"infradesktop/internal/modules/access"
	"strings"
)

// Image bytes are checked at the trusted adapter, independently of file-picker MIME.
func validLogo(logo string) bool {
	if logo == "" {
		return true
	}
	head, data, ok := strings.Cut(logo, ",")
	if !ok {
		return false
	}
	raw, e := base64.StdEncoding.Strict().DecodeString(data)
	if e != nil || len(raw) > access.MaxLogoBytes {
		return false
	}
	if head == "data:image/webp;base64" {
		return validWebP(raw)
	}
	cfg, format, e := image.DecodeConfig(bytes.NewReader(raw))
	if e != nil || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 4096 || cfg.Height > 4096 || cfg.Width*cfg.Height > 16777216 {
		return false
	}
	if (format == "png" && head != "data:image/png;base64") || (format == "jpeg" && head != "data:image/jpeg;base64") {
		return false
	}
	_, format, e = image.Decode(bytes.NewReader(raw))
	return e == nil && (format == "png" || format == "jpeg")
}

func (s *Store) ReadBrand(ctx context.Context, id string) (access.Branding, access.Code) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var b access.Branding
	if s.unit == nil || s.unitID != id {
		return b, access.Changed
	}
	if s.checkActive() != nil {
		return b, access.Unavailable
	}
	e := s.unit.QueryRowContext(ctx, "SELECT name,logo,accent FROM branding WHERE singleton=1").Scan(&b.Name, &b.Logo, &b.Accent)
	if e != nil || access.ValidBrand(b) != access.OK || !validLogo(b.Logo) {
		return access.Branding{}, access.Unavailable
	}
	return b, access.OK
}
func (s *Store) SaveBrand(ctx context.Context, id string, b access.Branding) access.Code {
	if c := access.ValidBrand(b); c != access.OK {
		return c
	}
	if !validLogo(b.Logo) {
		return "BRAND_LOGO_INVALID"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.unit == nil || s.unitID != id {
		return access.Changed
	}
	if s.checkActive() != nil {
		return access.Unavailable
	}
	tx, e := s.unit.BeginTx(ctx, nil)
	if e != nil {
		return access.Unavailable
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, "UPDATE branding SET name=?,logo=?,accent=? WHERE singleton=1", b.Name, b.Logo, b.Accent); e != nil {
		return access.Unavailable
	}
	s.observe("before-brand-commit")
	if tx.Commit() != nil {
		return access.Unavailable
	}
	return access.OK
}
