package loop

import "sort"

type heldRow struct {
	candidate Candidate
	form      Form
	size      int
}

type partKey struct {
	id      int64
	form    Form
	segment int
}

func keyOf(id int64, form Form, segment int) partKey {
	if form == FormSubstance {
		form = FormContent
	}
	return partKey{id: id, form: form, segment: segment}
}

func (k partKey) before(other partKey) bool {
	if k.id != other.id {
		return k.id < other.id
	}
	if k.form != other.form {
		return k.form < other.form
	}
	return k.segment < other.segment
}

type workingMemory struct {
	anchor     Anchor
	considered bool
	threshold  SubstanceRatio
	rows       map[partKey]heldRow
	spent      int
}

func newWorkingMemory(anchor Anchor, considered bool, threshold SubstanceRatio, admitted []Candidate, dispositions []Disposition) *workingMemory {
	memory := &workingMemory{anchor: anchor, considered: considered, threshold: threshold, rows: map[partKey]heldRow{}, spent: len(anchor.Content)}
	memory.absorb(admitted, dispositions)
	return memory
}

func includedCandidates(candidates []Candidate, dispositions []Disposition) []Candidate {
	included := make([]Candidate, 0, len(candidates))
	for i, d := range dispositions {
		if d.Included {
			included = append(included, candidates[i])
		}
	}
	return included
}

func (m *workingMemory) absorb(admitted []Candidate, dispositions []Disposition) {
	next := 0
	for _, d := range dispositions {
		if !d.Included {
			continue
		}
		if next >= len(admitted) {
			return
		}
		m.hold(admitted[next], d.Form, d.RenderedSize)
		next++
	}
}

func (m *workingMemory) hold(c Candidate, form Form, size int) {
	key := keyOf(c.ID, form, c.Part.Segment)
	held, present := m.rows[key]
	if present && held.form == FormContent && form == FormSubstance {
		return
	}
	if form == FormContent {
		m.evictPartsOf(c.ID)
	}
	if present {
		m.spent -= held.size
	}
	m.rows[key] = heldRow{candidate: c, form: form, size: size}
	m.spent += size
}

func (m *workingMemory) evictPartsOf(id int64) {
	for key, held := range m.rows {
		if key.id == id && (key.form == FormOutline || key.form == FormSegment) {
			m.spent -= held.size
			delete(m.rows, key)
		}
	}
}

func (m *workingMemory) room(round int) int {
	return max(0, min(round, JudgementPromptCeiling-m.spent))
}

func (m *workingMemory) roomToReplace(id int64, round int) int {
	freed := 0
	for key, held := range m.rows {
		if key.id == id {
			freed += held.size
		}
	}
	return max(0, min(round, JudgementPromptCeiling-m.spent+freed))
}

func (m *workingMemory) roomToReplacePart(key partKey, round int) int {
	return max(0, min(round, JudgementPromptCeiling-m.spent+m.rows[key].size))
}

func (m *workingMemory) holds(c Candidate) bool {
	held, present := m.rows[keyOf(c.ID, FormContent, c.Part.Segment)]
	if !present {
		return false
	}
	form, payload := renderedPayload(c, m.threshold)
	_, heldPayload := renderedPayload(held.candidate, m.threshold)
	return held.form == form && payload == heldPayload
}

func (m *workingMemory) render(nudges bool) string {
	keys := make([]partKey, 0, len(m.rows))
	for key := range m.rows {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].before(keys[j]) })

	rows := make([]Candidate, len(keys))
	for i, key := range keys {
		rows[i] = m.rows[key].candidate
	}

	return renderBlock(m.anchor, rows, m.considered, m.threshold, nudges)
}
