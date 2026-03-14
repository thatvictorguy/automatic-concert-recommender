package application_test

import (
	"errors"
	"testing"
	"time"

	"github.com/thatvictorguy/automatic-concert-recommender/application"
	"github.com/thatvictorguy/automatic-concert-recommender/domain"
)

// --- Mocks ---

type mockMusicSource struct {
	artists       []domain.Artist
	recentArtists []domain.Artist
	err           error
	recentErr     error
}

func (m *mockMusicSource) TopArtists() ([]domain.Artist, error) {
	return m.artists, m.err
}

func (m *mockMusicSource) RecentTopArtists() ([]domain.Artist, error) {
	return m.recentArtists, m.recentErr
}

type mockConcertFinder struct {
	concerts []domain.Concert
	err      error
}

func (m *mockConcertFinder) FindConcerts(_ []domain.Artist) ([]domain.Concert, error) {
	return m.concerts, m.err
}

type mockNotifier struct {
	calls []notifyCall
	err   error
}

type notifyCall struct {
	section  string
	concerts []domain.Concert
}

func (m *mockNotifier) Notify(section string, concerts []domain.Concert) error {
	m.calls = append(m.calls, notifyCall{section: section, concerts: concerts})
	return m.err
}

// --- Tests ---

func TestRecommendUseCase_Run_CallsBothSections(t *testing.T) {
	allTimeArtists := []domain.Artist{{ID: "1", Name: "Cornelius"}}
	recentArtists := []domain.Artist{{ID: "2", Name: "Ado"}}
	future := time.Now().Add(7 * 24 * time.Hour) // 7 days from now

	concerts := []domain.Concert{{ID: "c1", Artist: allTimeArtists[0], Venue: "O2", Date: future}}

	notifier := &mockNotifier{}
	uc := application.RecommendUseCase{
		Music: &mockMusicSource{
			artists:       allTimeArtists,
			recentArtists: recentArtists,
		},
		Concerts: &mockConcertFinder{concerts: concerts},
		Notifier: notifier,
	}

	if err := uc.Run(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(notifier.calls) != 2 {
		t.Fatalf("expected 2 Notify calls, got %d", len(notifier.calls))
	}
	if notifier.calls[0].section != "All-Time Top Artists" {
		t.Errorf("first section = %q, want %q", notifier.calls[0].section, "All-Time Top Artists")
	}
	if notifier.calls[1].section != "Past Month Top Artists" {
		t.Errorf("second section = %q, want %q", notifier.calls[1].section, "Past Month Top Artists")
	}
}

func TestRecommendUseCase_Run_FiltersToThreeWeeks(t *testing.T) {
	artists := []domain.Artist{{ID: "1", Name: "Band"}}
	withinWindow := time.Now().Add(10 * 24 * time.Hour)   // 10 days — within 3 weeks
	outsideWindow := time.Now().Add(25 * 24 * time.Hour)  // 25 days — outside 3 weeks

	concerts := []domain.Concert{
		{ID: "c1", Artist: artists[0], Venue: "V1", Date: withinWindow},
		{ID: "c2", Artist: artists[0], Venue: "V2", Date: outsideWindow},
	}

	notifier := &mockNotifier{}
	uc := application.RecommendUseCase{
		Music:    &mockMusicSource{artists: artists, recentArtists: artists},
		Concerts: &mockConcertFinder{concerts: concerts},
		Notifier: notifier,
	}

	if err := uc.Run(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Both sections get filtered — each should have 1 concert (withinWindow only)
	for _, call := range notifier.calls {
		if len(call.concerts) != 1 {
			t.Errorf("section %q: expected 1 concert within window, got %d", call.section, len(call.concerts))
		}
		if call.concerts[0].ID != "c1" {
			t.Errorf("section %q: expected concert c1, got %q", call.section, call.concerts[0].ID)
		}
	}
}

func TestRecommendUseCase_Run_MusicSourceError(t *testing.T) {
	uc := application.RecommendUseCase{
		Music:    &mockMusicSource{err: errors.New("spotify down")},
		Concerts: &mockConcertFinder{},
		Notifier: &mockNotifier{},
	}

	if err := uc.Run(); err == nil {
		t.Fatal("expected error from music source, got nil")
	}
}

func TestRecommendUseCase_Run_RecentMusicSourceError(t *testing.T) {
	uc := application.RecommendUseCase{
		Music: &mockMusicSource{
			artists:   []domain.Artist{{ID: "1", Name: "Band"}},
			recentErr: errors.New("spotify down for recent"),
		},
		Concerts: &mockConcertFinder{},
		Notifier: &mockNotifier{},
	}

	if err := uc.Run(); err == nil {
		t.Fatal("expected error from recent music source, got nil")
	}
}

func TestRecommendUseCase_Run_ConcertFinderError(t *testing.T) {
	uc := application.RecommendUseCase{
		Music: &mockMusicSource{
			artists:       []domain.Artist{{ID: "1", Name: "Band"}},
			recentArtists: []domain.Artist{{ID: "1", Name: "Band"}},
		},
		Concerts: &mockConcertFinder{err: errors.New("concert API down")},
		Notifier: &mockNotifier{},
	}

	if err := uc.Run(); err == nil {
		t.Fatal("expected error from concert finder, got nil")
	}
}
