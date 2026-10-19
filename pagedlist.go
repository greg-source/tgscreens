package tgscreens

// Default navigation captions. BackText has no default: a back button is only
// rendered when Back is set, and its wording is the bot's copy.
const (
	defaultPrevText = "⬅️"
	defaultNextText = "➡️"
)

// PagedList renders a slice of labels as one page of inline buttons, one item
// per row, with previous/next navigation and an optional back button.
type PagedList struct {
	Items    []string
	Page     int
	PageSize int

	// Data returns complete callback data for an item on the rendered zero-based page.
	// i is the index into Items, so callers can index richer records directly.
	Data func(page, i int, item string) string

	// Nav returns the callback data for the previous and next buttons.
	Nav func(page int) string

	Back     string     // callback data for the back button; empty to omit it
	BackText string     // its caption
	Header   [][]Button // rows rendered above the items
	Footer   [][]Button // rows rendered below the pager and above Back

	PrevText, NextText string
}

// Build returns the keyboard for the current page, the page it actually
// rendered, and the total page count.
//
// The page is clamped into range, so a button from a list that has since shrunk
// lands on the last real page rather than an empty one. Callers should label
// the screen with the returned page, not the one they asked for.
func (l PagedList) Build() (buttons [][]Button, page, totalPages int) {
	start, end, page, totalPages := paginate(len(l.Items), l.Page, l.PageSize)

	buttons = make([][]Button, 0, len(l.Header)+(end-start)+len(l.Footer)+2)
	buttons = append(buttons, l.Header...)

	for i, item := range l.Items[start:end] {
		buttons = append(buttons, []Button{{Text: item, Data: l.Data(page, start+i, item)}})
	}

	var prevData, nextData string
	if page > 0 {
		prevData = l.Nav(page - 1)
	}
	if end < len(l.Items) {
		nextData = l.Nav(page + 1)
	}
	if nav := navRow(prevData, nextData, l.prevText(), l.nextText()); len(nav) > 0 {
		buttons = append(buttons, nav)
	}

	buttons = append(buttons, l.Footer...)

	if l.Back != "" {
		buttons = append(buttons, []Button{{Text: l.BackText, Data: l.Back}})
	}

	return buttons, page, totalPages
}

func (l PagedList) prevText() string {
	if l.PrevText == "" {
		return defaultPrevText
	}
	return l.PrevText
}

func (l PagedList) nextText() string {
	if l.NextText == "" {
		return defaultNextText
	}
	return l.NextText
}

// paginate clamps page into range and returns the item window for it, the
// clamped page, and the total page count.
func paginate(total, page, size int) (start, end, clamped, totalPages int) {
	if size <= 0 {
		size = 1
	}

	totalPages = (total + size - 1) / size
	if totalPages < 1 {
		totalPages = 1
	}

	clamped = min(max(page, 0), totalPages-1)

	start = min(clamped*size, total)
	end = min(start+size, total)
	return start, end, clamped, totalPages
}

func navRow(prevData, nextData, prevText, nextText string) []Button {
	var row []Button
	if prevData != "" {
		row = append(row, Button{Text: prevText, Data: prevData})
	}
	if nextData != "" {
		row = append(row, Button{Text: nextText, Data: nextData})
	}
	return row
}
