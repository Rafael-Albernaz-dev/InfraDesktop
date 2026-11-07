package ipc

import "context"

// StartBackground ties daemon work to Server.Close, independently of UI connections.
func (s *Server) StartBackground(run func(context.Context)) {
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closing {
		cancel()
		return
	}
	s.backgroundStops = append(s.backgroundStops, cancel)
	s.backgroundWG.Add(1)
	go func() { defer s.backgroundWG.Done(); run(ctx) }()
}
func (s *Server) stopBackground() {
	for _, cancel := range s.backgroundStops {
		cancel()
	}
	s.backgroundWG.Wait()
}
