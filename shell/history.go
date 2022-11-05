package shell

import (
	"context"
	"fmt"

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

// record record user input in DB
func (h *History) record(ctx context.Context) error {
	history := model.History{
		ID:      len(h.workingCache) + 1,
		Request: h.currentInput(),
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

func (h *History) appendChar(r rune) {
	h.workingCache[h.index].Request = h.currentInput() + string(r)
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

func (h *History) newstIndex() uint {
	return uint(len(h.workingCache) - 1)
}

func (h *History) oldestIndex() uint {
	return 0
}
