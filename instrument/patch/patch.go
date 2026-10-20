package patch

import (
	"fmt"
	"strings"

	"github.com/xhd2015/xgo/support/fileutil"
	"github.com/xhd2015/xgo/support/strutil"
)

func EditFile(file string, callback func(content string) (string, error)) (err error) {
	bytes, err := fileutil.ReadFile(file)
	if err != nil {
		return err
	}
	content := CleanPatch(string(bytes))
	defer func() {
		if e := recover(); e != nil {
			if pe, ok := e.(error); ok {
				err = pe
			} else {
				err = fmt.Errorf("panic: %v", e)
			}
		}
		if err != nil {
			err = fmt.Errorf("%s: %w", file, err)
		}
	}()
	newContent, err := callback(content)
	if err != nil {
		return err
	}
	if newContent == content {
		return nil
	}
	return fileutil.WriteFile(file, []byte(newContent))
}

type UpdatePosition int

const (
	UpdatePosition_After   UpdatePosition = 0
	UpdatePosition_Before  UpdatePosition = 1
	UpdatePosition_Replace UpdatePosition = 2
)

func UpdateContent(content string, beginMark string, endMark string, seq []string, i int, position UpdatePosition, addContent string) string {
	return updateContent(content, beginMark, endMark, seq, i, position, addContent, "")
}

func updateContent(content string, beginMark string, endMark string, seq []string, i int, position UpdatePosition, patchContent string, separator string) string {
	isReplace := position == UpdatePosition_Replace
	if isReplace {
		replacedContent, ok := tryReplaceWithMark(content, beginMark, endMark, separator, patchContent)
		if ok {
			return replacedContent
		}
	}
	var begin bool
	if position == UpdatePosition_Before || isReplace {
		begin = true
	}
	offset, anchorLen, endOffset := strutil.SequenceOffset(content, seq, i, begin)
	if offset < 0 {
		// logS := content
		// if len(logS) > 1000 {
		// 	logS = logS[:1000] + "..."
		// }
		// fmt.Fprintf(os.Stderr, "DEBUG sequence %d %q missing from\n:%s\n", i, seq, logS)
		qseq := make([]string, len(seq))
		for i, s := range seq {
			qseq[i] = "  " + s
		}
		panic(fmt.Errorf("sequence missing:\n%v", strings.Join(qseq, "\n")))
	}
	// ensure sequence is unique
	anotherOff, _, _ := strutil.SequenceOffset(content[endOffset:], seq, i, false)
	if anotherOff >= 0 {
		panic(fmt.Errorf("sequence duplicate: %v", seq))
	}
	return insertOrReplaceContentNoDuplicate(content, isReplace, anchorLen, beginMark, endMark, offset, patchContent, separator)
}

func insertOrReplaceContentNoDuplicate(content string, replace bool, replaceLen int, beginMark string, endMark string, idx int, insertContent string, separator string) string {
	if insertContent == "" {
		return content
	}
	content, ok := tryReplaceWithMark(content, beginMark, endMark, separator, insertContent)
	if ok {
		return content
	}
	if replace {
		return content[:idx] + beginMark + fmt.Sprintf("/*old:%s*/", content[idx:idx+replaceLen]) + separator + insertContent + separator + endMark + separator + content[idx+replaceLen:]
	}
	closureContent := beginMark + separator + insertContent + separator + endMark + separator
	if strings.Contains(content, closureContent) {
		return content
	}
	return content[:idx] + closureContent + content[idx:]
}

func tryReplaceWithMark(content string, beginMark string, endMark string, separator string, patchContent string) (string, bool) {
	if beginMark == "" || endMark == "" {
		panic(fmt.Errorf("beginMark or endMark is empty"))
	}
	beginIdx := strings.Index(content, beginMark)
	if beginIdx < 0 {
		return content, false
	}
	endIdx := strings.Index(content, endMark)
	if endIdx < 0 {
		return content, false
	}
	oldContent := extractOldContent(content[beginIdx+len(beginMark) : endIdx])
	if oldContent != "" {
		patchContent = oldContent + patchContent
	}
	lastIdx := endIdx + len(endMark)
	return content[:beginIdx] + beginMark + separator + patchContent + separator + endMark + separator + content[lastIdx:], true
}
