package application

import (
	"time"

	"github.com/thatvictorguy/automatic-concert-recommender/domain"
)

// concertWindowDays is the lookahead window for upcoming concerts.
const concertWindowDays = 21 // 3 weeks

type RecommendUseCase struct {
	Music    domain.MusicSource
	Concerts domain.ConcertFinder
	Notifier domain.Notifier
}

func (uc *RecommendUseCase) Run() error {
	if err := uc.notifySection("All-Time Top Artists", uc.Music.TopArtists); err != nil {
		return err
	}
	return uc.notifySection("Past Month Top Artists", uc.Music.RecentTopArtists)
}

func (uc *RecommendUseCase) notifySection(section string, fetchArtists func() ([]domain.Artist, error)) error {
	artists, err := fetchArtists()
	if err != nil {
		return err
	}

	concerts, err := uc.Concerts.FindConcerts(artists)
	if err != nil {
		return err
	}

	upcoming := filterWithin(concerts, concertWindowDays)
	return uc.Notifier.Notify(section, artists, upcoming)
}

// filterWithin returns concerts that fall within the next [days] days from now.
func filterWithin(concerts []domain.Concert, days int) []domain.Concert {
	cutoff := time.Now().Truncate(24 * time.Hour).AddDate(0, 0, days)
	var result []domain.Concert
	for _, c := range concerts {
		if !c.Date.After(cutoff) {
			result = append(result, c)
		}
	}
	return result
}
