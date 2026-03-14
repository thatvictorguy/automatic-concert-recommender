package domain

// MusicSource fetches a user's top artists from a streaming platform.
type MusicSource interface {
	// TopArtists returns the user's top 5 artists of all time.
	TopArtists() ([]Artist, error)
	// RecentTopArtists returns the user's top 5 artists from the past month.
	RecentTopArtists() ([]Artist, error)
}

// ConcertFinder finds upcoming concerts for a list of artists.
type ConcertFinder interface {
	FindConcerts(artists []Artist) ([]Concert, error)
}

// Notifier delivers a concert digest to the user.
// section is a human-readable label for the group of concerts (e.g. "All-Time Top Artists").
// artists is the full list of artists in the section; used when there are no concerts to show.
type Notifier interface {
	Notify(section string, artists []Artist, concerts []Concert) error
}
