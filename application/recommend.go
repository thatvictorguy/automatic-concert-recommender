package application

import "github.com/thatvictorguy/automatic-concert-recommender/domain"

type RecommendUseCase struct {
	Music    domain.MusicSource
	Concerts domain.ConcertFinder
	Notifier domain.Notifier
}

func (uc *RecommendUseCase) Run() error {
	artists, err := uc.Music.TopArtists()
	if err != nil {
		return err
	}

	concerts, err := uc.Concerts.FindConcerts(artists)
	if err != nil {
		return err
	}

	return uc.Notifier.Notify(concerts)
}
