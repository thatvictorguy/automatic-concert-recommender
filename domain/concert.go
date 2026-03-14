package domain

import "time"

type Concert struct {
	ID        string
	Artist    Artist
	Venue     string
	City      string
	Date      time.Time
	TicketURL string
}
