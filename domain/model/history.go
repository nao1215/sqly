package model

// Histories is sqly history all record.
type Histories []History

// History is one entry of the sqly shell's command history.
//
// It carries no id. The history is a file whose order is its order, and nothing
// addresses an entry by anything else; an id was what the SQLite table it used
// to live in needed for its own sake.
type History struct {
	// Request is sqly history record that is user input from sqly prompt
	Request string
}

// NewHistory create new History.
func NewHistory(request string) History {
	return History{Request: request}
}

// ToStringList convert history to string list.
func (h Histories) ToStringList() []string {
	histories := make([]string, 0, len(h))
	for _, v := range h {
		histories = append(histories, v.Request)
	}
	return histories
}
