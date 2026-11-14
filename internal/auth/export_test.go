package auth

import "context"

// HoldHashSlotsForTest occupies all password hashing slots until released.
func HoldHashSlotsForTest(s *Service) func() {
	for range cap(s.hasher.sem) {
		if err := s.hasher.acquire(context.Background()); err != nil {
			panic(err)
		}
	}
	return func() {
		for range cap(s.hasher.sem) {
			s.hasher.release()
		}
	}
}
