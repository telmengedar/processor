package loop

import "sort"

type heldRow struct {
	candidate Candidate
	form      Form
	size      int
}

type workingMemory struct {
	anchor     Anchor
	considered bool
	rows       map[int64]heldRow
	spent      int
	ceiling    int
}

func newWorkingMemory(ceiling int, anchor Anchor, considered bool, admitted []Candidate, dispositions []Disposition) *workingMemory {
	memory := &workingMemory{anchor: anchor, considered: considered, rows: map[int64]heldRow{}, spent: len(anchor.Substance), ceiling: ceiling}
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
	held, present := m.rows[c.ID]
	if present && held.form == FormSubstance && form == FormUnavailable {
		return
	}
	if present {
		m.spent -= held.size
	}
	m.rows[c.ID] = heldRow{candidate: c, form: form, size: size}
	m.spent += size
}

func (m *workingMemory) charge(bytes int) {
	m.spent += bytes
}

func (m *workingMemory) room(round int) int {
	return max(0, min(round, m.ceiling-m.spent))
}

func (m *workingMemory) roomToReplace(id int64, round int) int {
	return max(0, min(round, m.ceiling-m.spent+m.rows[id].size))
}

func (m *workingMemory) holds(c Candidate) bool {
	held, present := m.rows[c.ID]
	if !present {
		return false
	}
	form, payload := renderedPayload(c)
	_, heldPayload := renderedPayload(held.candidate)
	return held.form == form && payload == heldPayload
}

func (m *workingMemory) render() string {
	rows := make([]Candidate, 0, len(m.rows))
	for _, held := range m.rows {
		rows = append(rows, held.candidate)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })

	return renderBlock(m.anchor, rows, m.considered)
}
