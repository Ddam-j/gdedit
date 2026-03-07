# gdedit

AI 에이전트를 포함한 터미널 cowork editor 프로토타입이다. WezTerm 같은 현대적인 터미널을 권장하지만, 현재 구현은 tmux/ssh 환경도 고려한 보수적인 키맵으로 정리되어 있다.

## Current Editor Controls

| Area | Keys | Behavior |
|------|------|----------|
| help / quit | `F1`, `Ctrl+Q` | open help, open quit confirmation |
| focus / tabs | `Ctrl+G`, `Alt+.`, `Alt+,` | focus control hub, next tab, previous tab |
| text selection | `Shift+Arrow`, `Alt+Arrow`, `Shift+Alt+Arrow` | expand or shrink character-range selection |
| line / page selection | `Shift+Home/End`, `Alt+Home/End`, `Shift/Alt+PageUp/PageDown` | extend selection to line edges or by page |
| word movement | `Ctrl+Left/Right`, `Ctrl+Alt+Left/Right` | move caret by word |
| word selection | `Ctrl+Shift+Left/Right`, `Ctrl+Shift+Alt+Left/Right` | extend selection by word |
| clipboard | `Ctrl+C`, `Ctrl+X`, `Ctrl+V` | copy, cut, paste |
| structure | `F2`, `Ctrl+Space`, `Ctrl+[`, `Ctrl+Up/Down` | select block, or apply line/block structural edits |
| indentation | `Tab`, `Shift+Tab`, `Alt+0..4` | insert literal tab or indent/outdent selection, set indent mode |

## Selection Model

| Concept | Rule |
|--------|------|
| caret | moves without changing selection when no selection modifier is held |
| text selection | character-range selection between anchor caret and active caret |
| selection modifiers | `Shift`, `Alt`, and `Shift+Alt` are treated the same for selection expansion |
| structural edit projection | indentation and block movement operate on the lines covered by the text selection |
| clipboard behavior | copy/cut use the selected text; paste inserts at the caret or replaces the current selection |
| mouse | terminal mouse selection is not part of gdedit's internal edit model |

## Indentation Policy

- Default selection indentation uses `2` spaces.
- `Alt+1` to `Alt+4` change the selection indentation width.
- `Alt+0` switches selection indentation to literal tab mode.
- `Tab` inserts a literal `\t` when there is no active selection.
- Stored literal tab characters render as a visible `»` marker with a distinct style so they can be distinguished from plain text.
