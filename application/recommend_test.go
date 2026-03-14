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
	artists []domain.Artist
	err     error
}

func (m *mockMusicSource) TopArtists() ([]domain.Artist, error) {
	return m.artists, m.err
}

type mockConcertFinder struct {
	concerts []domain.Concert
	err      error
}

func (m *mockConcertFinder) FindConcerts(_ []domain.Artist) ([]domain.Concert, error) {
	return m.concerts, m.err
}

type mockNotifier struct {
	notified []domain.Concert
	err      error
}

func (m *mockNotifier) Notify(concerts []domain.Concert) error {
	m.notified = concerts
	return m.err
}

// --- Tests ---

func TestRecommendUseCase_Run_Success(t *testing.T) {
	artists := []domain.Artist{{ID: "1", Name: "Radiohead"}}
	concerts := []domain.Concert{{ID: "c1", Artist: artists[0], Venue: "O2", Date: time.Now()}}

	notifier := &mockNotifier{}
	uc := application.RecommendUseCase{
		Music:    &mockMusicSource{artists: artists},
		Concerts: &mockConcertFinder{concerts: concerts},
		Notifier: notifier,
	}

	if err := uc.Run(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(notifier.notified) != 1 {
		t.Fatalf("expected 1 concert notified, got %d", len(notifier.notified))
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

func TestRecommendUseCase_Run_ConcertFinderError(t *testing.T) {
	uc := application.RecommendUseCase{
		Music:    &mockMusicSource{artists: []domain.Artist{{ID: "1", Name: "Radiohead"}}},
		Concerts: &mockConcertFinder{err: errors.New("concert API down")},
		Notifier: &mockNotifier{},
	}

	if err := uc.Run(); err == nil {
		t.Fatal("expected error from concert finder, got nil")
	}
}
