# gdedit

AI 에이전트를 포함한 터미널 cowork editor 프로토타입이다. WezTerm 같은 현대적인 터미널을 권장하지만, 현재 구현은 tmux/ssh 환경도 고려한 보수적인 키맵으로 정리되어 있다.

## Current Editor Controls

- `F1`: help dialog
- `Ctrl+C`: quit confirmation dialog
- `Alt+.` / `Alt+,`: next / previous tab
- `F2`: select current code block, then expand to parent block
- `Alt+Up/Down` or `Shift+Up/Down`: expand or shrink line selection
- `Ctrl+Up/Down`: move selected block up or down
- `Tab`: insert a literal `\t` when there is no selection, or indent the active selection
- `Shift+Tab`: outdent the active selection

## Indentation Policy

- Default selection indentation uses `2` spaces.
- `Alt+1` to `Alt+4` change the selection indentation width.
- `Alt+0` switches selection indentation to literal tab mode.
- Stored literal tab characters render as a visible `»` marker with a distinct style so they can be distinguished from plain text.
