package instrument_go

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/xhd2015/xgo/support/edit/goedit"
)

// AddEditsNotes adds notes to the content to indicate the edits made to the file.
// It marshals the edits into JSON and appends it to the content.
// to support the solution described in https://github.com/xhd2015/xgo/issues/301
func AddEditsNotes(fileEdit *goedit.Edit, file string, origalContent string, content string) (string, error) {
	type Edit struct {
		Start int    `json:"start,omitempty"`
		End   int    `json:"end,omitempty"`
		New   string `json:"new,omitempty"`
	}
	var edits []Edit
	fileEdit.Buffer().RangeEdits(func(start int, end int, new string) {
		edits = append(edits, Edit{Start: start, End: end, New: new})
	})
	editsJSON, err := json.Marshal(edits)
	if err != nil {
		return "", err
	}
	sep := ""
	if !strings.HasSuffix(content, "\n") {
		sep = "\n"
	}
	return content + sep +
		"// __xgo_file: " + strconv.Quote(file) + "\n" +
		"// __xgo_edits: " + string(editsJSON) + "\n" +
		"// __xgo_original: " + strconv.Quote(origalContent), nil
}
