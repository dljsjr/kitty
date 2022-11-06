// License: GPLv3 Copyright: 2022, Kovid Goyal, <kovid at kovidgoyal.net>

package readline

import (
	"fmt"
	"kitty/tools/wcswidth"
)

var _ = fmt.Print

func (self *Readline) update_current_screen_size() {
	screen_size, err := self.loop.ScreenSize()
	if err != nil {
		screen_size.WidthCells = 80
		screen_size.HeightCells = 24
	}
	if screen_size.WidthCells < 1 {
		screen_size.WidthCells = 1
	}
	if screen_size.HeightCells < 1 {
		screen_size.HeightCells = 1
	}
	self.screen_width = int(screen_size.WidthCells)
}

type ScreenLine struct {
	ParentLineNumber, OffsetInParentLine         int
	Prompt                                       Prompt
	TextLengthInCells, CursorCell, CursorTextPos int
	Text                                         string
}

func (self *Readline) format_arg_prompt(cna string) string {
	return fmt.Sprintf("(arg: %s) ", self.fmt_ctx.Yellow(cna))
}

func (self *Readline) prompt_for_line_number(i int) Prompt {
	is_line_with_cursor := i == self.cursor.Y
	if is_line_with_cursor && self.keyboard_state.current_numeric_argument != "" {
		return self.make_prompt(self.format_arg_prompt(self.keyboard_state.current_numeric_argument), i > 0)
	}
	if i == 0 {
		if self.history_search != nil {
			return self.make_prompt(self.history_search_prompt(), i > 0)
		}
		return self.prompt
	}
	return self.continuation_prompt
}

func (self *Readline) get_screen_lines() []*ScreenLine {
	if self.screen_width == 0 {
		self.update_current_screen_size()
	}
	ans := make([]*ScreenLine, 0, len(self.lines))
	found_cursor := false
	cursor_at_start_of_next_line := false
	for i, line := range self.lines {
		prompt := self.prompt_for_line_number(i)
		offset := 0
		has_cursor := i == self.cursor.Y
		for is_first := true; is_first || offset < len(line); is_first = false {
			l, width := wcswidth.TruncateToVisualLengthWithWidth(line[offset:], self.screen_width-prompt.Length)
			sl := ScreenLine{
				ParentLineNumber: i, OffsetInParentLine: offset,
				Prompt: prompt, TextLengthInCells: width,
				CursorCell: -1, Text: l, CursorTextPos: -1,
			}
			if cursor_at_start_of_next_line {
				cursor_at_start_of_next_line = false
				sl.CursorCell = prompt.Length
				sl.CursorTextPos = 0
			}
			ans = append(ans, &sl)
			if has_cursor && !found_cursor && offset <= self.cursor.X && self.cursor.X <= offset+len(l) {
				found_cursor = true
				ctpos := self.cursor.X - offset
				ccell := prompt.Length + wcswidth.Stringwidth(l[:ctpos])
				if ccell >= self.screen_width {
					if offset+len(l) < len(line) || i < len(self.lines)-1 {
						cursor_at_start_of_next_line = true
					} else {
						ans = append(ans, &ScreenLine{ParentLineNumber: i, OffsetInParentLine: len(line)})
					}
				} else {
					sl.CursorTextPos = ctpos
					sl.CursorCell = ccell
				}
			}
			prompt = Prompt{}
			offset += len(l)
		}
	}
	return ans
}

func (self *Readline) redraw() {
	if self.screen_width == 0 {
		self.update_current_screen_size()
	}
	if self.screen_width < 4 {
		return
	}
	if self.cursor_y > 0 {
		self.loop.MoveCursorVertically(-self.cursor_y)
	}
	self.loop.QueueWriteString("\r")
	self.loop.ClearToEndOfScreen()
	cursor_x := -1
	cursor_y := 0
	move_cursor_up_by := 0
	self.loop.AllowLineWrapping(false)
	for i, sl := range self.get_screen_lines() {
		self.loop.QueueWriteString("\r")
		if i > 0 {
			self.loop.QueueWriteString("\n")
		}
		if sl.Prompt.Length > 0 {
			self.loop.QueueWriteString(self.prompt_for_line_number(i).Text)
		}
		self.loop.QueueWriteString(sl.Text)
		if sl.CursorCell > -1 {
			cursor_x = sl.CursorCell
		} else if cursor_x > -1 {
			move_cursor_up_by++
		}
		cursor_y++
	}
	self.loop.AllowLineWrapping(true)
	self.loop.MoveCursorVertically(-move_cursor_up_by)
	self.loop.QueueWriteString("\r")
	self.loop.MoveCursorHorizontally(cursor_x)
	self.cursor_y = 0
	if cursor_y > 0 {
		self.cursor_y = cursor_y - 1
	}
}
