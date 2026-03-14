package domain

// MusicSource fetches a user's top artists from a streaming platform.
type MusicSource interface {
	TopArtists() ([]Artist, error)
}

// ConcertFinder finds upcoming concerts for a list of artists.
type ConcertFinder interface {
	FindConcerts(artists []Artist) ([]Concert, error)
}

// Notifier delivers a concert digest to the user.
type Notifier interface {
	Notify(concerts []Concert) error
}
