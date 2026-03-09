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
| select all | `Ctrl+A` | select the full document |
| clipboard | `Ctrl+C`, `Ctrl+X`, `Ctrl+V` | copy, cut, paste |
| structure | `F2`, `Ctrl+Space`, `Ctrl+[`, `Ctrl+Up/Down` | select block, or apply line/block structural edits |
| indentation | `Tab`, `Shift+Tab`, `Alt+0..4` | insert literal tab or indent/outdent selection, set indent mode |

## Current Control Hub Controls

| Area | Keys | Behavior |
|------|------|----------|
| focus | `Ctrl+G`, `Esc` | enter or leave the Control Hub |
| select all | `Ctrl+A` | select the full one-line command |
| text selection | `Shift+Left/Right`, `Alt+Left/Right`, `Shift+Alt+Left/Right` | expand or shrink one-line selection |
| line selection | `Shift+Home/End`, `Alt+Home/End`, `Shift+Alt+Home/End` | extend selection to input boundaries |
| clipboard | `Ctrl+C`, `Ctrl+X`, `Ctrl+V` | copy, cut, paste, or replace the current selection |
| command flow | `Enter` | run talk/inspect immediately; preview edit and memo commands first, then confirm on second press |

## Selection Model

| Concept | Rule |
|--------|------|
| caret | moves without changing selection when no selection modifier is held |
| text selection | character-range selection between anchor caret and active caret |
| selection modifiers | `Shift`, `Alt`, and `Shift+Alt` are treated the same for selection expansion |
| structural edit projection | indentation and block movement operate on the lines covered by the text selection |
| clipboard behavior | copy/cut use the selected text; paste inserts at the caret or replaces the current selection |
| external clipboard | system clipboard is used when available; gdedit falls back to its internal clipboard otherwise |
| agent boundary | current scope is the collaboration boundary; line-level lock/proposal state is not part of the live model |
| mouse | terminal mouse selection is not part of gdedit's internal edit model |

## Indentation Policy

- Default selection indentation uses `2` spaces.
- `Alt+1` to `Alt+4` change the selection indentation width.
- `Alt+0` switches selection indentation to literal tab mode.
- `Tab` inserts a literal `\t` when there is no active selection.
- Stored literal tab characters render as a visible `»` marker with a distinct style so they can be distinguished from plain text.

## Edit Agent Config

- Global config lives at `~/.config/gdedit/config.json`.
- Project-local memo state belongs under each project's `.gdedit/` directory.
- `gdedit --doctor` now reports the loaded `memoRoot`, edit-agent role, provider, model, and API-key readiness.
- Control Hub confirm now executes the configured edit agent against the current scope and accepts either a scoped replacement or a message-only response.
- Live edit-agent requests now include memo context from the configured system memo root and the current project's `.gdedit/` directory when those files exist.

`config.json` example:

```json
{
  "memoRoot": "~/gdedit/",
  "editAgent": {
    "enabled": true,
    "role": "edit-agent",
    "provider": "openai",
    "model": "gpt-5.4",
    "apiKeyEnv": "OPENAI_API_KEY"
  }
}
```

What each field means:

- `memoRoot`: system/app memo root. `~` is expanded to your home directory and the path is normalized with a trailing `/`.
- `editAgent.enabled`: turns the external edit agent on or off.
- `editAgent.role`: label shown in `--doctor` and UI summaries. The default is `edit-agent`.
- `editAgent.provider`: currently `openai` is supported.
- `editAgent.model`: model name sent to the provider, for example `gpt-5.4`.
- `editAgent.apiKeyEnv`: environment variable name that stores the API key, for example `OPENAI_API_KEY`.
- `editAgent.baseURL`: optional custom OpenAI-compatible endpoint. Leave it out when using the default OpenAI API.

Defaults when the file does not exist:

```json
{
  "memoRoot": "~/gdedit/",
  "editAgent": {
    "enabled": true,
    "role": "edit-agent",
    "provider": "openai",
    "model": "gpt-5.4",
    "apiKeyEnv": "OPENAI_API_KEY"
  }
}
```

Practical setup flow:

1. Create `~/.config/gdedit/config.json`.
2. Set your API key in the shell, for example `OPENAI_API_KEY`.
3. Keep system and app memos under `memoRoot`.
4. Open a project normally; project-local memos are stored under that project's `.gdedit/` directory.
5. Run `gdedit --doctor` to confirm the config path, memo root, provider, model, and API-key readiness.

Example with a custom endpoint:

```json
{
  "memoRoot": "~/gdedit/",
  "editAgent": {
    "enabled": true,
    "role": "edit-agent",
    "provider": "openai",
    "model": "gpt-5.4",
    "apiKeyEnv": "OPENAI_API_KEY",
    "baseURL": "https://api.openai.com/v1"
  }
}
```

Verification:

```bash
gdedit --doctor
```

You should see the loaded config path plus fields like `memo_root`, `edit_agent_provider`, `edit_agent_model`, and `edit_agent_status`.
