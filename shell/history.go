package shell

import "github.com/nao1215/sqly/usecase"

const current = -1

// History is user input history manager.
type History struct {
	// index for working cache
	index int
	// workingCache 0:oldest, n:current
	// workingCache also store the string while the user is typing.
	workingCache []string
	// cache store only the string that the user typed Enter (The sqly command or query executed is stored)
	cache []string
	// maxLength is max string length in cache.
	maxLength int
	// interactor control history usecase
	interactor *usecase.HistoryInteractor
}

// NewHistory return *History.
func NewHistory(interactor *usecase.HistoryInteractor) *History {
	return &History{
		index:        0,
		workingCache: []string{""},
		maxLength:    0,
		interactor:   interactor,
	}
}

// alloc allocate cache for storing strings
func (h *History) alloc() {
	h.cache = append(h.cache, h.workingCache[h.index])
	h.workingCache = append([]string{}, h.cache...)
	h.setMaxLength()

	h.workingCache = append(h.workingCache, "")
	h.index = len(h.workingCache) - 1
}

func (h *History) setMaxLength() {
	max := -1
	for _, v := range h.cache {
		l := len(v)
		if max < l {
			max = l
		}
	}
	h.maxLength = max
}

func (h *History) appendChar(r rune) {
	h.workingCache[h.index] = h.currentInput() + string(r)
	if len(h.workingCache[h.index]) > h.maxLength {
		h.maxLength = len(h.workingCache[h.index])
	}
}

func (h *History) replace(s string) {
	h.workingCache[h.index] = s
}

func (h *History) older() {
	if h.index == h.oldestIndex() {
		return
	}
	h.setMaxLength()
	h.index--
}

func (h *History) newer() {
	if h.index == h.newstIndex() {
		return
	}
	h.setMaxLength()
	h.index++
}

func (h *History) currentInput() string {
	return h.workingCache[h.index]
}

func (h *History) newstIndex() int {
	return len(h.workingCache) - 1
}

func (h *History) oldestIndex() int {
	return 0
}
