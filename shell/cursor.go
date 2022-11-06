package shell

// cursor is current user input positon in terminal
type cursor struct {
	// pos is cursor positon; 0 is head (left)
	pos int
}

func newCursor() *cursor {
	return &cursor{
		pos: 0,
	}
}

func (c *cursor) position() int {
	return c.pos
}

func (c *cursor) moveHead() {
	c.pos = 0
}

func (c *cursor) moveLeft() {
	c.pos--
	if c.position() == 0 {
		c.moveHead()
		return
	}
}

func (c *cursor) moveRight() {
	c.pos++
}

func (c *cursor) set(p int) {
	if p < 0 {
		p = 0
	}
	c.pos = p
}
