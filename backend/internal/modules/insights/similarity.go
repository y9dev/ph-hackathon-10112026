// Package insights provides recurring-problem detection (token Jaccard similarity) and
// role-scoped analytics. It reads requests through its own read-only query store.
package insights

import (
	"sort"
	"strings"
	"time"
	"unicode"
)

// Tunables (D4): chosen on the seed clusters, see similarity_test.go.
const (
	// Threshold is the minimum score for two requests to count as similar.
	Threshold = 0.25
	// CategoryBoost is added to the Jaccard score when both requests share a category.
	CategoryBoost = 0.08
	// StemLen is the prefix length used as a crude Russian stem.
	StemLen = 5
	// MinGroupSize is the smallest suggested group.
	MinGroupSize = 3
)

var stopWords = func() map[string]bool {
	words := `и в во не что он на я с со как а то все она так его но да ты к у же вы за бы по только ее её мне было вот от меня еще ещё нет о из ему
	теперь когда даже ну ли если уже или ни быть был него до вас нибудь опять уж вам ведь там потом себя ничего ей может они тут где есть надо ней
	для мы тебя их чем была сам чтоб без будто чего раз тоже себе под будет ж тогда кто этот того потому этого какой совсем ним здесь этом один почти
	мой тем чтобы нее сейчас были куда зачем всех никогда можно при наконец два об другой хоть после над больше тот через эти нас про всего них какая
	много разве три эту моя впрочем хорошо свою этой перед иногда лучше чуть том нельзя такой им более всегда конечно всю между это также очень нам
	нужно часто каждый просто которые который которая которых которой свой своих наш наши наших весь вся всё весь этих эта также либо пока пусть
	предлагаю предложение проблема проблемы прошу необходимо сделать нужен нужна нужны получается приходится постоянно сейчас сегодня день дня дней
	the and for with from`
	m := map[string]bool{}
	for _, w := range strings.Fields(words) {
		m[w] = true
	}
	return m
}()

// Tokens normalizes text: lowercase, ё→е, split on non-letters/digits, drop stop-words and
// short words (keeping 2-char tokens with digits, e.g. "1с"), stem to a StemLen-rune prefix.
func Tokens(text string) map[string]struct{} {
	text = strings.ReplaceAll(strings.ToLower(text), "ё", "е")
	out := map[string]struct{}{}
	for _, w := range strings.FieldsFunc(text, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		if stopWords[w] {
			continue
		}
		r := []rune(w)
		hasDigit := strings.IndexFunc(w, unicode.IsDigit) >= 0
		if len(r) < 3 && !(hasDigit && len(r) == 2) {
			continue
		}
		if hasDigit && strings.IndexFunc(w, unicode.IsLetter) < 0 {
			continue // pure numbers carry no topic
		}
		if len(r) > StemLen {
			r = r[:StemLen]
		}
		out[string(r)] = struct{}{}
	}
	return out
}

// Jaccard is |A∩B| / |A∪B| (0 for two empty sets).
func Jaccard(a, b map[string]struct{}) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	if len(a) > len(b) {
		a, b = b, a
	}
	inter := 0
	for t := range a {
		if _, ok := b[t]; ok {
			inter++
		}
	}
	union := len(a) + len(b) - inter
	return float64(inter) / float64(union)
}

// Doc is a request as seen by the similarity engine.
type Doc struct {
	ID           int64
	Number       string
	Title        string
	Body         string
	CategoryID   int64
	CategoryName string
	Status       string
	DepartmentID *int64
	AssigneeID   *int64
	ThemeID      *int64
	CreatedAt    time.Time
	tokens       map[string]struct{}
}

func (d *Doc) toks() map[string]struct{} {
	if d.tokens == nil {
		d.tokens = Tokens(d.Title + " " + d.Body)
	}
	return d.tokens
}

// Score is Jaccard over title+body tokens with a same-category boost, capped at 1.
func Score(a, b *Doc) float64 {
	s := Jaccard(a.toks(), b.toks())
	if s > 0 && a.CategoryID == b.CategoryID {
		s += CategoryBoost
	}
	if s > 1 {
		s = 1
	}
	return s
}

func round2(x float64) float64 { return float64(int(x*100+0.5)) / 100 }

// Group is a cluster of similar docs (indexes into the input slice).
type Group struct {
	Members []int
	Score   float64 // mean pairwise score over linked pairs
	Center  int     // member with the highest total similarity
}

// Cluster links docs whose pairwise score >= threshold (union-find) and returns groups of
// at least minSize members, largest first.
func Cluster(docs []Doc, threshold float64, minSize int) []Group {
	n := len(docs)
	parent := make([]int, n)
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(x int) int {
		for parent[x] != x {
			parent[x] = parent[parent[x]]
			x = parent[x]
		}
		return x
	}
	type edge struct {
		i, j int
		s    float64
	}
	var edges []edge
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if s := Score(&docs[i], &docs[j]); s >= threshold {
				edges = append(edges, edge{i, j, s})
				if ri, rj := find(i), find(j); ri != rj {
					parent[ri] = rj
				}
			}
		}
	}
	members := map[int][]int{}
	for i := 0; i < n; i++ {
		r := find(i)
		members[r] = append(members[r], i)
	}
	sum := map[int]float64{}
	cnt := map[int]int{}
	weight := make([]float64, n)
	for _, e := range edges {
		r := find(e.i)
		sum[r] += e.s
		cnt[r]++
		weight[e.i] += e.s
		weight[e.j] += e.s
	}
	var out []Group
	for r, m := range members {
		if len(m) < minSize {
			continue
		}
		center := m[0]
		for _, i := range m {
			if weight[i] > weight[center] {
				center = i
			}
		}
		out = append(out, Group{Members: m, Score: round2(sum[r] / float64(max(cnt[r], 1))), Center: center})
	}
	sort.Slice(out, func(i, j int) bool {
		if len(out[i].Members) != len(out[j].Members) {
			return len(out[i].Members) > len(out[j].Members)
		}
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return docs[out[i].Center].ID < docs[out[j].Center].ID
	})
	return out
}
