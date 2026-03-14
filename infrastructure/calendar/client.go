package calendar

import "github.com/thatvictorguy/automatic-concert-recommender/domain"

// Client implements adding concerts to Google Calendar.
type Client struct {
	CredentialsFile string
}

func (c *Client) AddConcert(concert domain.Concert) error {
	// TODO: implement Google Calendar API call
	panic("not implemented")
}
