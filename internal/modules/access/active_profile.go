package access

import "context"

// ActiveProfile is an internal daemon port; it grants no UI or inventory access.
func (s *Service) ActiveProfile(ctx context.Context) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, code := s.store.Read(ctx)
	if code != OK || st.Account == nil || st.Account.Profile == "" {
		return "", false
	}
	for _, company := range st.Companies {
		if company.ID == st.Account.Profile {
			return company.ID, true
		}
	}
	return "", false
}
