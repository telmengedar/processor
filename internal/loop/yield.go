package loop

const barrenRoundsToClose = 2

type rowIdentity struct {
	id   int64
	hash string
}

type yieldAccount struct {
	shown  map[rowIdentity]bool
	barren int
}

func newYieldAccount(block []Disposition) *yieldAccount {
	account := &yieldAccount{shown: make(map[rowIdentity]bool, len(block))}
	account.show(block)
	return account
}

func (a *yieldAccount) show(rows []Disposition) int {
	unseen := 0
	for _, d := range rows {
		if !d.Included {
			continue
		}
		identity := rowIdentity{id: d.ID, hash: d.ContentHash}
		if !a.shown[identity] {
			unseen++
		}
		a.shown[identity] = true
	}
	return unseen
}

func (a *yieldAccount) round(rows []Disposition) int {
	yield := a.show(rows)
	if yield == 0 {
		a.barren++
		return yield
	}
	a.barren = 0
	return yield
}

func (a *yieldAccount) closed() bool {
	return a.barren >= barrenRoundsToClose
}
