package access

import (
	"context"
	"strings"
	"unicode"
	"unicode/utf8"
)

const MaxLogoBytes = 2 * 1024 * 1024

// Branding is company presentation data; identity and account theme remain separate.
type Branding struct{ Name, Logo, Accent string }
type BrandStore interface {
	ReadBrand(context.Context, string) (Branding, Code)
	SaveBrand(context.Context, string, Branding) Code
}

func ValidBrand(b Branding) Code {
	if !utf8.ValidString(b.Name) || strings.TrimSpace(b.Name) != b.Name || utf8.RuneCountInString(b.Name) > 48 {
		return "BRAND_NAME_INVALID"
	}
	for _, r := range b.Name {
		if unicode.IsControl(r) {
			return "BRAND_NAME_INVALID"
		}
	}
	switch b.Accent {
	case "teal", "violet", "copper", "blue", "green", "rose", "indigo", "slate":
	default:
		return "BRAND_ACCENT_INVALID"
	}
	if b.Logo != "" {
		if len(b.Logo) > ((MaxLogoBytes+2)/3)*4+32 {
			return "BRAND_LOGO_INVALID"
		}
		head, _, ok := strings.Cut(b.Logo, ",")
		if !ok || (head != "data:image/png;base64" && head != "data:image/jpeg;base64" && head != "data:image/webp;base64") {
			return "BRAND_LOGO_INVALID"
		}
	}
	return OK
}

// Handle holds the service lock through authorization and the short company transaction.
func (s *Service) brand(ctx context.Context, st State, q Command) (View, Code) {
	if q.ExpectedProfile == "" || q.ExpectedProfile != st.Account.Profile || q.ExpectedActivation != s.activation {
		return View{}, Changed
	}
	if s.brands == nil {
		return View{}, Unavailable
	}
	if q.Op == "company.brand-set" {
		if c := ValidBrand(q.Brand); c != OK {
			return View{}, c
		}
		if c := s.brands.SaveBrand(ctx, q.ExpectedProfile, q.Brand); c != OK {
			return View{}, c
		}
	}
	b, c := s.brands.ReadBrand(ctx, q.ExpectedProfile)
	if c != OK {
		return View{}, c
	}
	v := s.view(st)
	v.Authenticated = true
	v.Brand = &b
	return v, OK
}
