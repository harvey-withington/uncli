// What the mock's pages changed, for the walkthrough (decision 0029): a
// page's touched files with hunks have them here, by path.
import type { FileChange } from '../types'

const DIR = 'C:\\Users\\you\\code\\internal\\adapter\\claude\\'

export const MOCK_CHANGES: FileChange[] = [
  {
    path: DIR + 'parser.go',
    hunks: [
      {
        oldStart: 142, newStart: 142, lines: [
          '\tcase "assistant":',
          '\t\tvar m message',
          '\t\t_ = json.Unmarshal(fieldOf(raw, "message"), &m)',
          '-\t\tif p.partial != "" {',
          '-\t\t\tout = append(out, p.flush())',
          '+\t\t// A partial line can arrive before its message: keep it until the',
          '+\t\t// message that ends it, instead of sending it on its own.',
          '+\t\tif p.partial != "" && m.ID == p.partialID {',
          '+\t\t\tout = append(out, p.flush())',
          '+\t\t\tp.partialID = ""',
          '\t\t}',
          '\t\tfor _, b := range blocks(m.Content) {',
        ],
      },
      {
        oldStart: 201, newStart: 204, lines: [
          'func (p *parser) flush() core.Event {',
          '-\ttext := p.partial',
          '+\ttext := strings.TrimRight(p.partial, "\\n")',
          '\tp.partial = ""',
          '\treturn p.ev(core.EvAnswerText, core.AnswerText{Text: text})',
          '}',
        ],
      },
    ],
  },
  {
    path: DIR + 'parser_test.go',
    hunks: [
      {
        oldStart: 0, newStart: 1, lines: [
          '+package claude',
          '+',
          '+import "testing"',
          '+',
          '+// A partial line that arrives before its message stays with it.',
          '+func TestMultiTurnPartial(t *testing.T) {',
          '+\tfor range 50 {',
          '+\t\tevs := parseFixture(t, "multi-turn-partial")',
          '+\t\tif got := answerText(evs); got != "Done." {',
          '+\t\t\tt.Fatalf("answer = %q", got)',
          '+\t\t}',
          '+\t}',
          '+}',
        ],
      },
    ],
  },
]
