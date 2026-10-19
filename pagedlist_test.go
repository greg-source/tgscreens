package tgscreens

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testList(items []string, page int) PagedList {
	return PagedList{
		Items:    items,
		Page:     page,
		PageSize: 2,
		Data:     func(_ int, i int, item string) string { return "pick:" + strconv.Itoa(i) + "-" + item },
		Nav:      func(p int) string { return "nav:" + strconv.Itoa(p) },
	}
}

func items(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = "item" + strconv.Itoa(i)
	}
	return out
}

// captions returns the button labels row by row, which is what the nav
// assertions actually care about.
func captions(rows [][]Button) []string {
	var out []string
	for _, row := range rows {
		for _, b := range row {
			out = append(out, b.Text)
		}
	}
	return out
}

func TestPagedList_navigationAppearsOnlyWhereThereIsSomewhereToGo(t *testing.T) {
	tests := []struct {
		name         string
		total        int
		page         int
		wantCaptions []string
		wantPages    int
	}{
		{name: "first page", total: 5, page: 0, wantCaptions: []string{"item0", "item1", "➡️"}, wantPages: 3},
		{name: "middle page", total: 5, page: 1, wantCaptions: []string{"item2", "item3", "⬅️", "➡️"}, wantPages: 3},
		{name: "last page", total: 5, page: 2, wantCaptions: []string{"item4", "⬅️"}, wantPages: 3},
		{name: "single page", total: 2, page: 0, wantCaptions: []string{"item0", "item1"}, wantPages: 1},
		{name: "exact multiple has no trailing next", total: 4, page: 1, wantCaptions: []string{"item2", "item3", "⬅️"}, wantPages: 2},
		{name: "empty", total: 0, page: 0, wantCaptions: nil, wantPages: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buttons, page, totalPages := testList(items(tt.total), tt.page).Build()

			assert.Equal(t, tt.wantCaptions, captions(buttons))
			assert.Equal(t, tt.wantPages, totalPages)
			assert.Equal(t, tt.page, page)
		})
	}
}

// A button drawn against a longer list stays pressable after the list shrinks.
// Clamping lands it on the last real page instead of an empty one captioned
// with a page number that does not exist.
func TestPagedList_clampsAPageBeyondTheEnd(t *testing.T) {
	buttons, page, totalPages := testList(items(5), 9).Build()

	assert.Equal(t, 2, page)
	assert.Equal(t, 3, totalPages)
	assert.Equal(t, []string{"item4", "⬅️"}, captions(buttons))
}

func TestPagedList_dataReceivesTheRenderedPage(t *testing.T) {
	list := testList(items(5), 9)
	var pages []int
	list.Data = func(page, _ int, _ string) string {
		pages = append(pages, page)
		return ""
	}

	list.Build()

	assert.Equal(t, []int{2}, pages)
}

func TestPagedList_clampsANegativePage(t *testing.T) {
	buttons, page, _ := testList(items(5), -3).Build()

	assert.Equal(t, 0, page)
	assert.Equal(t, []string{"item0", "item1", "➡️"}, captions(buttons))
}

// Nothing is appended to what Data returns: callers that supply their own
// callback data must not have to strip anything off.
func TestPagedList_callbackDataIsVerbatim(t *testing.T) {
	buttons, _, _ := testList(items(2), 0).Build()

	assert.Equal(t, "pick:0-item0", buttons[0][0].Data)
	assert.Equal(t, "pick:1-item1", buttons[1][0].Data)
}

// Data receives the index into Items, so two rows that read the same still
// resolve to different records.
func TestPagedList_identicalLabelsDoNotCollide(t *testing.T) {
	buttons, _, _ := testList([]string{"same", "same"}, 0).Build()

	require.Len(t, buttons, 2)
	assert.NotEqual(t, buttons[0][0].Data, buttons[1][0].Data)
}

// Indices stay absolute on later pages, or a caller indexing back into its own
// slice would read the wrong record.
func TestPagedList_indexIsAbsoluteAcrossPages(t *testing.T) {
	buttons, _, _ := testList(items(5), 1).Build()

	assert.Equal(t, "pick:2-item2", buttons[0][0].Data)
	assert.Equal(t, "pick:3-item3", buttons[1][0].Data)
}

func TestPagedList_headerFooterAndBack(t *testing.T) {
	l := testList(items(3), 0)
	l.Header = [][]Button{{{Text: "⭐ all", Data: "all"}}}
	l.Footer = [][]Button{{{Text: "✍️ custom", Data: "custom"}}}
	l.Back = "back"
	l.BackText = "🔙 back"

	buttons, _, _ := l.Build()

	assert.Equal(t, []string{"⭐ all", "item0", "item1", "➡️", "✍️ custom", "🔙 back"}, captions(buttons))
}

func TestPagedList_emptyFooterChangesNothing(t *testing.T) {
	withoutFooter, _, _ := testList(items(3), 0).Build()

	l := testList(items(3), 0)
	l.Footer = [][]Button{}
	withEmptyFooter, _, _ := l.Build()

	assert.Equal(t, withoutFooter, withEmptyFooter)
}

func TestPagedList_backIsOmittedWithoutCallbackData(t *testing.T) {
	l := testList(items(2), 0)
	l.BackText = "🔙 back"

	buttons, _, _ := l.Build()

	assert.Equal(t, []string{"item0", "item1"}, captions(buttons))
}

func TestPagedList_zeroPageSizeDoesNotDivideByZero(t *testing.T) {
	l := testList(items(3), 0)
	l.PageSize = 0

	buttons, _, totalPages := l.Build()

	assert.Equal(t, 3, totalPages)
	assert.Equal(t, []string{"item0", "➡️"}, captions(buttons))
}
