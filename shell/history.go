package shell

import (
	"context"
	"fmt"
	"strings"

	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/usecase"
)

// History is user input history manager.
type History struct {
	// index for working cache
	index uint
	// workingCache 1:oldest, n:current
	// workingCache also store the string while the user is typing.
	workingCache model.Histories
	// maxLength is max string length in cache.
	maxLength int
	// interactor control history usecase
	interactor *usecase.HistoryInteractor
}

// NewHistory return *History.
func NewHistory(interactor *usecase.HistoryInteractor) *History {
	return &History{
		index:      0,
		maxLength:  0,
		interactor: interactor,
	}
}

// initialize for
func (h *History) initialize(ctx context.Context) error {
	if err := h.interactor.CreateTable(ctx); err != nil {
		return fmt.Errorf("failed to create table for sqly history: %v", err)
	}
	return h.refreshWorkingCache(ctx)
}

func (h *History) refreshWorkingCache(ctx context.Context) error {
	histories, err := h.interactor.List(ctx)
	if err != nil {
		return fmt.Errorf("failed to refresh history cache: %v", err)
	}
	h.workingCache = histories
	h.workingCache = append(h.workingCache, &model.History{
		ID:      len(h.workingCache),
		Request: "",
	})
	h.index = uint(len(h.workingCache) - 1)
	h.setMaxInputLength()
	return nil
}

// recordAndRefreshCache record user input in DB.
// The cache is refreshed, so the string data that was temporarily entered is deleted.
func (h *History) recordAndRefreshCache(ctx context.Context) error {
	history := model.History{
		ID:      len(h.workingCache) + 1,
		Request: strings.ReplaceAll(h.currentInput(), "\b", ""),
	}

	if err := h.interactor.Create(ctx, history); err != nil {
		return fmt.Errorf("failed to store user input history: %v", err)
	}
	return h.refreshWorkingCache(ctx)
}

func (h *History) setMaxInputLength() {
	max := -1
	for _, v := range h.workingCache {
		l := len(v.Request)
		if max < l {
			max = l
		}
	}
	h.maxLength = max
}

func (h *History) appendChar(r rune, position int) {
	current := h.currentInput()

	if position == h.currentInputLen() {
		h.workingCache[h.index].Request = current + string(r)
	} else if position == 0 {
		if r != runeBackSpace {
			h.workingCache[h.index].Request = string(r) + current
		}
	} else {
		if r == runeBackSpace {
			if position == 1 {
				// Bug is here
				h.workingCache[h.index].Request = current[:position] + string(r) + current[position:]
				return
			}
			h.workingCache[h.index].Request = current[:position+1] + string(r) + current[position+1:]
		} else {
			h.workingCache[h.index].Request = current[:position] + string(r) + current[position:]
		}
	}

	if len(h.workingCache[h.index].Request) > h.maxLength {
		h.maxLength = len(h.workingCache[h.index].Request)
	}
}

func (h *History) replace(s string) {
	h.workingCache[h.index].Request = s
}

func (h *History) older() {
	if h.index == h.oldestIndex() {
		return
	}
	h.setMaxInputLength()
	h.index--
}

func (h *History) newer() {
	if h.index == h.newstIndex() {
		return
	}
	h.setMaxInputLength()
	h.index++
}

func (h *History) currentInput() string {
	return h.workingCache[h.index].Request
}

func (h *History) currentInputLen() int {
	return len(h.workingCache[h.index].Request)
}

func (h *History) newstIndex() uint {
	return uint(len(h.workingCache) - 1)
}

func (h *History) oldestIndex() uint {
	return 0
}
